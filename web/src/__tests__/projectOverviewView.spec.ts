import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import { getProject, ProjectRequestError, type Project } from '../api/projects'
import { createAppRouter } from '../router'
import { useSessionStore } from '../stores/session'
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
    }
})

const project: Project = {
    id: '11111111-1111-4111-8111-111111111111',
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private',
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:01:00Z',
}

const secondProject: Project = {
    ...project,
    id: '22222222-2222-4222-8222-222222222222',
    name: 'Gearbox',
    description: null,
}

const user = {
    id: '33333333-3333-4333-8333-333333333333',
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-09-11T12:00:00Z',
    updatedAt: '2026-09-11T12:01:00Z',
}

let pinia: Pinia

async function mountOverview(projectId = project.id) {
    const router = createAppRouter(createMemoryHistory())

    await router.push(`/projects/${projectId}`)

    const wrapper = mount(ProjectOverviewView, {
        global: {
            plugins: [pinia, router],
        },
    })

    return { wrapper, router }
}

describe('project overview view', () => {
    beforeEach(() => {
        vi.resetAllMocks()
        pinia = createPinia()
        setActivePinia(pinia)

        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user,
        })
    })

    it('loads and displays real project metadata', async () => {
        vi.mocked(getProject).mockResolvedValueOnce(project)

        const { wrapper } = await mountOverview()
        await flushPromises()

        expect(getProject).toHaveBeenCalledWith(project.id, expect.any(AbortSignal))
        expect(wrapper.get('h2').text()).toBe('Robotic Gripper')
        expect(wrapper.text()).toContain('Mechanical gripper development')
        expect(wrapper.get('a[href="/projects"]').text()).toContain('My Projects')
        expect(wrapper.find('[role="status"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('shows loading while a project request is pending', async () => {
        let resolveRequest!: (value: Project) => void

        vi.mocked(getProject).mockReturnValueOnce(
            new Promise<Project>((resolve) => {
                resolveRequest = resolve
            }),
        )

        const { wrapper } = await mountOverview()

        expect(wrapper.get('[role="status"]').text()).toContain('Loading project')

        resolveRequest(project)
        await flushPromises()

        expect(wrapper.get('h2').text()).toBe(project.name)
        expect(wrapper.find('[role="status"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('accepts a project without a description', async () => {
        vi.mocked(getProject).mockResolvedValueOnce(secondProject)

        const { wrapper } = await mountOverview(secondProject.id)
        await flushPromises()

        expect(wrapper.get('h2').text()).toBe('Gearbox')
        expect(wrapper.find('.project-overview__header p').exists()).toBe(false)

        wrapper.unmount()
    })

    it('shows a safe message for a missing or inaccessible project', async () => {
        vi.mocked(getProject).mockRejectedValueOnce(new ProjectRequestError(404, 'Not Found'))

        const { wrapper } = await mountOverview()
        await flushPromises()

        expect(wrapper.get('[role="alert"]').text()).toContain('not found or is not accessible')
        expect(wrapper.find('button').exists()).toBe(false)

        wrapper.unmount()
    })

    it('shows a helpful message for an invalid project ID', async () => {
        vi.mocked(getProject).mockRejectedValueOnce(new ProjectRequestError(400, 'Bad Request'))

        const { wrapper } = await mountOverview('invalid-project-id')
        await flushPromises()

        expect(wrapper.get('[role="alert"]').text()).toContain('project link is invalid')

        wrapper.unmount()
    })

    it('redirects an expired session to login with its return path', async () => {
        vi.mocked(getProject).mockRejectedValueOnce(new ProjectRequestError(401, 'Unauthorized'))

        const { wrapper, router } = await mountOverview()
        const session = useSessionStore(pinia)

        await vi.waitFor(() => {
            expect(session.status).toBe('unauthenticated')
            expect(router.currentRoute.value.name).toBe('login')
        })

        expect(router.currentRoute.value.query.redirect).toBe(`/projects/${project.id}`)

        wrapper.unmount()
    })

    it('offers retry after a recoverable request failure', async () => {
        vi.mocked(getProject)
            .mockRejectedValueOnce(new TypeError('Failed to fetch'))
            .mockResolvedValueOnce(project)

        const { wrapper } = await mountOverview()
        await flushPromises()

        expect(wrapper.get('[role="alert"]').text()).toContain('Unable to load the project')

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(getProject).toHaveBeenCalledTimes(2)
        expect(wrapper.get('h2').text()).toBe(project.name)
        expect(wrapper.find('[role="alert"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('ignores an obsolete response when navigating to another project', async () => {
        let resolveFirst!: (value: Project) => void

        const firstRequest = new Promise<Project>((resolve) => {
            resolveFirst = resolve
        })

        vi.mocked(getProject).mockReturnValueOnce(firstRequest).mockResolvedValueOnce(secondProject)

        const { wrapper, router } = await mountOverview()

        await router.push(`/projects/${secondProject.id}`)
        await flushPromises()

        expect(wrapper.get('h2').text()).toBe('Gearbox')

        const firstSignal = vi.mocked(getProject).mock.calls[0]?.[1]
        expect(firstSignal?.aborted).toBe(true)

        resolveFirst(project)
        await flushPromises()

        expect(wrapper.get('h2').text()).toBe('Gearbox')
        expect(wrapper.text()).not.toContain('Robotic Gripper')

        wrapper.unmount()
    })

    it('aborts a pending project request when the view unmounts', async () => {
        vi.mocked(getProject).mockReturnValueOnce(new Promise<Project>(() => {}))

        const { wrapper } = await mountOverview()

        const signal = vi.mocked(getProject).mock.calls[0]?.[1]

        expect(signal?.aborted).toBe(false)

        wrapper.unmount()

        expect(signal?.aborted).toBe(true)
    })
})
