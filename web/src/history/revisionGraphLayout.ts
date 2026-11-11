import type { RevisionGraph, RevisionGraphEdge, RevisionGraphRevision } from './revisionGraph'

// Pure graph-to-layout adapter. Positions are integer rows and lanes;
// Vue and CSS decide spacing, dimensions, colors, and rendering.
//
// A child always appears above every loaded parent, even when timestamps
// are inconsistent. Primary ancestry stays in the same lane where
// possible; merge-parent ancestry can introduce another lane.

export type RevisionGraphLayoutNode = {
    readonly revisionId: string
    readonly row: number
    readonly lane: number
}

export type RevisionGraphLayoutEdge = RevisionGraphEdge & {
    readonly childRow: number
    readonly childLane: number
    readonly parentRow: number
    readonly parentLane: number
}

export type RevisionGraphLayoutUnresolvedEdge = RevisionGraphEdge & {
    readonly childRow: number
    readonly childLane: number
}

export type RevisionGraphLayout = {
    readonly nodes: readonly RevisionGraphLayoutNode[]
    readonly edges: readonly RevisionGraphLayoutEdge[]
    readonly unresolvedEdges: readonly RevisionGraphLayoutUnresolvedEdge[]
    readonly laneCount: number
}

function assignLanes(graph: RevisionGraph): Map<string, number> {
    const revisionsById = new Map<string, RevisionGraphRevision>(
        graph.nodes.map((node) => [node.revision.id, node.revision]),
    )

    const laneById = new Map<string, number>()
    const pendingMergeParents: string[] = []
    let laneCount = 0

    // Follow an entire primary-parent line until it reaches a revision
    // already assigned to another line, an unknown ancestor, or a root.
    // The iterative walk also works for long revision histories.
    function claimPrimaryLineage(startId: string): void {
        if (!revisionsById.has(startId) || laneById.has(startId)) {
            return
        }

        const lane = laneCount
        laneCount += 1

        let currentId: string | null = startId

        while (currentId !== null && !laneById.has(currentId)) {
            const revision = revisionsById.get(currentId)

            if (revision === undefined) {
                break
            }

            laneById.set(currentId, lane)

            const mergeParentId = revision.mergeParentRevisionId

            if (
                mergeParentId !== null &&
                revisionsById.has(mergeParentId) &&
                !laneById.has(mergeParentId)
            ) {
                pendingMergeParents.push(mergeParentId)
            }

            currentId = revision.parentRevisionId
        }
    }

    // Prefer main as lane zero when a loaded main head exists.
    // The normalized graph already has a stable branch ordering.
    const main = graph.branches.find((branch) => branch.name === 'main')
    const orderedBranches = main
        ? [main, ...graph.branches.filter((branch) => branch.id !== main.id)]
        : graph.branches

    for (const branch of orderedBranches) {
        if (branch.headRevisionId !== null) {
            claimPrimaryLineage(branch.headRevisionId)
        }
    }

    // Merge-parent lineages may not currently have branch heads.
    // Assign them before unrelated project-wide revisions.
    let mergeIndex = 0

    function claimPendingMergeParents(): void {
        while (mergeIndex < pendingMergeParents.length) {
            const revisionId = pendingMergeParents[mergeIndex]
            mergeIndex += 1

            if (revisionId !== undefined) {
                claimPrimaryLineage(revisionId)
            }
        }
    }

    claimPendingMergeParents()

    // Future project-wide APIs may include revisions unreachable from
    // today's branch heads. Keep each disconnected primary lineage.
    for (const node of graph.nodes) {
        claimPrimaryLineage(node.revision.id)
        claimPendingMergeParents()
    }

    return laneById
}

function orderNodesByAncestry(graph: RevisionGraph): string[] {
    // The graph model provides deterministic node order, which acts as
    // a priority tie-breaker between otherwise independent revisions.
    const rank = new Map(graph.nodes.map((node, index) => [node.revision.id, index]))

    // A parent becomes eligible only after every loaded child edge has
    // been processed. Both primary and merge-parent edges count.
    const pendingChildren = new Map(graph.nodes.map((node) => [node.revision.id, 0]))
    const parentsByChild = new Map<string, string[]>()

    for (const edge of graph.edges) {
        const count = pendingChildren.get(edge.parentRevisionId)

        if (count === undefined || !rank.has(edge.childRevisionId)) {
            throw new Error('Revision layout contains an invalid edge')
        }

        pendingChildren.set(edge.parentRevisionId, count + 1)

        const parents = parentsByChild.get(edge.childRevisionId) ?? []
        parents.push(edge.parentRevisionId)
        parentsByChild.set(edge.childRevisionId, parents)
    }

    // Highest-priority ready node is kept at the end for efficient pop.
    const ready = graph.nodes
        .filter((node) => pendingChildren.get(node.revision.id) === 0)
        .map((node) => node.revision.id)
        .reverse()

    function enqueueReady(revisionId: string): void {
        const priority = rank.get(revisionId)

        if (priority === undefined) {
            throw new Error('Revision layout contains an unknown revision')
        }

        // Binary search keeps ready nodes ordered without repeatedly
        // sorting every revision after a parent becomes eligible.
        let low = 0
        let high = ready.length

        while (low < high) {
            const mid = Math.floor((low + high) / 2)
            const candidate = ready[mid]
            const candidateRank = candidate === undefined ? undefined : rank.get(candidate)

            if (candidateRank === undefined) {
                throw new Error('Revision layout ready queue is inconsistent')
            }

            if (candidateRank > priority) {
                low = mid + 1
            } else {
                high = mid
            }
        }

        ready.splice(low, 0, revisionId)
    }

    const orderedIds: string[] = []

    while (ready.length > 0) {
        const currentId = ready.pop()

        if (currentId === undefined) {
            break
        }

        orderedIds.push(currentId)

        for (const parentId of parentsByChild.get(currentId) ?? []) {
            const remaining = pendingChildren.get(parentId)

            if (remaining === undefined || remaining < 1) {
                throw new Error('Revision layout ancestry is inconsistent')
            }

            pendingChildren.set(parentId, remaining - 1)

            if (remaining === 1) {
                enqueueReady(parentId)
            }
        }
    }

    if (orderedIds.length !== graph.nodes.length) {
        throw new Error('Revision layout could not order the entire graph')
    }

    return orderedIds
}

export function layoutRevisionGraph(graph: RevisionGraph): RevisionGraphLayout {
    const laneById = assignLanes(graph)
    const orderedIds = orderNodesByAncestry(graph)

    const nodes: RevisionGraphLayoutNode[] = orderedIds.map((revisionId, row) => {
        const lane = laneById.get(revisionId)

        if (lane === undefined) {
            throw new Error('Revision layout is missing a node lane')
        }

        return { revisionId, row, lane }
    })

    const positions = new Map(nodes.map((node) => [node.revisionId, node]))

    const edges: RevisionGraphLayoutEdge[] = graph.edges.map((edge) => {
        const child = positions.get(edge.childRevisionId)
        const parent = positions.get(edge.parentRevisionId)

        if (child === undefined || parent === undefined) {
            throw new Error('Revision layout is missing an edge endpoint')
        }

        return {
            ...edge,
            childRow: child.row,
            childLane: child.lane,
            parentRow: parent.row,
            parentLane: parent.lane,
        }
    })

    const unresolvedEdges: RevisionGraphLayoutUnresolvedEdge[] = graph.unresolvedEdges.map(
        (edge) => {
            const child = positions.get(edge.childRevisionId)

            if (child === undefined) {
                throw new Error('Revision layout is missing an unresolved edge source')
            }

            return {
                ...edge,
                childRow: child.row,
                childLane: child.lane,
            }
        },
    )

    return {
        nodes,
        edges,
        unresolvedEdges,
        laneCount: laneById.size === 0 ? 0 : Math.max(...laneById.values()) + 1,
    }
}
