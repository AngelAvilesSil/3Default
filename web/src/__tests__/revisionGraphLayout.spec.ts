import { describe, expect, it } from 'vitest'

import {
    buildRevisionGraph,
    type RevisionGraphBranch,
    type RevisionGraphCoverage,
    type RevisionGraphRevision,
} from '../history/revisionGraph'
import { layoutRevisionGraph } from '../history/revisionGraphLayout'

const projectId = '11111111-1111-4111-8111-111111111111'
const authorUserId = '22222222-2222-4222-8222-222222222222'

const completeCoverage: RevisionGraphCoverage = {
    source: 'branch-head-reachable',
    status: 'complete',
    failedBranchIds: [],
}

function revision(
    id: string,
    day: number,
    parentRevisionId: string | null = null,
    mergeParentRevisionId: string | null = null,
): RevisionGraphRevision {
    return {
        id,
        projectId,
        authorUserId,
        message: `Revision ${id}`,
        createdAt: new Date(Date.UTC(2026, 9, day, 12)).toISOString(),
        parentRevisionId,
        mergeParentRevisionId,
    }
}

function branch(id: string, name: string, headRevisionId: string | null): RevisionGraphBranch {
    return {
        id,
        projectId,
        name,
        headRevisionId,
    }
}

function graph(
    revisions: readonly RevisionGraphRevision[],
    branches: readonly RevisionGraphBranch[],
    coverage: RevisionGraphCoverage = completeCoverage,
) {
    return buildRevisionGraph({
        projectId,
        revisions,
        branches,
        coverage,
    })
}

describe('deterministic revision graph layout', () => {
    it('returns an empty layout when the project has no revisions', () => {
        const result = layoutRevisionGraph(graph([], []))

        expect(result).toEqual({
            nodes: [],
            edges: [],
            unresolvedEdges: [],
            laneCount: 0,
        })
    })

    it('does not create lanes or revisions for headless branches', () => {
        const result = layoutRevisionGraph(
            graph([], [branch('main-id', 'main', null), branch('feature-id', 'feature', null)]),
        )

        expect(result.nodes).toEqual([])
        expect(result.edges).toEqual([])
        expect(result.unresolvedEdges).toEqual([])
        expect(result.laneCount).toBe(0)
    })

    it('places a linear main ancestry in a single vertical lane', () => {
        const result = layoutRevisionGraph(
            graph(
                [
                    revision('root', 1),
                    revision('second', 2, 'root'),
                    revision('third', 3, 'second'),
                ],
                [branch('main-id', 'main', 'third')],
            ),
        )

        expect(result.nodes).toEqual([
            { revisionId: 'third', row: 0, lane: 0 },
            { revisionId: 'second', row: 1, lane: 0 },
            { revisionId: 'root', row: 2, lane: 0 },
        ])

        expect(result.edges).toEqual([
            {
                childRevisionId: 'third',
                parentRevisionId: 'second',
                kind: 'parent',
                childRow: 0,
                childLane: 0,
                parentRow: 1,
                parentLane: 0,
            },
            {
                childRevisionId: 'second',
                parentRevisionId: 'root',
                kind: 'parent',
                childRow: 1,
                childLane: 0,
                parentRow: 2,
                parentLane: 0,
            },
        ])

        expect(result.laneCount).toBe(1)
    })

    it('places diverging branches in separate lanes with one shared ancestor', () => {
        const result = layoutRevisionGraph(
            graph(
                [
                    revision('root', 1),
                    revision('feature-head', 3, 'root'),
                    revision('main-head', 4, 'root'),
                ],
                [
                    branch('feature-id', 'feature', 'feature-head'),
                    branch('main-id', 'main', 'main-head'),
                ],
            ),
        )

        expect(result.nodes.find((node) => node.revisionId === 'main-head')?.lane).toBe(0)
        expect(result.nodes.find((node) => node.revisionId === 'feature-head')?.lane).toBe(1)
        expect(result.nodes.find((node) => node.revisionId === 'root')?.lane).toBe(0)
        expect(result.nodes.filter((node) => node.revisionId === 'root')).toHaveLength(1)
        expect(result.laneCount).toBe(2)
    })

    it('keeps primary and merge-parent ancestry as distinct edges', () => {
        const result = layoutRevisionGraph(
            graph(
                [
                    revision('root', 1),
                    revision('main-parent', 3, 'root'),
                    revision('feature-parent', 4, 'root'),
                    revision('merge-head', 5, 'main-parent', 'feature-parent'),
                ],
                [
                    branch('main-id', 'main', 'merge-head'),
                    branch('feature-id', 'feature', 'feature-parent'),
                ],
            ),
        )

        const merge = result.nodes.find((node) => node.revisionId === 'merge-head')
        const primary = result.nodes.find((node) => node.revisionId === 'main-parent')
        const feature = result.nodes.find((node) => node.revisionId === 'feature-parent')

        expect(merge?.lane).toBe(0)
        expect(primary?.lane).toBe(0)
        expect(feature?.lane).toBe(1)

        expect(result.edges).toContainEqual({
            childRevisionId: 'merge-head',
            parentRevisionId: 'main-parent',
            kind: 'parent',
            childRow: merge?.row,
            childLane: 0,
            parentRow: primary?.row,
            parentLane: 0,
        })

        expect(result.edges).toContainEqual({
            childRevisionId: 'merge-head',
            parentRevisionId: 'feature-parent',
            kind: 'merge-parent',
            childRow: merge?.row,
            childLane: 0,
            parentRow: feature?.row,
            parentLane: 1,
        })

        expect(result.edges).toHaveLength(4)
        expect(result.laneCount).toBe(2)
    })

    it('allocates a lane for a merge lineage without its own branch head', () => {
        const result = layoutRevisionGraph(
            graph(
                [
                    revision('root', 1),
                    revision('main-parent', 2, 'root'),
                    revision('other-parent', 3, 'root'),
                    revision('merge-head', 4, 'main-parent', 'other-parent'),
                ],
                [branch('main-id', 'main', 'merge-head')],
            ),
        )

        expect(result.nodes.find((node) => node.revisionId === 'merge-head')?.lane).toBe(0)
        expect(result.nodes.find((node) => node.revisionId === 'main-parent')?.lane).toBe(0)
        expect(result.nodes.find((node) => node.revisionId === 'other-parent')?.lane).toBe(1)
        expect(result.nodes.find((node) => node.revisionId === 'root')?.lane).toBe(0)
        expect(result.laneCount).toBe(2)
    })

    it('does not allocate multiple lanes for branches sharing one head', () => {
        const result = layoutRevisionGraph(
            graph(
                [revision('root', 1), revision('shared-head', 2, 'root')],
                [
                    branch('main-id', 'main', 'shared-head'),
                    branch('feature-id', 'feature', 'shared-head'),
                ],
            ),
        )

        expect(result.nodes).toEqual([
            { revisionId: 'shared-head', row: 0, lane: 0 },
            { revisionId: 'root', row: 1, lane: 0 },
        ])
        expect(result.laneCount).toBe(1)
    })

    it('orders children above both parents despite inconsistent timestamps', () => {
        const result = layoutRevisionGraph(
            graph(
                [
                    revision('root', 9),
                    revision('main-parent', 2, 'root'),
                    revision('feature-parent', 8, 'root'),
                    revision('merge-head', 1, 'main-parent', 'feature-parent'),
                ],
                [branch('main-id', 'main', 'merge-head')],
            ),
        )

        const byId = new Map(result.nodes.map((node) => [node.revisionId, node]))

        for (const edge of result.edges) {
            expect(byId.get(edge.childRevisionId)?.row).toBeLessThan(
                byId.get(edge.parentRevisionId)!.row,
            )
            expect(edge.childRow).toBe(byId.get(edge.childRevisionId)?.row)
            expect(edge.parentRow).toBe(byId.get(edge.parentRevisionId)?.row)
        }

        expect(result.nodes[0]?.revisionId).toBe('merge-head')
    })

    it('retains disconnected project-wide revisions in distinct lanes', () => {
        const result = layoutRevisionGraph(
            graph(
                [revision('main-root', 1), revision('unreferenced', 4)],
                [branch('main-id', 'main', 'main-root')],
                {
                    source: 'project-wide',
                    status: 'complete',
                    failedBranchIds: [],
                },
            ),
        )

        expect(result.nodes).toHaveLength(2)
        expect(result.nodes.find((node) => node.revisionId === 'main-root')?.lane).toBe(0)
        expect(result.nodes.find((node) => node.revisionId === 'unreferenced')?.lane).toBe(1)
        expect(result.laneCount).toBe(2)
    })

    it('locates unresolved ancestor references without creating phantom nodes', () => {
        const source = graph(
            [revision('child', 4, 'missing-parent', 'missing-merge')],
            [branch('main-id', 'main', 'child'), branch('other-id', 'other', 'missing-head')],
            {
                source: 'branch-head-reachable',
                status: 'partial',
                failedBranchIds: ['other-id'],
            },
        )

        const result = layoutRevisionGraph(source)

        expect(result.nodes).toEqual([{ revisionId: 'child', row: 0, lane: 0 }])
        expect(result.edges).toEqual([])
        expect(result.unresolvedEdges).toEqual([
            {
                childRevisionId: 'child',
                parentRevisionId: 'missing-parent',
                kind: 'parent',
                childRow: 0,
                childLane: 0,
            },
            {
                childRevisionId: 'child',
                parentRevisionId: 'missing-merge',
                kind: 'merge-parent',
                childRow: 0,
                childLane: 0,
            },
        ])
        expect(result.nodes.some((node) => node.revisionId === 'missing-head')).toBe(false)
        expect(result.laneCount).toBe(1)
        expect(source.unresolvedHeadBranchIds).toEqual(['other-id'])
    })

    it('is deterministic across input permutations and does not mutate the graph', () => {
        const revisions = [
            revision('root', 1),
            revision('main-head', 4, 'root'),
            revision('feature-head', 3, 'root'),
        ]

        const branches = [
            branch('main-id', 'main', 'main-head'),
            branch('feature-id', 'feature', 'feature-head'),
        ]

        const first = graph(revisions, branches)
        const second = graph([...revisions].reverse(), [...branches].reverse())

        const originalNodes = first.nodes.map((node) => node.revision.id)
        const originalEdges = first.edges.map((edge) => ({ ...edge }))

        Object.freeze(first.nodes)
        Object.freeze(first.edges)
        Object.freeze(first.branches)

        expect(layoutRevisionGraph(first)).toEqual(layoutRevisionGraph(second))
        expect(first.nodes.map((node) => node.revision.id)).toEqual(originalNodes)
        expect(first.edges).toEqual(originalEdges)
    })

    it('handles a long linear ancestry without recursive traversal', () => {
        const revisions = Array.from({ length: 512 }, (_, index) =>
            revision(`revision-${index}`, index + 1, index === 0 ? null : `revision-${index - 1}`),
        )

        const result = layoutRevisionGraph(
            graph(revisions, [branch('main-id', 'main', 'revision-511')]),
        )

        expect(result.nodes).toHaveLength(512)
        expect(result.edges).toHaveLength(511)
        expect(result.laneCount).toBe(1)
        expect(result.nodes[0]).toEqual({
            revisionId: 'revision-511',
            row: 0,
            lane: 0,
        })
        expect(result.nodes[511]).toEqual({
            revisionId: 'revision-0',
            row: 511,
            lane: 0,
        })
    })
})
