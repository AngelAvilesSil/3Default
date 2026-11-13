import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import { getProject, listProjectBranches, type Project, type ProjectBranch } from '../api/projects'
import { RevisionRequestError } from '../api/revisions'
import { loadProjectHistoryGraph } from '../history/projectHistoryProvider'
import { buildRevisionGraph } from '../history/revisionGraph'
import { createAppRouter } from '../router'
import ProjectOverviewView from '../views/ProjectOverviewView.vue'

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

vi.mock('../history/projectHistoryProvider', async (importOriginal) => {
    const original = await importOriginal<typeof import('../history/projectHistoryProvider')>()
    return {
        ...original,
        loadProjectHistoryGraph: vi.fn<typeof original.loadProjectHistoryGraph>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const mainBranchId = '22222222-2222-4222-8222-222222222222'
const designBranchId = '33333333-3333-4333-8333-333333333333'
const designHeadId = '44444444-4444-4444-8444-444444444444'
const authorId = '55555555-5555-4555-8555-555555555555'

const project: Project = {
    id: projectId,
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private',
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:01:00Z',
}

const mainBranch: ProjectBranch = {
    id: mainBranchId,
    projectId,
    name: 'main',
    headRevisionId: null,
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:00:00Z',
}

const designBranch: ProjectBranch = {
    ...mainBranch,
    id: designBranchId,
    name: 'gripper-redesign',
    headRevisionId: designHeadId,
}

const WorkspaceStub = defineComponent({
    name: 'ProjectFileWorkspace',
    props: {
        projectId: { type: String, required: true },
        branchId: { type: String, default: null },
        headRevisionId: { type: String, default: null },
    },
    template: `
        <div
            data-testid="overview-workspace"
            :data-project-id="projectId"
            :data-branch-id="branchId ?? ''"
            :data-head-revision-id="headRevisionId ?? ''"
        ></div>
    `,
})

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
            createdAt: '2026-10-11T12:00:00Z',
            updatedAt: '2026-10-11T12:00:00Z',
        },
    })

    vi.mocked(getProject).mockResolvedValue(project)
    vi.mocked(listProjectBranches).mockResolvedValue([mainBranch, designBranch])

    vi.mocked(loadProjectHistoryGraph).mockResolvedValue(
        buildRevisionGraph({
            projectId,
            revisions: [
                {
                    id: designHeadId,
                    projectId,
                    authorUserId: authorId,
                    authorDisplayName: 'Engineer',
                    message: 'Alternative gripper design',
                    createdAt: '2026-10-11T12:30:00Z',
                    parentRevisionId: null,
                    mergeParentRevisionId: null,
                },
            ],
            branches: [mainBranch, designBranch],
            coverage: {
                source: 'branch-head-reachable',
                status: 'complete',
                failedBranchIds: [],
            },
        }),
    )
})

async function mountOverview() {
    const router = createAppRouter(createMemoryHistory())

    await router.push({
        name: 'project-overview',
        params: { projectId },
        query: { branchId: designBranchId },
    })
    await router.isReady()

    const wrapper = mount(ProjectOverviewView, {
        global: {
            plugins: [pinia, router],
            stubs: {
                ProjectFileWorkspace: WorkspaceStub,
            },
        },
    })

    await vi.waitFor(() => {
        expect(wrapper.get('[data-testid="overview-workspace"]').attributes('data-branch-id')).toBe(
            designBranchId,
        )
    })

    await flushPromises()

    return { wrapper, router }
}

describe('Project Overview history panel shell', () => {
    it('starts collapsed without eagerly loading history', async () => {
        const { wrapper } = await mountOverview()

        try {
            const toggle = wrapper.get('button[aria-controls="project-overview-history-panel"]')

            expect(toggle.attributes('aria-expanded')).toBe('false')

            const panel = wrapper.get('#project-overview-history-panel')
            expect(panel.isVisible()).toBe(false)

            expect(loadProjectHistoryGraph).not.toHaveBeenCalled()
        } finally {
            wrapper.unmount()
        }
    })

    it('opens and closes without changing the workspace branch', async () => {
        const { wrapper, router } = await mountOverview()

        try {
            const toggle = wrapper.get('button[aria-controls="project-overview-history-panel"]')

            const workspace = wrapper.get('[data-testid="overview-workspace"]')

            await toggle.trigger('click')

            expect(toggle.attributes('aria-expanded')).toBe('true')
            expect(wrapper.get('#project-overview-history-panel').isVisible()).toBe(true)

            expect(router.currentRoute.value.query.branchId).toBe(designBranchId)
            expect(workspace.attributes('data-branch-id')).toBe(designBranchId)
            expect(workspace.attributes('data-head-revision-id')).toBe(designHeadId)

            await toggle.trigger('click')

            expect(toggle.attributes('aria-expanded')).toBe('false')
            expect(wrapper.get('#project-overview-history-panel').isVisible()).toBe(false)

            expect(router.currentRoute.value.query.branchId).toBe(designBranchId)
            expect(workspace.attributes('data-branch-id')).toBe(designBranchId)
        } finally {
            wrapper.unmount()
        }
    })

    it('loads all branch-head-reachable histories only when opened', async () => {
        const { wrapper } = await mountOverview()

        try {
            expect(loadProjectHistoryGraph).not.toHaveBeenCalled()

            const toggle = wrapper.get('button[aria-controls="project-overview-history-panel"]')

            await toggle.trigger('click')

            await vi.waitFor(() => {
                expect(loadProjectHistoryGraph).toHaveBeenCalledTimes(1)
            })

            expect(loadProjectHistoryGraph).toHaveBeenCalledWith(
                projectId,
                [mainBranch, designBranch],
                expect.any(AbortSignal),
            )
        } finally {
            wrapper.unmount()
        }
    })

    it('aborts pending history loading when closed and ignores late results', async () => {
        let resolveHistory!: (value: Awaited<ReturnType<typeof loadProjectHistoryGraph>>) => void

        const pendingHistory = new Promise<Awaited<ReturnType<typeof loadProjectHistoryGraph>>>(
            (resolve) => {
                resolveHistory = resolve
            },
        )

        vi.mocked(loadProjectHistoryGraph).mockReturnValueOnce(pendingHistory)

        const { wrapper } = await mountOverview()

        try {
            const toggle = wrapper.get('button[aria-controls="project-overview-history-panel"]')

            await toggle.trigger('click')

            await vi.waitFor(() => {
                expect(loadProjectHistoryGraph).toHaveBeenCalledTimes(1)
            })

            const signal = vi.mocked(loadProjectHistoryGraph).mock.calls[0]?.[2]

            expect(signal).toBeInstanceOf(AbortSignal)
            expect(signal?.aborted).toBe(false)

            await toggle.trigger('click')

            expect(toggle.attributes('aria-expanded')).toBe('false')
            expect(signal?.aborted).toBe(true)

            resolveHistory(
                buildRevisionGraph({
                    projectId,
                    revisions: [
                        {
                            id: designHeadId,
                            projectId,
                            authorUserId: authorId,
                            authorDisplayName: 'Engineer',
                            message: 'Alternative gripper design',
                            createdAt: '2026-10-11T12:30:00Z',
                            parentRevisionId: null,
                            mergeParentRevisionId: null,
                        },
                    ],
                    branches: [mainBranch, designBranch],
                    coverage: {
                        source: 'branch-head-reachable',
                        status: 'complete',
                        failedBranchIds: [],
                    },
                }),
            )

            await flushPromises()

            expect(toggle.attributes('aria-expanded')).toBe('false')
            expect(wrapper.get('#project-overview-history-panel').text()).not.toContain(
                'loaded from branch-head-reachable history',
            )

            expect(loadProjectHistoryGraph).toHaveBeenCalledTimes(1)
        } finally {
            wrapper.unmount()
        }
    })

    it('allows retry after a history request failure', async () => {
        vi.mocked(loadProjectHistoryGraph).mockRejectedValueOnce(
            new TypeError('Network unavailable'),
        )

        const { wrapper, router } = await mountOverview()

        try {
            const toggle = wrapper.get('button[aria-controls="project-overview-history-panel"]')

            await toggle.trigger('click')

            await vi.waitFor(() => {
                expect(
                    wrapper.get('#project-overview-history-panel [role="alert"]').text(),
                ).toContain('Unable to load combined branch history')
            })

            expect(loadProjectHistoryGraph).toHaveBeenCalledTimes(1)

            await wrapper.get('#project-overview-history-panel button').trigger('click')

            await vi.waitFor(() => {
                expect(loadProjectHistoryGraph).toHaveBeenCalledTimes(2)
                expect(wrapper.get('#project-overview-history-panel').text()).toContain(
                    '1 revision loaded',
                )
            })

            expect(wrapper.find('#project-overview-history-panel [role="alert"]').exists()).toBe(
                false,
            )

            expect(router.currentRoute.value.query.branchId).toBe(designBranchId)
            expect(
                wrapper
                    .get('[data-testid="overview-workspace"]')
                    .attributes('data-head-revision-id'),
            ).toBe(designHeadId)
        } finally {
            wrapper.unmount()
        }
    })

    it('redirects an expired history session to login', async () => {
        vi.mocked(loadProjectHistoryGraph).mockRejectedValueOnce(
            new RevisionRequestError(401, 'Unauthorized'),
        )

        const { wrapper, router } = await mountOverview()

        try {
            const returnPath = router.currentRoute.value.fullPath

            const toggle = wrapper.get('button[aria-controls="project-overview-history-panel"]')

            await toggle.trigger('click')

            await vi.waitFor(() => {
                expect(router.currentRoute.value.name).toBe('login')
            })

            expect(router.currentRoute.value.query.redirect).toBe(returnPath)
            expect(loadProjectHistoryGraph).toHaveBeenCalledTimes(1)

            expect(wrapper.find('#project-overview-history-panel').exists()).toBe(false)
        } finally {
            wrapper.unmount()
        }
    })

    it("discards an old project's history after project navigation", async () => {
        const nextProjectId = '66666666-6666-4666-8666-666666666666'
        const nextBranchId = '77777777-7777-4777-8777-777777777777'

        vi.mocked(getProject).mockImplementation(async (requestedProjectId) => {
            if (requestedProjectId === nextProjectId) {
                return {
                    ...project,
                    id: nextProjectId,
                    name: 'Gearbox',
                }
            }

            return project
        })

        vi.mocked(listProjectBranches).mockImplementation(async (requestedProjectId) => {
            if (requestedProjectId === nextProjectId) {
                return [
                    {
                        ...mainBranch,
                        id: nextBranchId,
                        projectId: nextProjectId,
                    },
                ]
            }

            return [mainBranch, designBranch]
        })

        let resolveHistory!: (value: Awaited<ReturnType<typeof loadProjectHistoryGraph>>) => void

        const pendingHistory = new Promise<Awaited<ReturnType<typeof loadProjectHistoryGraph>>>(
            (resolve) => {
                resolveHistory = resolve
            },
        )

        vi.mocked(loadProjectHistoryGraph).mockReturnValueOnce(pendingHistory)

        const { wrapper, router } = await mountOverview()

        try {
            const toggle = wrapper.get('button[aria-controls="project-overview-history-panel"]')

            await toggle.trigger('click')

            await vi.waitFor(() => {
                expect(loadProjectHistoryGraph).toHaveBeenCalledTimes(1)
            })

            const signal = vi.mocked(loadProjectHistoryGraph).mock.calls[0]?.[2]

            expect(signal).toBeInstanceOf(AbortSignal)
            expect(signal?.aborted).toBe(false)

            await router.push({
                name: 'project-overview',
                params: { projectId: nextProjectId },
            })

            await vi.waitFor(() => {
                expect(
                    wrapper.get('[data-testid="overview-workspace"]').attributes('data-project-id'),
                ).toBe(nextProjectId)
            })

            expect(signal?.aborted).toBe(true)
            expect(router.currentRoute.value.params.projectId).toBe(nextProjectId)

            resolveHistory(
                buildRevisionGraph({
                    projectId,
                    revisions: [
                        {
                            id: designHeadId,
                            projectId,
                            authorUserId: authorId,
                            authorDisplayName: 'Engineer',
                            message: 'Alternative gripper design',
                            createdAt: '2026-10-11T12:30:00Z',
                            parentRevisionId: null,
                            mergeParentRevisionId: null,
                        },
                    ],
                    branches: [mainBranch, designBranch],
                    coverage: {
                        source: 'branch-head-reachable',
                        status: 'complete',
                        failedBranchIds: [],
                    },
                }),
            )

            await flushPromises()

            const currentToggle = wrapper.get(
                'button[aria-controls="project-overview-history-panel"]',
            )

            expect(currentToggle.attributes('aria-expanded')).toBe('false')
            expect(wrapper.get('#project-overview-history-panel').isVisible()).toBe(false)

            expect(wrapper.get('#project-overview-history-panel').text()).not.toContain(
                'loaded from branch-head-reachable history',
            )

            expect(
                wrapper.get('[data-testid="overview-workspace"]').attributes('data-branch-id'),
            ).toBe(nextBranchId)

            expect(loadProjectHistoryGraph).toHaveBeenCalledTimes(1)
        } finally {
            wrapper.unmount()
        }
    })

    it('warns about partial branch coverage and supports retry', async () => {
        const mainHeadId = '88888888-8888-4888-8888-888888888888'
        const mainWithHead = {
            ...mainBranch,
            headRevisionId: mainHeadId,
        }

        vi.mocked(listProjectBranches).mockResolvedValueOnce([mainWithHead, designBranch])

        const designRevision = {
            id: designHeadId,
            projectId,
            authorUserId: authorId,
            authorDisplayName: 'Engineer',
            message: 'Alternative gripper design',
            createdAt: '2026-10-11T12:30:00Z',
            parentRevisionId: null,
            mergeParentRevisionId: null,
        }

        vi.mocked(loadProjectHistoryGraph)
            .mockResolvedValueOnce(
                buildRevisionGraph({
                    projectId,
                    revisions: [designRevision],
                    branches: [mainWithHead, designBranch],
                    coverage: {
                        source: 'branch-head-reachable',
                        status: 'partial',
                        failedBranchIds: [mainBranchId],
                    },
                }),
            )
            .mockResolvedValueOnce(
                buildRevisionGraph({
                    projectId,
                    revisions: [
                        designRevision,
                        {
                            ...designRevision,
                            id: mainHeadId,
                            message: 'Main branch revision',
                        },
                    ],
                    branches: [mainWithHead, designBranch],
                    coverage: {
                        source: 'branch-head-reachable',
                        status: 'complete',
                        failedBranchIds: [],
                    },
                }),
            )

        const { wrapper, router } = await mountOverview()

        try {
            const toggle = wrapper.get('button[aria-controls="project-overview-history-panel"]')

            await toggle.trigger('click')

            await vi.waitFor(() => {
                expect(
                    wrapper.get('#project-overview-history-panel [role="alert"]').text(),
                ).toContain('Revision history is incomplete.')
            })

            const panel = wrapper.get('#project-overview-history-panel')

            expect(panel.text()).toContain('1 revision')
            expect(panel.text()).toContain('Try again')
            expect(router.currentRoute.value.query.branchId).toBe(designBranchId)

            await panel.get('button').trigger('click')

            await vi.waitFor(() => {
                expect(loadProjectHistoryGraph).toHaveBeenCalledTimes(2)
                expect(panel.text()).toContain('2 revisions')
                expect(panel.find('[role="alert"]').exists()).toBe(false)
            })

            expect(router.currentRoute.value.query.branchId).toBe(designBranchId)
            expect(
                wrapper
                    .get('[data-testid="overview-workspace"]')
                    .attributes('data-head-revision-id'),
            ).toBe(designHeadId)
        } finally {
            wrapper.unmount()
        }
    })

    it('warns when ancestry is unresolved without a failed branch request', async () => {
        const missingParentId = '99999999-9999-4999-8999-999999999999'

        const graph = buildRevisionGraph({
            projectId,
            revisions: [
                {
                    id: designHeadId,
                    projectId,
                    authorUserId: authorId,
                    authorDisplayName: 'Engineer',
                    message: 'Alternative gripper design',
                    createdAt: '2026-10-11T12:30:00Z',
                    parentRevisionId: missingParentId,
                    mergeParentRevisionId: null,
                },
            ],
            branches: [mainBranch, designBranch],
            coverage: {
                source: 'branch-head-reachable',
                status: 'complete',
                failedBranchIds: [],
            },
        })

        // Incomplete ancestry alone must produce partial coverage.
        // No branch request has failed in this fixture.
        expect(graph.coverage.status).toBe('partial')
        expect(graph.coverage.failedBranchIds).toEqual([])
        expect(graph.unresolvedEdges).toHaveLength(1)
        expect(graph.unresolvedHeadBranchIds).toEqual([])

        vi.mocked(loadProjectHistoryGraph).mockResolvedValueOnce(graph)

        const { wrapper, router } = await mountOverview()

        try {
            const toggle = wrapper.get('button[aria-controls="project-overview-history-panel"]')

            expect(loadProjectHistoryGraph).not.toHaveBeenCalled()

            await toggle.trigger('click')

            await vi.waitFor(() => {
                expect(
                    wrapper.get('#project-overview-history-panel [role="alert"]').text(),
                ).toContain('Revision history is incomplete.')
            })

            const panel = wrapper.get('#project-overview-history-panel')

            expect(panel.text()).toContain('1 revision loaded')
            expect(panel.text()).toContain('Try again')

            expect(loadProjectHistoryGraph).toHaveBeenCalledTimes(1)
            expect(loadProjectHistoryGraph).toHaveBeenCalledWith(
                projectId,
                [mainBranch, designBranch],
                expect.any(AbortSignal),
            )

            expect(router.currentRoute.value.query.branchId).toBe(designBranchId)

            expect(
                wrapper
                    .get('[data-testid="overview-workspace"]')
                    .attributes('data-head-revision-id'),
            ).toBe(designHeadId)
        } finally {
            wrapper.unmount()
        }
    })
})
