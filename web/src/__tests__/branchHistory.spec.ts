import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import {
    getProject,
    listProjectBranches,
    ProjectRequestError,
    type Project,
    type ProjectBranch,
} from '../api/projects'
import {
    listProjectBranchHistory,
    RevisionRequestError,
    type ProjectRevision,
} from '../api/revisions'
import { createAppRouter } from '../router'
import BranchHistoryView from '../views/BranchHistoryView.vue'
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

vi.mock('../api/revisions', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/revisions')>()
    return {
        ...original,
        listProjectBranchHistory: vi.fn<typeof original.listProjectBranchHistory>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const branchId = '22222222-2222-4222-8222-222222222222'
const rootId = '33333333-3333-4333-8333-333333333333'
const headId = '44444444-4444-4444-8444-444444444444'

const project: Project = {
    id: projectId,
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private',
    createdAt: '2026-10-19T12:00:00Z',
    updatedAt: '2026-10-19T12:00:00Z',
}

const branch: ProjectBranch = {
    id: branchId,
    projectId,
    name: 'main',
    headRevisionId: headId,
    createdAt: '2026-10-19T12:00:00Z',
    updatedAt: '2026-10-19T12:30:00Z',
}

const root: ProjectRevision = {
    id: rootId,
    projectId,
    authorUserId: '55555555-5555-4555-8555-555555555555',
    message: 'Initial gripper design',
    parentRevisionId: null,
    mergeParentRevisionId: null,
    createdAt: '2026-10-19T12:00:00Z',
}

const head: ProjectRevision = {
    ...root,
    id: headId,
    message: 'Add mounting holes',
    parentRevisionId: rootId,
    createdAt: '2026-10-19T12:30:00Z',
}

const user = {
    id: root.authorUserId,
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-10-19T10:00:00Z',
    updatedAt: '2026-10-19T10:00:00Z',
}

let pinia: Pinia

beforeEach(() => {
    vi.resetAllMocks()
    pinia = createPinia()
    setActivePinia(pinia)

    vi.mocked(getCurrentUser).mockResolvedValue({
        status: 'authenticated',
        user,
    })
    vi.mocked(getProject).mockResolvedValue(project)
    vi.mocked(listProjectBranches).mockResolvedValue([branch])
    vi.mocked(listProjectBranchHistory).mockResolvedValue([head, root])
})

async function mountHistory() {
    const router = createAppRouter(createMemoryHistory())
    await router.push(`/projects/${projectId}/branches/${branchId}/history`)
    await router.isReady()

    const wrapper = mount(BranchHistoryView, {
        props: { projectId, branchId },
        global: { plugins: [pinia, router] },
    })

    return { wrapper, router }
}

async function mountOverview() {
    const router = createAppRouter(createMemoryHistory())
    await router.push(`/projects/${projectId}`)
    await router.isReady()

    const wrapper = mount(ProjectOverviewView, {
        global: {
            plugins: [pinia, router],
            stubs: { ProjectFileWorkspace: true },
        },
    })

    return { wrapper, router }
}

describe('branch history route and navigation', () => {
    it('registers a protected lazy-loaded route with explicit identifiers', () => {
        const router = createAppRouter(createMemoryHistory())
        const route = router.getRoutes().find((item) => item.name === 'project-branch-history')

        expect(route?.path).toBe('/projects/:projectId/branches/:branchId/history')
        expect(route?.meta.requiresAuth).toBe(true)
        expect(route?.props.default).toBe(true)
        expect(typeof route?.components?.default).toBe('function')
    })

    it('requires authentication for a direct history URL', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })

        const router = createAppRouter(createMemoryHistory())
        const path = `/projects/${projectId}/branches/${branchId}/history`

        await router.push(path)

        expect(router.currentRoute.value.name).toBe('login')
        expect(router.currentRoute.value.query.redirect).toBe(path)
    })

    it('opens the selected branch history from project overview', async () => {
        const secondBranch: ProjectBranch = {
            ...branch,
            id: '66666666-6666-4666-8666-666666666666',
            name: 'reinforced',
            headRevisionId: null,
        }

        vi.mocked(listProjectBranches).mockResolvedValue([branch, secondBranch])

        const { wrapper, router } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.findAll('button[aria-label^="View history of "]')).toHaveLength(2)
        })

        await wrapper.get('button[aria-label="View history of reinforced"]').trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('project-branch-history')
        })

        expect(router.currentRoute.value.params).toMatchObject({
            projectId,
            branchId: secondBranch.id,
        })

        wrapper.unmount()
    })

    it('does not invent history actions when no branches exist', async () => {
        vi.mocked(listProjectBranches).mockResolvedValue([])

        const { wrapper } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('No branches are available')
        })

        expect(wrapper.findAll('button[aria-label^="View history of "]')).toHaveLength(0)

        wrapper.unmount()
    })

    it('does not display branch actions for mismatched project records', async () => {
        vi.mocked(listProjectBranches).mockResolvedValue([
            { ...branch, projectId: 'different-project' },
        ])

        const { wrapper } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('Unable to load branches')
        })

        expect(wrapper.findAll('button[aria-label^="View history of "]')).toHaveLength(0)

        wrapper.unmount()
    })
})

describe('read-only branch history', () => {
    it('renders real revisions, ancestry, ordering, and observed head', async () => {
        const { wrapper } = await mountHistory()

        await vi.waitFor(() => {
            expect(wrapper.findAll('.branch-history__item')).toHaveLength(2)
        })

        expect(getProject).toHaveBeenCalledWith(projectId, expect.any(AbortSignal))
        expect(listProjectBranches).toHaveBeenCalledWith(projectId, expect.any(AbortSignal))
        expect(listProjectBranchHistory).toHaveBeenCalledWith(
            projectId,
            branchId,
            expect.any(AbortSignal),
        )

        const items = wrapper.findAll('.branch-history__item')
        expect(items[0]?.text()).toContain(head.message)
        expect(items[0]?.text()).toContain('Head at load')
        expect(items[1]?.text()).toContain(root.message)
        expect(items[1]?.text()).toContain('None (root revision)')

        const headDisplay = wrapper.get('.branch-history__context .revision-id__short')
        expect(headDisplay.text()).toBe(headId.slice(0, 8))
        expect(headDisplay.attributes('title')).toBe(headId)

        const firstRevisionIds = items[0]!.findAll('.revision-id__short')
        expect(firstRevisionIds.map((node) => node.text())).toEqual([
            headId.slice(0, 8),
            rootId.slice(0, 8),
        ])
        expect(firstRevisionIds.map((node) => node.attributes('title'))).toEqual([headId, rootId])

        expect(items[0]?.text()).not.toContain(rootId)
        expect(wrapper.text()).not.toContain(headId)
        expect(items[1]?.findAll('.revision-id')).toHaveLength(1)

        const inspectLink = items[0]!.get('a.branch-history__inspect')
        expect(inspectLink.attributes('href')).toContain(
            `/projects/${projectId}/revisions/${headId}`,
        )
        expect(inspectLink.attributes('href')).toContain(`fromBranchId=${branchId}`)

        expect(wrapper.text()).toContain('Head when loaded')
        expect(wrapper.text()).toContain(project.name)
        expect(wrapper.text()).toContain(branch.name)
        expect(wrapper.find('time').attributes('datetime')).toBe(head.createdAt)

        wrapper.unmount()
    })

    it('represents an empty branch without inventing revisions', async () => {
        vi.mocked(listProjectBranches).mockResolvedValue([{ ...branch, headRevisionId: null }])
        vi.mocked(listProjectBranchHistory).mockResolvedValue([])

        const { wrapper } = await mountHistory()

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('This branch has no revisions yet')
        })

        expect(wrapper.findAll('.branch-history__item')).toHaveLength(0)
        expect(wrapper.find('.revision-id').exists()).toBe(false)

        wrapper.unmount()
    })

    it('shows distinct primary and merge parents', async () => {
        const mergeId = '77777777-7777-4777-8777-777777777777'
        const merged: ProjectRevision = {
            ...head,
            id: mergeId,
            message: 'Integrate reinforced design',
            parentRevisionId: headId,
            mergeParentRevisionId: rootId,
            createdAt: '2026-10-19T13:00:00Z',
        }

        vi.mocked(listProjectBranches).mockResolvedValue([{ ...branch, headRevisionId: mergeId }])
        vi.mocked(listProjectBranchHistory).mockResolvedValue([merged, head, root])

        const { wrapper } = await mountHistory()

        await vi.waitFor(() => {
            expect(wrapper.findAll('.branch-history__item')).toHaveLength(3)
        })

        const first = wrapper.findAll('.branch-history__item')[0]
        expect(first?.text()).toContain('Integrate reinforced design')
        expect(first?.text()).toContain('Merge parent')

        const mergeRevisionIds = first!.findAll('.revision-id__short')
        expect(mergeRevisionIds.map((node) => node.text())).toEqual([
            mergeId.slice(0, 8),
            headId.slice(0, 8),
            rootId.slice(0, 8),
        ])
        expect(mergeRevisionIds.map((node) => node.attributes('title'))).toEqual([
            mergeId,
            headId,
            rootId,
        ])

        expect(first?.text()).not.toContain(headId)
        expect(first?.text()).not.toContain(rootId)
        expect(
            first!
                .get('button[aria-label="Copy full merge parent revision ID 33333333"]')
                .attributes('type'),
        ).toBe('button')

        expect(wrapper.text()).toContain('not necessarily a single development path')

        wrapper.unmount()
    })

    it('rejects an unexpected project identity', async () => {
        vi.mocked(getProject).mockResolvedValue({
            ...project,
            id: 'different-project',
        })

        const { wrapper } = await mountHistory()

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain('Unable to load')
        })

        expect(listProjectBranchHistory).not.toHaveBeenCalled()
        wrapper.unmount()
    })

    it('does not request history for an unavailable branch', async () => {
        vi.mocked(listProjectBranches).mockResolvedValue([])

        const { wrapper } = await mountHistory()

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain('not found')
        })

        expect(listProjectBranchHistory).not.toHaveBeenCalled()
        wrapper.unmount()
    })

    it('does not expose a project that returns a 404', async () => {
        vi.mocked(getProject).mockRejectedValue(new ProjectRequestError(404, 'Not Found'))

        const { wrapper } = await mountHistory()

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain('not accessible')
        })

        expect(listProjectBranchHistory).not.toHaveBeenCalled()
        wrapper.unmount()
    })

    it('redirects to login when history reports an expired session', async () => {
        vi.mocked(listProjectBranchHistory).mockRejectedValue(
            new RevisionRequestError(401, 'Unauthorized'),
        )

        const { wrapper, router } = await mountHistory()

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('login')
        })

        expect(router.currentRoute.value.query.redirect).toBe(
            `/projects/${projectId}/branches/${branchId}/history`,
        )

        wrapper.unmount()
    })

    it('allows retry after a temporary history API failure', async () => {
        vi.mocked(listProjectBranchHistory)
            .mockRejectedValueOnce(new RevisionRequestError(503, 'Unavailable'))
            .mockResolvedValueOnce([head, root])

        const { wrapper } = await mountHistory()

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain('Unable to load')
        })

        await wrapper.get('button').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.findAll('.branch-history__item')).toHaveLength(2)
        })

        expect(listProjectBranchHistory).toHaveBeenCalledTimes(2)
        wrapper.unmount()
    })

    it('reports an obvious branch-head and history mismatch', async () => {
        vi.mocked(listProjectBranchHistory).mockResolvedValue([root])

        const { wrapper } = await mountHistory()

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain('changed while loading')
        })

        expect(wrapper.findAll('.branch-history__item')).toHaveLength(0)
        wrapper.unmount()
    })

    it('ignores an obsolete project response after parameters change', async () => {
        let resolveFirst!: (value: Project) => void

        vi.mocked(getProject).mockImplementationOnce(
            () =>
                new Promise<Project>((resolve) => {
                    resolveFirst = resolve
                }),
        )

        const { wrapper } = await mountHistory()

        await vi.waitFor(() => {
            expect(getProject).toHaveBeenCalledTimes(1)
        })

        const otherProjectId = '88888888-8888-4888-8888-888888888888'
        const otherBranchId = '99999999-9999-4999-8999-999999999999'
        const otherHeadId = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'

        const otherProject: Project = {
            ...project,
            id: otherProjectId,
            name: 'Gearbox',
        }

        const otherBranch: ProjectBranch = {
            ...branch,
            id: otherBranchId,
            projectId: otherProjectId,
            headRevisionId: otherHeadId,
        }

        const otherRevision: ProjectRevision = {
            ...root,
            id: otherHeadId,
            projectId: otherProjectId,
            message: 'Initial gearbox',
        }

        vi.mocked(getProject).mockResolvedValue(otherProject)
        vi.mocked(listProjectBranches).mockResolvedValue([otherBranch])
        vi.mocked(listProjectBranchHistory).mockResolvedValue([otherRevision])

        await wrapper.setProps({
            projectId: otherProjectId,
            branchId: otherBranchId,
        })

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('Initial gearbox')
        })

        resolveFirst(project)
        await flushPromises()

        expect(wrapper.text()).toContain('Gearbox')
        expect(wrapper.text()).not.toContain('Initial gripper design')

        wrapper.unmount()
    })

    it('aborts a pending request when the view unmounts', async () => {
        let observedSignal: AbortSignal | undefined

        vi.mocked(getProject).mockImplementation((_id, signal) => {
            observedSignal = signal
            return new Promise<Project>(() => {})
        })

        const { wrapper } = await mountHistory()

        await vi.waitFor(() => {
            expect(observedSignal).toBeDefined()
        })

        wrapper.unmount()
        expect(observedSignal?.aborted).toBe(true)
    })
})
