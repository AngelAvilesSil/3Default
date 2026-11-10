import type { ProjectBranch } from '../api/projects'
import {
    listProjectBranchHistory,
    RevisionRequestError,
    RevisionResponseError,
    type ProjectRevision,
} from '../api/revisions'
import { buildRevisionGraph, type RevisionGraph, type RevisionGraphCoverage } from './revisionGraph'

// Adapter for the current per-branch history API.
//
// The caller supplies an observed branch snapshot. The provider never
// fetches or silently refreshes branch heads.
//
// A future project-wide history adapter can produce the same RevisionGraph
// without changing the graph renderer or its selection behavior.

type BranchHistoryResult = {
    readonly branchId: string
    readonly revisions: readonly ProjectRevision[] | null
}

function abortError(): Error {
    const error = new Error('Project history loading was cancelled')
    error.name = 'AbortError'
    return error
}

function assertNotAborted(signal?: AbortSignal): void {
    if (signal?.aborted) {
        throw abortError()
    }
}

function isAbortError(error: unknown): boolean {
    return error instanceof Error && error.name === 'AbortError'
}

function isRecoverableHistoryFailure(error: unknown): boolean {
    // A failed branch request can leave the overall graph partially
    // usable. Unexpected programming or graph-integrity failures must
    // propagate instead of being silently represented as missing data.
    return (
        error instanceof RevisionRequestError ||
        error instanceof RevisionResponseError ||
        error instanceof TypeError
    )
}

async function loadBranchHistory(
    projectId: string,
    branchId: string,
    expectedHeadRevisionId: string,
    signal?: AbortSignal,
): Promise<BranchHistoryResult> {
    assertNotAborted(signal)

    try {
        const revisions = await listProjectBranchHistory(projectId, branchId, signal)

        assertNotAborted(signal)

        // The branch list and history requests are independent reads,
        // not an atomic backend snapshot.
        //
        // If the observed head is absent, do not mix this inconsistent
        // history into the displayed graph. Report partial coverage so
        // the UI can offer an explicit refresh.
        if (!revisions.some((revision) => revision.id === expectedHeadRevisionId)) {
            return { branchId, revisions: null }
        }

        return { branchId, revisions }
    } catch (error) {
        // Cancellation must never publish obsolete or partial history.
        if (signal?.aborted || isAbortError(error)) {
            throw abortError()
        }

        // Session expiration must reach the caller's auth handling.
        if (error instanceof RevisionRequestError && error.status === 401) {
            throw error
        }

        // Other request, response, and network failures affect this
        // branch's coverage, not the integrity of successful branches.
        if (isRecoverableHistoryFailure(error)) {
            return { branchId, revisions: null }
        }

        throw error
    }
}

export async function loadProjectHistoryGraph(
    projectId: string,
    observedBranches: readonly ProjectBranch[],
    signal?: AbortSignal,
): Promise<RevisionGraph> {
    assertNotAborted(signal)

    const initialCoverage: RevisionGraphCoverage = {
        source: 'branch-head-reachable',
        status: 'complete',
        failedBranchIds: [],
    }

    // Validate all branch identities before starting requests.
    // This also copies and deterministically orders the observed heads,
    // protecting the load against subsequent caller-side mutation.
    const initialGraph = buildRevisionGraph({
        projectId,
        revisions: [],
        branches: observedBranches,
        coverage: initialCoverage,
    })

    const branchesWithHeads = initialGraph.branches.filter(
        (branch) => branch.headRevisionId !== null,
    )

    if (branchesWithHeads.length === 0) {
        assertNotAborted(signal)
        return initialGraph
    }

    // Each branch starts independently. Promise.all preserves result
    // ordering even if individual requests finish in another order.
    const results = await Promise.all(
        branchesWithHeads.map((branch) => {
            const headRevisionId = branch.headRevisionId

            if (headRevisionId === null) {
                throw new Error('A history request requires a branch head')
            }

            return loadBranchHistory(projectId, branch.id, headRevisionId, signal)
        }),
    )

    // An API or mock may resolve even after cancellation. Never publish
    // a graph from an obsolete request.
    assertNotAborted(signal)

    const revisions: ProjectRevision[] = []
    const failedBranchIds: string[] = []

    for (const result of results) {
        if (result.revisions === null) {
            failedBranchIds.push(result.branchId)
        } else {
            revisions.push(...result.revisions)
        }
    }

    const coverage: RevisionGraphCoverage = {
        source: 'branch-head-reachable',
        status: failedBranchIds.length > 0 ? 'partial' : 'complete',
        failedBranchIds,
    }

    // Canonical revision identity, duplicate validation, ancestry,
    // missing references, branch labels, and cycles are all handled
    // by the pure model rather than duplicated in this adapter.
    return buildRevisionGraph({
        projectId,
        revisions,
        branches: initialGraph.branches,
        coverage,
    })
}
