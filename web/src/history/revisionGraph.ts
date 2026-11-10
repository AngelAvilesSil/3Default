// Pure revision-DAG model. No HTTP, Vue, or endpoint-specific dependencies.
// A history provider supplies records; this model normalizes them for the UI.

export type RevisionGraphRevision = {
    readonly id: string
    readonly projectId: string
    readonly authorUserId: string
    readonly authorDisplayName?: string
    readonly message: string
    readonly createdAt: string
    readonly parentRevisionId: string | null
    readonly mergeParentRevisionId: string | null
}

export type RevisionGraphBranch = {
    readonly id: string
    readonly projectId: string
    readonly name: string
    readonly headRevisionId: string | null
}

export type RevisionGraphCoverage = {
    readonly source: 'branch-head-reachable' | 'project-wide'
    readonly status: 'complete' | 'partial'
    readonly failedBranchIds: readonly string[]
}

export type RevisionGraphEdge = {
    readonly childRevisionId: string
    readonly parentRevisionId: string
    readonly kind: 'parent' | 'merge-parent'
}

export type RevisionGraphNode = {
    readonly revision: RevisionGraphRevision
    readonly headBranchIds: readonly string[]
}

export type RevisionGraphInput = {
    readonly projectId: string
    readonly revisions: readonly RevisionGraphRevision[]
    readonly branches: readonly RevisionGraphBranch[]
    readonly coverage: RevisionGraphCoverage
}

export type RevisionGraph = {
    readonly projectId: string
    readonly nodes: readonly RevisionGraphNode[]
    readonly branches: readonly RevisionGraphBranch[]
    readonly edges: readonly RevisionGraphEdge[]
    readonly unresolvedEdges: readonly RevisionGraphEdge[]
    readonly unresolvedHeadBranchIds: readonly string[]
    readonly coverage: RevisionGraphCoverage
}

function compareText(a: string, b: string): number {
    return a < b ? -1 : a > b ? 1 : 0
}

function compareRevisions(a: RevisionGraphRevision, b: RevisionGraphRevision): number {
    return Date.parse(b.createdAt) - Date.parse(a.createdAt) || compareText(a.id, b.id)
}

function compareBranches(a: RevisionGraphBranch, b: RevisionGraphBranch): number {
    return (
        compareText(a.name.toLowerCase(), b.name.toLowerCase()) ||
        compareText(a.name, b.name) ||
        compareText(a.id, b.id)
    )
}

function copyRevision(revision: RevisionGraphRevision, projectId: string): RevisionGraphRevision {
    if (revision.projectId !== projectId) {
        throw new Error(`Revision belongs to another project: ${revision.id}`)
    }

    if (
        !revision.id.trim() ||
        !revision.authorUserId.trim() ||
        !revision.message.trim() ||
        Number.isNaN(Date.parse(revision.createdAt)) ||
        (revision.parentRevisionId !== null && !revision.parentRevisionId.trim()) ||
        (revision.mergeParentRevisionId !== null && !revision.mergeParentRevisionId.trim())
    ) {
        throw new Error(`Invalid revision graph record: ${revision.id}`)
    }

    return {
        id: revision.id,
        projectId: revision.projectId,
        authorUserId: revision.authorUserId,
        ...(revision.authorDisplayName === undefined
            ? {}
            : { authorDisplayName: revision.authorDisplayName }),
        message: revision.message,
        createdAt: revision.createdAt,
        parentRevisionId: revision.parentRevisionId,
        mergeParentRevisionId: revision.mergeParentRevisionId,
    }
}

function copyBranch(branch: RevisionGraphBranch, projectId: string): RevisionGraphBranch {
    if (branch.projectId !== projectId) {
        throw new Error(`Branch belongs to another project: ${branch.id}`)
    }

    if (
        !branch.id.trim() ||
        !branch.name.trim() ||
        (branch.headRevisionId !== null && !branch.headRevisionId.trim())
    ) {
        throw new Error(`Invalid revision graph branch: ${branch.id}`)
    }

    return {
        id: branch.id,
        projectId: branch.projectId,
        name: branch.name,
        headRevisionId: branch.headRevisionId,
    }
}

// Check the resolved DAG iteratively without recursion.
// Unavailable references remain explicit rather than becoming invented nodes.
function assertAcyclic(revisionIds: readonly string[], edges: readonly RevisionGraphEdge[]): void {
    const incoming = new Map(revisionIds.map((id) => [id, 0]))
    const parentsByChild = new Map<string, string[]>()

    for (const edge of edges) {
        const count = incoming.get(edge.parentRevisionId)

        if (count === undefined) {
            throw new Error('Revision graph contains an invalid resolved edge')
        }

        incoming.set(edge.parentRevisionId, count + 1)

        const parents = parentsByChild.get(edge.childRevisionId) ?? []
        parents.push(edge.parentRevisionId)
        parentsByChild.set(edge.childRevisionId, parents)
    }

    const queue = revisionIds.filter((id) => incoming.get(id) === 0)
    let processed = 0

    for (let index = 0; index < queue.length; index += 1) {
        const child = queue[index]

        if (child === undefined) {
            continue
        }

        processed += 1

        for (const parentId of parentsByChild.get(child) ?? []) {
            const remaining = (incoming.get(parentId) ?? 0) - 1
            incoming.set(parentId, remaining)

            if (remaining === 0) {
                queue.push(parentId)
            }
        }
    }

    if (processed !== revisionIds.length) {
        throw new Error('Revision graph contains a cycle or self-ancestry')
    }
}

export function buildRevisionGraph(input: RevisionGraphInput): RevisionGraph {
    if (!input.projectId.trim()) {
        throw new Error('Revision graph requires a project ID')
    }

    const revisionsById = new Map<string, RevisionGraphRevision>()

    for (const rawRevision of input.revisions) {
        const revision = copyRevision(rawRevision, input.projectId)
        const existing = revisionsById.get(revision.id)

        if (existing) {
            if (JSON.stringify(existing) !== JSON.stringify(revision)) {
                throw new Error(`Conflicting revision identity: ${revision.id}`)
            }
            continue
        }

        revisionsById.set(revision.id, revision)
    }

    const branchesById = new Map<string, RevisionGraphBranch>()

    for (const rawBranch of input.branches) {
        const branch = copyBranch(rawBranch, input.projectId)
        const existing = branchesById.get(branch.id)

        if (existing) {
            if (JSON.stringify(existing) !== JSON.stringify(branch)) {
                throw new Error(`Conflicting branch identity: ${branch.id}`)
            }
            continue
        }

        branchesById.set(branch.id, branch)
    }

    const revisions = [...revisionsById.values()].sort(compareRevisions)
    const branches = [...branchesById.values()].sort(compareBranches)

    const edges: RevisionGraphEdge[] = []
    const unresolvedEdges: RevisionGraphEdge[] = []

    for (const revision of revisions) {
        const references: Array<{
            kind: RevisionGraphEdge['kind']
            parentRevisionId: string | null
        }> = [
            { kind: 'parent', parentRevisionId: revision.parentRevisionId },
            { kind: 'merge-parent', parentRevisionId: revision.mergeParentRevisionId },
        ]

        for (const reference of references) {
            if (reference.parentRevisionId === null) {
                continue
            }

            const edge: RevisionGraphEdge = {
                childRevisionId: revision.id,
                parentRevisionId: reference.parentRevisionId,
                kind: reference.kind,
            }

            if (revisionsById.has(reference.parentRevisionId)) {
                edges.push(edge)
            } else {
                unresolvedEdges.push(edge)
            }
        }
    }

    assertAcyclic(
        revisions.map((revision) => revision.id),
        edges,
    )

    const headsByRevision = new Map<string, string[]>()
    const unresolvedHeadBranchIds: string[] = []

    for (const branch of branches) {
        if (branch.headRevisionId === null) {
            continue
        }

        if (!revisionsById.has(branch.headRevisionId)) {
            unresolvedHeadBranchIds.push(branch.id)
            continue
        }

        const heads = headsByRevision.get(branch.headRevisionId) ?? []
        heads.push(branch.id)
        headsByRevision.set(branch.headRevisionId, heads)
    }

    const nodes: RevisionGraphNode[] = revisions.map((revision) => ({
        revision,
        headBranchIds: headsByRevision.get(revision.id) ?? [],
    }))

    const failedBranchIds = [...new Set(input.coverage.failedBranchIds)].sort(compareText)

    const incomplete =
        input.coverage.status === 'partial' ||
        failedBranchIds.length > 0 ||
        unresolvedEdges.length > 0 ||
        unresolvedHeadBranchIds.length > 0

    return {
        projectId: input.projectId,
        nodes,
        branches,
        edges,
        unresolvedEdges,
        unresolvedHeadBranchIds,
        coverage: {
            source: input.coverage.source,
            status: incomplete ? 'partial' : 'complete',
            failedBranchIds,
        },
    }
}

// Return the selected revision and all loaded primary/merge ancestors.
// Output follows deterministic node order, not traversal or API response order.
export function getRevisionAncestry(graph: RevisionGraph, revisionId: string | null): string[] {
    if (revisionId === null || !graph.nodes.some((node) => node.revision.id === revisionId)) {
        return []
    }

    const parentsByChild = new Map<string, string[]>()

    for (const edge of graph.edges) {
        const parents = parentsByChild.get(edge.childRevisionId) ?? []
        parents.push(edge.parentRevisionId)
        parentsByChild.set(edge.childRevisionId, parents)
    }

    const visited = new Set<string>()
    const pending = [revisionId]

    while (pending.length > 0) {
        const current = pending.pop()

        if (current === undefined || visited.has(current)) {
            continue
        }

        visited.add(current)

        for (const parentId of parentsByChild.get(current) ?? []) {
            if (!visited.has(parentId)) {
                pending.push(parentId)
            }
        }
    }

    return graph.nodes
        .filter((node) => visited.has(node.revision.id))
        .map((node) => node.revision.id)
}
