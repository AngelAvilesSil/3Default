import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import RevisionGraph from '../components/history/RevisionGraph.vue'
import {
    buildRevisionGraph,
    type RevisionGraphBranch,
    type RevisionGraphCoverage,
    type RevisionGraphRevision,
} from '../history/revisionGraph'

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
    coverage: RevisionGraphCoverage = completeCoverage,
) {
    return buildRevisionGraph({
        projectId,
        revisions,
        branches,
        coverage,
    })
}

const root = revision('root', 1)
const mainHead = revision('main-head', 4, 'root')
const featureHead = revision('feature-head', 3, 'root')

function branchingGraph() {
    return graph(
        [root, mainHead, featureHead],
        [branch('main-id', 'main', 'main-head'), branch('feature-id', 'feature', 'feature-head')],
    )
}

describe('read-only visual revision graph', () => {
    it('shows an empty state without manufacturing revision nodes', () => {
        const wrapper = mount(RevisionGraph, {
            props: {
                graph: graph([], [branch('main-id', 'main', null)]),
            },
        })

        expect(wrapper.get('[role="status"]').text()).toContain('No revisions')
        expect(wrapper.findAll('button[data-revision-id]')).toHaveLength(0)
        expect(wrapper.find('svg').exists()).toBe(false)

        wrapper.unmount()
    })

    it('shows revision messages, authors, timestamps, and accessible IDs', () => {
        const wrapper = mount(RevisionGraph, {
            props: {
                graph: graph([root], [branch('main-id', 'main', 'root')]),
            },
        })

        const button = wrapper.get('button[data-revision-id="root"]')

        expect(button.text()).toContain('Revision root')
        expect(button.text()).toContain('Engineer')
        expect(button.text()).toContain('2026-10-01')
        expect(button.attributes('aria-label')).toContain('root')
        expect(button.attributes('type')).toBe('button')
        expect(wrapper.get('time').attributes('datetime')).toBe(root.createdAt)

        wrapper.unmount()
    })

    it('follows ancestry order even when parent timestamps are newer', () => {
        const parent = revision('parent', 9)
        const child = revision('child', 1, 'parent')

        const wrapper = mount(RevisionGraph, {
            props: {
                graph: graph([parent, child], [branch('main-id', 'main', 'child')]),
            },
        })

        const buttons = wrapper.findAll('button[data-revision-id]')

        expect(buttons.map((button) => button.attributes('data-revision-id'))).toEqual([
            'child',
            'parent',
        ])

        wrapper.unmount()
    })

    it('uses the rows and lanes supplied by the V.3a layout engine', () => {
        const wrapper = mount(RevisionGraph, {
            props: { graph: branchingGraph() },
        })

        expect(wrapper.get('svg').attributes('aria-hidden')).toBe('true')

        const main = wrapper.get('button[data-revision-id="main-head"]')
        const feature = wrapper.get('button[data-revision-id="feature-head"]')
        const ancestor = wrapper.get('button[data-revision-id="root"]')

        expect(main.attributes('data-lane')).toBe('0')
        expect(feature.attributes('data-lane')).toBe('1')
        expect(ancestor.attributes('data-lane')).toBe('0')

        expect(wrapper.findAll('circle[data-revision-id]')).toHaveLength(3)
        expect(wrapper.findAll('path[data-kind="parent"]')).toHaveLength(2)

        wrapper.unmount()
    })

    it('labels branch heads without duplicating shared ancestors', () => {
        const wrapper = mount(RevisionGraph, {
            props: { graph: branchingGraph() },
        })

        const main = wrapper.get('button[data-revision-id="main-head"]')
        const feature = wrapper.get('button[data-revision-id="feature-head"]')
        const ancestor = wrapper.get('button[data-revision-id="root"]')

        expect(main.get('.revision-graph__head').text()).toContain('main')
        expect(feature.get('.revision-graph__head').text()).toContain('feature')
        expect(ancestor.find('.revision-graph__head').exists()).toBe(false)
        expect(wrapper.findAll('button[data-revision-id="root"]')).toHaveLength(1)

        wrapper.unmount()
    })

    it('represents primary and merge-parent connections distinctly', () => {
        const merged = revision('merge-head', 5, 'main-head', 'feature-head')

        const wrapper = mount(RevisionGraph, {
            props: {
                graph: graph(
                    [root, mainHead, featureHead, merged],
                    [
                        branch('main-id', 'main', 'merge-head'),
                        branch('feature-id', 'feature', 'feature-head'),
                    ],
                ),
            },
        })

        const paths = wrapper.findAll('path[data-child-id="merge-head"]')

        expect(paths).toHaveLength(2)
        expect(paths.map((path) => path.attributes('data-kind')).sort()).toEqual([
            'merge-parent',
            'parent',
        ])
        expect(paths.every((path) => Boolean(path.attributes('d')))).toBe(true)

        wrapper.unmount()
    })

    it('does not invent missing ancestors or missing branch heads', () => {
        const partial = graph(
            [revision('child', 4, 'missing-parent')],
            [branch('main-id', 'main', 'child'), branch('other-id', 'other', 'missing-head')],
            {
                source: 'branch-head-reachable',
                status: 'partial',
                failedBranchIds: ['other-id'],
            },
        )

        const wrapper = mount(RevisionGraph, {
            props: { graph: partial },
        })

        expect(wrapper.findAll('button[data-revision-id]')).toHaveLength(1)
        expect(wrapper.findAll('path[data-unresolved="true"]')).toHaveLength(1)
        expect(wrapper.find('button[data-revision-id="missing-parent"]').exists()).toBe(false)
        expect(wrapper.find('button[data-revision-id="missing-head"]').exists()).toBe(false)
        expect(wrapper.get('.revision-graph__partial').text()).toContain('incomplete')

        wrapper.unmount()
    })

    it('distinguishes branch-reachable and project-wide coverage', async () => {
        const wrapper = mount(RevisionGraph, {
            props: { graph: branchingGraph() },
        })

        expect(wrapper.get('.revision-graph__coverage').text()).toContain('Branch-head-reachable')

        await wrapper.setProps({
            graph: graph([root], [branch('main-id', 'main', 'root')], {
                source: 'project-wide',
                status: 'complete',
                failedBranchIds: [],
            }),
        })

        expect(wrapper.get('.revision-graph__coverage').text()).toContain('Project-wide')
        expect(wrapper.get('.revision-graph__coverage').text()).not.toContain(
            'Branch-head-reachable',
        )

        wrapper.unmount()
    })

    it('emits revision selection without owning selection state', async () => {
        const wrapper = mount(RevisionGraph, {
            props: { graph: branchingGraph() },
        })

        const button = wrapper.get('button[data-revision-id="feature-head"]')

        expect(button.attributes('aria-pressed')).toBe('false')

        await button.trigger('click')

        expect(wrapper.emitted('select-revision')).toEqual([['feature-head']])
        expect(button.attributes('aria-pressed')).toBe('false')

        wrapper.unmount()
    })

    it('highlights only the selected revision and its loaded ancestors', () => {
        const wrapper = mount(RevisionGraph, {
            props: {
                graph: branchingGraph(),
                selectedRevisionId: 'main-head',
            },
        })

        const selected = wrapper.get('button[data-revision-id="main-head"]')
        const ancestor = wrapper.get('button[data-revision-id="root"]')
        const unrelated = wrapper.get('button[data-revision-id="feature-head"]')

        expect(selected.attributes('aria-pressed')).toBe('true')
        expect(ancestor.attributes('aria-pressed')).toBe('false')
        expect(ancestor.classes()).toContain('revision-graph__revision--ancestor')
        expect(unrelated.classes()).not.toContain('revision-graph__revision--ancestor')

        wrapper.unmount()
    })

    it('does not highlight an unknown revision selection', () => {
        const wrapper = mount(RevisionGraph, {
            props: {
                graph: branchingGraph(),
                selectedRevisionId: 'unavailable',
            },
        })

        expect(wrapper.findAll('button[aria-pressed="true"]')).toHaveLength(0)
        expect(wrapper.findAll('.revision-graph__revision--ancestor')).toHaveLength(0)

        wrapper.unmount()
    })

    it('reacts to changed graph and selection props', async () => {
        const wrapper = mount(RevisionGraph, {
            props: {
                graph: branchingGraph(),
                selectedRevisionId: 'main-head',
            },
        })

        await wrapper.setProps({
            graph: graph([root], [branch('main-id', 'main', 'root')]),
            selectedRevisionId: 'root',
        })

        expect(wrapper.findAll('button[data-revision-id]')).toHaveLength(1)
        expect(wrapper.get('button[data-revision-id="root"]').attributes('aria-pressed')).toBe(
            'true',
        )
        expect(wrapper.find('button[data-revision-id="main-head"]').exists()).toBe(false)
        expect(wrapper.findAll('circle[data-revision-id]')).toHaveLength(1)

        wrapper.unmount()
    })
})
