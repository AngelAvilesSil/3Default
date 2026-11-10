import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { ProjectBranch } from '../api/projects'
import {
    listProjectBranchHistory,
    RevisionRequestError,
    type ProjectRevision,
} from '../api/revisions'
import { loadProjectHistoryGraph } from '../history/projectHistoryProvider'

vi.mock('../api/revisions', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/revisions')>()

    return {
        ...original,
        listProjectBranchHistory: vi.fn<typeof original.listProjectBranchHistory>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const authorUserId = '22222222-2222-4222-8222-222222222222'
const timestamp = '2026-11-09T12:00:00Z'

function branch(id: string, headRevisionId: string | null, name = id): ProjectBranch {
    return {
        id,
        projectId,
        name,
        headRevisionId,
        createdAt: timestamp,
        updatedAt: timestamp,
    }
}

function revision(
    id: string,
    parentRevisionId: string | null = null,
    mergeParentRevisionId: string | null = null,
): ProjectRevision {
    return {
        id,
        projectId,
        authorUserId,
        message: `Revision ${id}`,
        parentRevisionId,
        mergeParentRevisionId,
        createdAt: timestamp,
    }
}

const root = revision('root')
const mainHead = revision('main-head', 'root')
const alternativeHead = revision('alternative-head', 'root')
const mergedHead = revision('merged-head', 'main-head', 'alternative-head')

beforeEach(() => {
    vi.resetAllMocks()
})

describe('project history provider', () => {
    it('represents a project without branches without requesting history', async () => {
        const result = await loadProjectHistoryGraph(projectId, [])

        expect(result.projectId).toBe(projectId)
        expect(result.branches).toEqual([])
        expect(result.nodes).toEqual([])
        expect(result.edges).toEqual([])
        expect(result.coverage).toEqual({
            source: 'branch-head-reachable',
            status: 'complete',
            failedBranchIds: [],
        })
        expect(listProjectBranchHistory).not.toHaveBeenCalled()
    })

    it('skips headless branches and preserves their graph labels', async () => {
        const main = branch('main', 'root')
        const empty = branch('empty', null)

        vi.mocked(listProjectBranchHistory).mockResolvedValue([root])

        const result = await loadProjectHistoryGraph(projectId, [main, empty])

        expect(listProjectBranchHistory).toHaveBeenCalledExactlyOnceWith(
            projectId,
            'main',
            undefined,
        )
        expect(result.branches.map((item) => item.id)).toEqual(['empty', 'main'])
        expect(result.nodes.map((node) => node.revision.id)).toEqual(['root'])
        expect(result.nodes[0]?.headBranchIds).toEqual(['main'])
        expect(result.coverage.status).toBe('complete')
    })

    it('unifies branch histories with shared ancestors and merge parents', async () => {
        const branches = [branch('main', 'merged-head'), branch('alternative', 'alternative-head')]

        vi.mocked(listProjectBranchHistory).mockImplementation(async (_, branchId) => {
            return branchId === 'main'
                ? [mergedHead, mainHead, alternativeHead, root]
                : [alternativeHead, root]
        })

        const result = await loadProjectHistoryGraph(projectId, branches)

        expect(listProjectBranchHistory).toHaveBeenCalledTimes(2)
        expect(result.nodes).toHaveLength(4)
        expect(result.nodes.filter((node) => node.revision.id === 'root')).toHaveLength(1)
        expect(result.edges).toContainEqual({
            childRevisionId: 'merged-head',
            parentRevisionId: 'main-head',
            kind: 'parent',
        })
        expect(result.edges).toContainEqual({
            childRevisionId: 'merged-head',
            parentRevisionId: 'alternative-head',
            kind: 'merge-parent',
        })
        expect(
            result.nodes.find((node) => node.revision.id === 'merged-head')?.headBranchIds,
        ).toEqual(['main'])
        expect(result.coverage.status).toBe('complete')
        expect(result.coverage.source).toBe('branch-head-reachable')
    })

    it('starts independent branch requests concurrently', async () => {
        let resolveMain!: (value: ProjectRevision[]) => void
        let resolveAlternative!: (value: ProjectRevision[]) => void

        vi.mocked(listProjectBranchHistory).mockImplementation((_, branchId) => {
            return new Promise<ProjectRevision[]>((resolve) => {
                if (branchId === 'main') {
                    resolveMain = resolve
                } else {
                    resolveAlternative = resolve
                }
            })
        })

        const pending = loadProjectHistoryGraph(projectId, [
            branch('main', 'main-head'),
            branch('alternative', 'alternative-head'),
        ])

        await vi.waitFor(() => {
            expect(listProjectBranchHistory).toHaveBeenCalledTimes(2)
        })

        resolveMain([mainHead, root])
        resolveAlternative([alternativeHead, root])

        const result = await pending

        expect(result.nodes).toHaveLength(3)
        expect(result.coverage.status).toBe('complete')
    })

    it('preserves successful branches when another history request fails', async () => {
        vi.mocked(listProjectBranchHistory).mockImplementation(async (_, branchId) => {
            if (branchId === 'alternative') {
                throw new RevisionRequestError(503, 'Unavailable')
            }

            return [mainHead, root]
        })

        const result = await loadProjectHistoryGraph(projectId, [
            branch('main', 'main-head'),
            branch('alternative', 'alternative-head'),
        ])

        expect(result.nodes.map((node) => node.revision.id)).toEqual(['main-head', 'root'])
        expect(result.coverage).toEqual({
            source: 'branch-head-reachable',
            status: 'partial',
            failedBranchIds: ['alternative'],
        })
        expect(result.unresolvedHeadBranchIds).toEqual(['alternative'])
    })

    it('excludes a history response that omits its observed branch head', async () => {
        vi.mocked(listProjectBranchHistory).mockImplementation(async (_, branchId) => {
            return branchId === 'main' ? [mainHead, root] : [root]
        })

        const result = await loadProjectHistoryGraph(projectId, [
            branch('main', 'main-head'),
            branch('alternative', 'alternative-head'),
        ])

        expect(result.nodes.map((node) => node.revision.id)).toEqual(['main-head', 'root'])
        expect(result.coverage.status).toBe('partial')
        expect(result.coverage.failedBranchIds).toEqual(['alternative'])
        expect(result.unresolvedHeadBranchIds).toEqual(['alternative'])
    })

    it('reports partial coverage when every branch request fails', async () => {
        vi.mocked(listProjectBranchHistory).mockRejectedValue(
            new RevisionRequestError(503, 'Unavailable'),
        )

        const result = await loadProjectHistoryGraph(projectId, [
            branch('main', 'main-head'),
            branch('alternative', 'alternative-head'),
        ])

        expect(result.nodes).toEqual([])
        expect(result.coverage.status).toBe('partial')
        expect(result.coverage.failedBranchIds).toEqual(['alternative', 'main'])
    })

    it('propagates authentication failures rather than returning partial history', async () => {
        vi.mocked(listProjectBranchHistory).mockImplementation(async (_, branchId) => {
            if (branchId === 'alternative') {
                throw new RevisionRequestError(401, 'Unauthorized')
            }

            return [mainHead, root]
        })

        await expect(
            loadProjectHistoryGraph(projectId, [
                branch('main', 'main-head'),
                branch('alternative', 'alternative-head'),
            ]),
        ).rejects.toMatchObject({
            name: 'RevisionRequestError',
            status: 401,
        })
    })

    it('rejects an already-aborted load before starting history requests', async () => {
        const controller = new AbortController()
        controller.abort()

        await expect(
            loadProjectHistoryGraph(projectId, [branch('main', 'main-head')], controller.signal),
        ).rejects.toMatchObject({ name: 'AbortError' })

        expect(listProjectBranchHistory).not.toHaveBeenCalled()
    })

    it('does not publish a completed graph after cancellation', async () => {
        const controller = new AbortController()
        let resolveRequest!: (value: ProjectRevision[]) => void

        vi.mocked(listProjectBranchHistory).mockImplementation(
            () =>
                new Promise<ProjectRevision[]>((resolve) => {
                    resolveRequest = resolve
                }),
        )

        const pending = loadProjectHistoryGraph(
            projectId,
            [branch('main', 'main-head')],
            controller.signal,
        )

        await vi.waitFor(() => {
            expect(listProjectBranchHistory).toHaveBeenCalledWith(
                projectId,
                'main',
                controller.signal,
            )
        })

        controller.abort()
        resolveRequest([mainHead, root])

        await expect(pending).rejects.toMatchObject({ name: 'AbortError' })
    })

    it('rejects foreign-project branches before making any request', async () => {
        await expect(
            loadProjectHistoryGraph(projectId, [
                { ...branch('foreign', 'main-head'), projectId: 'another-project' },
            ]),
        ).rejects.toThrow(/project/i)

        expect(listProjectBranchHistory).not.toHaveBeenCalled()
    })

    it('rejects conflicting revision identities from different branches', async () => {
        vi.mocked(listProjectBranchHistory).mockImplementation(async (_, branchId) => {
            return branchId === 'main'
                ? [mainHead, root]
                : [{ ...mainHead, message: 'Conflicting content' }, root]
        })

        await expect(
            loadProjectHistoryGraph(projectId, [
                branch('main', 'main-head'),
                branch('alternative', 'main-head'),
            ]),
        ).rejects.toThrow(/conflicting revision/i)
    })

    it('reports unresolved ancestors without manufacturing missing revisions', async () => {
        vi.mocked(listProjectBranchHistory).mockResolvedValue([
            revision('main-head', 'missing-ancestor'),
        ])

        const result = await loadProjectHistoryGraph(projectId, [branch('main', 'main-head')])

        expect(result.nodes).toHaveLength(1)
        expect(result.edges).toEqual([])
        expect(result.unresolvedEdges).toEqual([
            {
                childRevisionId: 'main-head',
                parentRevisionId: 'missing-ancestor',
                kind: 'parent',
            },
        ])
        expect(result.coverage.status).toBe('partial')
        expect(result.coverage.failedBranchIds).toEqual([])
    })
})
