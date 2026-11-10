import { describe, expect, it } from 'vitest'

import {
    buildRevisionGraph,
    getRevisionAncestry,
    type RevisionGraphBranch,
    type RevisionGraphCoverage,
    type RevisionGraphRevision,
} from '../history/revisionGraph'

const projectId = '11111111-1111-4111-8111-111111111111'
const authorUserId = '22222222-2222-4222-8222-222222222222'

const completeReachable: RevisionGraphCoverage = {
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
        authorDisplayName: 'Engineer',
        message: `Revision ${id}`,
        createdAt: `2026-10-${String(day).padStart(2, '0')}T12:00:00Z`,
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
    coverage: RevisionGraphCoverage = completeReachable,
) {
    return buildRevisionGraph({
        projectId,
        revisions,
        branches,
        coverage,
    })
}

describe('normalized revision graph', () => {
    it('supports an empty branch without inventing a revision', () => {
        const empty = branch('branch-main', 'main', null)
        const result = graph([], [empty])

        expect(result.projectId).toBe(projectId)
        expect(result.nodes).toEqual([])
        expect(result.edges).toEqual([])
        expect(result.unresolvedEdges).toEqual([])
        expect(result.unresolvedHeadBranchIds).toEqual([])
        expect(result.branches).toEqual([empty])
        expect(result.coverage).toEqual(completeReachable)
        expect(getRevisionAncestry(result, null)).toEqual([])
    })

    it('preserves actual primary and merge ancestry with branch heads', () => {
        const root = revision('root', 1)
        const main = revision('main-rev', 2, 'root')
        const alternative = revision('alternative-rev', 3, 'root')
        const merged = revision('merge-rev', 4, 'main-rev', 'alternative-rev')

        const result = graph(
            [root, merged, main, alternative],
            [
                branch('branch-main', 'main', 'merge-rev'),
                branch('branch-alt', 'alternative', 'alternative-rev'),
            ],
        )

        expect(result.nodes.map((node) => node.revision.id)).toEqual([
            'merge-rev',
            'alternative-rev',
            'main-rev',
            'root',
        ])

        expect(result.branches.map((item) => item.name)).toEqual(['alternative', 'main'])

        expect(result.edges).toEqual([
            {
                childRevisionId: 'merge-rev',
                parentRevisionId: 'main-rev',
                kind: 'parent',
            },
            {
                childRevisionId: 'merge-rev',
                parentRevisionId: 'alternative-rev',
                kind: 'merge-parent',
            },
            {
                childRevisionId: 'alternative-rev',
                parentRevisionId: 'root',
                kind: 'parent',
            },
            {
                childRevisionId: 'main-rev',
                parentRevisionId: 'root',
                kind: 'parent',
            },
        ])

        expect(
            result.nodes.find((node) => node.revision.id === 'merge-rev')?.headBranchIds,
        ).toEqual(['branch-main'])

        expect(
            result.nodes.find((node) => node.revision.id === 'alternative-rev')?.headBranchIds,
        ).toEqual(['branch-alt'])

        expect(getRevisionAncestry(result, 'merge-rev')).toEqual([
            'merge-rev',
            'alternative-rev',
            'main-rev',
            'root',
        ])
    })

    it('deduplicates shared revisions without duplicating graph nodes', () => {
        const root = revision('root', 1)
        const first = revision('first', 2, 'root')
        const second = revision('second', 3, 'root')

        const result = graph(
            [first, root, second, root, first],
            [branch('branch-main', 'main', 'first'), branch('branch-alt', 'alternative', 'second')],
        )

        expect(result.nodes.map((node) => node.revision.id)).toEqual(['second', 'first', 'root'])

        expect(result.edges).toHaveLength(2)
        expect(result.nodes.filter((node) => node.revision.id === 'root')).toHaveLength(1)
    })

    it('uses deterministic ordering independent of API response order', () => {
        const root = revision('root', 1)
        const alpha = revision('alpha', 2, 'root')
        const beta = revision('beta', 2, 'root')

        const branches = [branch('branch-z', 'zeta', 'beta'), branch('branch-a', 'alpha', 'alpha')]

        const first = graph([beta, root, alpha], branches)
        const second = graph([alpha, beta, root], [...branches].reverse())

        expect(first).toEqual(second)
        expect(first.nodes.map((node) => node.revision.id)).toEqual(['alpha', 'beta', 'root'])
    })

    it('keeps project-wide revisions not reachable from current branch heads', () => {
        const root = revision('root', 1)
        const unreferenced = revision('unreferenced', 2)

        const result = graph([unreferenced, root], [branch('branch-main', 'main', 'root')], {
            source: 'project-wide',
            status: 'complete',
            failedBranchIds: [],
        })

        expect(result.nodes.map((node) => node.revision.id)).toEqual(['unreferenced', 'root'])

        expect(getRevisionAncestry(result, 'root')).toEqual(['root'])
        expect(result.coverage.source).toBe('project-wide')
    })

    it('reports missing ancestors and unresolved branch heads without inventing nodes', () => {
        const child = revision('child', 3, 'missing-primary', 'missing-merge')

        const result = graph(
            [child],
            [
                branch('branch-main', 'main', 'child'),
                branch('branch-other', 'other', 'missing-head'),
            ],
            {
                source: 'branch-head-reachable',
                status: 'partial',
                failedBranchIds: ['branch-other'],
            },
        )

        expect(result.nodes).toHaveLength(1)
        expect(result.edges).toEqual([])
        expect(result.unresolvedEdges).toEqual([
            {
                childRevisionId: 'child',
                parentRevisionId: 'missing-primary',
                kind: 'parent',
            },
            {
                childRevisionId: 'child',
                parentRevisionId: 'missing-merge',
                kind: 'merge-parent',
            },
        ])
        expect(result.unresolvedHeadBranchIds).toEqual(['branch-other'])
        expect(result.coverage.status).toBe('partial')
        expect(result.coverage.failedBranchIds).toEqual(['branch-other'])
    })

    it('cannot claim complete coverage when graph references are missing', () => {
        const result = graph(
            [revision('child', 2, 'unavailable-parent')],
            [branch('branch-main', 'main', 'child')],
        )

        expect(result.edges).toEqual([])
        expect(result.unresolvedEdges).toHaveLength(1)
        expect(result.coverage.status).toBe('partial')
    })

    it('does not mutate input revisions, branch records, or coverage', () => {
        const originalRevision = Object.freeze(revision('root', 1))
        const originalBranch = Object.freeze(branch('branch-main', 'main', 'root'))
        const revisions = Object.freeze([originalRevision])
        const branches = Object.freeze([originalBranch])
        const coverage: RevisionGraphCoverage = Object.freeze({
            source: 'branch-head-reachable',
            status: 'complete',
            failedBranchIds: Object.freeze([] as string[]),
        })

        const result = graph(revisions, branches, coverage)

        expect(result.nodes[0]?.revision).toEqual(originalRevision)
        expect(result.branches[0]).toEqual(originalBranch)
        expect(revisions).toHaveLength(1)
        expect(branches).toHaveLength(1)
    })

    it('rejects cross-project records and conflicting duplicate identities', () => {
        const root = revision('root', 1)
        const main = branch('branch-main', 'main', 'root')

        expect(() => graph([{ ...root, projectId: 'another-project' }], [main])).toThrow(/project/i)

        expect(() => graph([root], [{ ...main, projectId: 'another-project' }])).toThrow(/project/i)

        expect(() =>
            graph([root, { ...root, message: 'Different revision data' }], [main]),
        ).toThrow(/conflicting revision/i)

        expect(() => graph([root], [main, { ...main, headRevisionId: null }])).toThrow(
            /conflicting branch/i,
        )
    })

    it('rejects self-ancestry and cycles involving merge parents', () => {
        expect(() =>
            graph([revision('self', 1, 'self')], [branch('branch-main', 'main', 'self')]),
        ).toThrow(/cycle|self/i)

        expect(() =>
            graph(
                [revision('first', 1, 'second'), revision('second', 2, null, 'first')],
                [branch('branch-main', 'main', 'second')],
            ),
        ).toThrow(/cycle/i)
    })

    it('returns no highlighted ancestry for a revision outside the loaded graph', () => {
        const result = graph([revision('root', 1)], [branch('branch-main', 'main', 'root')])

        expect(getRevisionAncestry(result, 'missing')).toEqual([])
    })
})
