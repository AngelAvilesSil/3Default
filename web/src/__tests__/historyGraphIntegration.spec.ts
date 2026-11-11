import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import { getProject, listProjectBranches, type Project, type ProjectBranch } from '../api/projects'
import {
    listProjectBranchHistory,
    RevisionRequestError,
    type ProjectRevision,
} from '../api/revisions'
import { createAppRouter } from '../router'
import BranchHistoryView from '../views/BranchHistoryView.vue'

vi.mock('../api/auth', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/auth')>()

    return {
        ...original,
        getCurrentUser: vi.fn<typeof original.getCurrentUser>(),
    }
})

vi.mock('../api/projects', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/projects')>()

    return {
        ...original,
        getProject: vi.fn<typeof original.getProject>(),
        listProjectBranches: vi.fn<typeof original.listProjectBranches>(),
    }
})

vi.mock('../api/revisions', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/revisions')>()

    return {
        ...original,
        listProjectBranchHistory: vi.fn<typeof original.listProjectBranchHistory>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const mainBranchId = '22222222-2222-4222-8222-222222222222'
const rootId = '33333333-3333-4333-8333-333333333333'
const mainHeadId = '44444444-4444-4444-8444-444444444444'
const authorId = '55555555-5555-4555-8555-555555555555'
const featureBranchId = '66666666-6666-4666-8666-666666666666'
const featureHeadId = '77777777-7777-4777-8777-777777777777'

const project: Project = {
    id: projectId,
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private',
    createdAt: '2026-10-19T12:00:00Z',
    updatedAt: '2026-10-19T12:00:00Z',
}

const mainBranch: ProjectBranch = {
    id: mainBranchId,
    projectId,
    name: 'main',
    headRevisionId: mainHeadId,
    createdAt: '2026-10-19T12:00:00Z',
    updatedAt: '2026-10-19T12:30:00Z',
}

const featureBranch: ProjectBranch = {
    ...mainBranch,
    id: featureBranchId,
    name: 'reinforced',
    headRevisionId: featureHeadId,
}

const root: ProjectRevision = {
    id: rootId,
    projectId,
    authorUserId: authorId,
    authorDisplayName: 'Engineer',
    message: 'Initial gripper design',
    parentRevisionId: null,
    mergeParentRevisionId: null,
    createdAt: '2026-10-19T12:00:00Z',
}

const mainHead: ProjectRevision = {
    ...root,
    id: mainHeadId,
    message: 'Add mounting holes',
    parentRevisionId: rootId,
    createdAt: '2026-10-19T12:30:00Z',
}

const featureHead: ProjectRevision = {
    ...root,
    id: featureHeadId,
    message: 'Alternative reinforced geometry',
    parentRevisionId: rootId,
    createdAt: '2026-10-19T12:20:00Z',
}

let pinia: Pinia

beforeEach(() => {
    vi.resetAllMocks()

    pinia = createPinia()
    setActivePinia(pinia)

    vi.mocked(getCurrentUser).mockResolvedValue({
        status: 'authenticated',
        user: {
            id: authorId,
            email: 'engineer@example.com',
            displayName: 'Engineer',
            createdAt: '2026-10-19T10:00:00Z',
            updatedAt: '2026-10-19T10:00:00Z',
        },
    })

    vi.mocked(getProject).mockResolvedValue(project)
    vi.mocked(listProjectBranches).mockResolvedValue([mainBranch, featureBranch])

    vi.mocked(listProjectBranchHistory).mockImplementation(
        async (_projectId, requestedBranchId) => {
            if (requestedBranchId === mainBranchId) {
                return [mainHead, root]
            }

            if (requestedBranchId === featureBranchId) {
                return [featureHead, root]
            }

            throw new RevisionRequestError(404, 'Not Found')
        },
    )
})

async function mountHistory() {
    const router = createAppRouter(createMemoryHistory())

    await router.push(`/projects/${projectId}/branches/${mainBranchId}/history`)
    await router.isReady()

    const wrapper = mount(BranchHistoryView, {
        props: {
            projectId,
            branchId: mainBranchId,
        },
        global: {
            plugins: [pinia, router],
        },
    })

    await vi.waitFor(() => {
        expect(wrapper.get('.branch-history__context').text()).toContain('main')
    })

    return { wrapper, router }
}

describe('GitLab-inspired project history integration', () => {
    it('shows the selected branch as a visual revision graph', async () => {
        const { wrapper, router } = await mountHistory()

        await vi.waitFor(() => {
            expect(wrapper.findAll('.revision-graph__revision')).toHaveLength(2)
        })

        expect(wrapper.get(`button[data-revision-id="${mainHeadId}"]`)).toBeTruthy()

        expect(wrapper.get(`button[data-revision-id="${rootId}"]`)).toBeTruthy()

        expect(wrapper.find(`button[data-revision-id="${featureHeadId}"]`).exists()).toBe(false)

        expect(router.currentRoute.value.name).toBe('project-branch-history')

        wrapper.unmount()
    })

    it('does not eagerly request another branch history', async () => {
        const { wrapper } = await mountHistory()

        await vi.waitFor(() => {
            expect(wrapper.findAll('.revision-graph__revision')).toHaveLength(2)
        })

        expect(listProjectBranchHistory).toHaveBeenCalledTimes(1)

        expect(listProjectBranchHistory).toHaveBeenCalledWith(
            projectId,
            mainBranchId,
            expect.any(AbortSignal),
        )

        expect(
            vi
                .mocked(listProjectBranchHistory)
                .mock.calls.some((call) => call[1] === featureBranchId),
        ).toBe(false)

        wrapper.unmount()
    })

    it('selects a revision without navigation and offers explicit inspection', async () => {
        const { wrapper, router } = await mountHistory()

        await vi.waitFor(() => {
            expect(wrapper.findAll('.revision-graph__revision')).toHaveLength(2)
        })

        expect(wrapper.find('a.branch-history__selected-inspect').exists()).toBe(false)

        const revision = wrapper.get(`button[data-revision-id="${mainHeadId}"]`)

        await revision.trigger('click')

        expect(revision.attributes('aria-pressed')).toBe('true')

        expect(router.currentRoute.value.name).toBe('project-branch-history')

        const inspect = wrapper.get('a.branch-history__selected-inspect')

        expect(inspect.attributes('href')).toContain(
            `/projects/${projectId}/revisions/${mainHeadId}`,
        )

        expect(inspect.attributes('href')).toContain(`fromBranchId=${mainBranchId}`)

        wrapper.unmount()
    })

    it('loads all accessible branch-head histories only after an explicit action', async () => {
        const { wrapper, router } = await mountHistory()

        expect(
            vi
                .mocked(listProjectBranchHistory)
                .mock.calls.some((call) => call[1] === featureBranchId),
        ).toBe(false)

        await wrapper.get('button[aria-label="Show all branches"]').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.findAll('.revision-graph__revision')).toHaveLength(3)
        })

        expect(listProjectBranchHistory).toHaveBeenCalledWith(
            projectId,
            featureBranchId,
            expect.any(AbortSignal),
        )

        expect(wrapper.get(`button[data-revision-id="${featureHeadId}"]`)).toBeTruthy()

        expect(wrapper.findAll(`button[data-revision-id="${rootId}"]`)).toHaveLength(1)

        expect(router.currentRoute.value.params.branchId).toBe(mainBranchId)

        wrapper.unmount()
    })

    it('returns to selected branch without refetching an already loaded scope', async () => {
        const { wrapper } = await mountHistory()

        await wrapper.get('button[aria-label="Show all branches"]').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.findAll('.revision-graph__revision')).toHaveLength(3)
        })

        const callsAfterAll = vi.mocked(listProjectBranchHistory).mock.calls.length

        await wrapper.get('button[aria-label="Show selected branch"]').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.findAll('.revision-graph__revision')).toHaveLength(2)
        })

        expect(wrapper.find(`button[data-revision-id="${featureHeadId}"]`).exists()).toBe(false)

        expect(listProjectBranchHistory).toHaveBeenCalledTimes(callsAfterAll)

        wrapper.unmount()
    })

    it('warns about partial all-branches coverage without inventing missing nodes', async () => {
        vi.mocked(listProjectBranchHistory).mockImplementation(
            async (_projectId, requestedBranchId) => {
                if (requestedBranchId === featureBranchId) {
                    throw new RevisionRequestError(503, 'Unavailable')
                }

                return [mainHead, root]
            },
        )

        const { wrapper } = await mountHistory()

        await wrapper.get('button[aria-label="Show all branches"]').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.get('.revision-graph__partial').text()).toContain('incomplete')
        })

        expect(wrapper.find(`button[data-revision-id="${featureHeadId}"]`).exists()).toBe(false)

        expect(wrapper.findAll('.revision-graph__revision')).toHaveLength(2)

        wrapper.unmount()
    })

    it('does not claim branch-head-reachable history is exhaustive project history', async () => {
        const { wrapper } = await mountHistory()

        await wrapper.get('button[aria-label="Show all branches"]').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.findAll('.revision-graph__revision')).toHaveLength(3)
        })

        expect(wrapper.get('.revision-graph__coverage').text()).toContain('Branch-head-reachable')

        expect(wrapper.get('.revision-graph__coverage').text()).not.toContain('Project-wide')

        wrapper.unmount()
    })

    it('inspects a revision from another branch without switching the active branch', async () => {
        const { wrapper, router } = await mountHistory()

        await wrapper.get('button[aria-label="Show all branches"]').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.get(`button[data-revision-id="${featureHeadId}"]`)).toBeTruthy()
        })

        await wrapper.get(`button[data-revision-id="${featureHeadId}"]`).trigger('click')

        expect(router.currentRoute.value.params.branchId).toBe(mainBranchId)

        const inspect = wrapper.get('a.branch-history__selected-inspect')

        expect(inspect.attributes('href')).toContain(
            `/projects/${projectId}/revisions/${featureHeadId}`,
        )

        expect(inspect.attributes('href')).toContain(`fromBranchId=${mainBranchId}`)

        wrapper.unmount()
    })

    it('distinguishes total all-branch loading failure from empty history and supports retry', async () => {
        let initialSelectedBranchRequest = true

        vi.mocked(listProjectBranchHistory).mockImplementation(
            async (_requestedProjectId, requestedBranchId) => {
                if (initialSelectedBranchRequest && requestedBranchId === mainBranchId) {
                    initialSelectedBranchRequest = false
                    return [mainHead, root]
                }

                throw new RevisionRequestError(503, 'Unavailable')
            },
        )

        const { wrapper, router } = await mountHistory()

        await wrapper.get('button[aria-label="Show all branches"]').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.get('.branch-history__feedback [role="alert"]').text()).toContain(
                'Unable to load any branch revision history',
            )
        })

        expect(wrapper.findAll('.revision-graph__revision')).toHaveLength(0)

        expect(wrapper.text()).not.toContain(
            'No revisions were found in the loaded branch histories',
        )

        expect(router.currentRoute.value.params.branchId).toBe(mainBranchId)

        vi.mocked(listProjectBranchHistory).mockImplementation(
            async (_requestedProjectId, requestedBranchId) => {
                if (requestedBranchId === mainBranchId) {
                    return [mainHead, root]
                }

                if (requestedBranchId === featureBranchId) {
                    return [featureHead, root]
                }

                throw new RevisionRequestError(404, 'Not Found')
            },
        )

        await wrapper.get('.branch-history__feedback button').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.findAll('.revision-graph__revision')).toHaveLength(3)
        })

        expect(wrapper.find('.branch-history__feedback').exists()).toBe(false)
        expect(router.currentRoute.value.params.branchId).toBe(mainBranchId)

        wrapper.unmount()
    })
})
