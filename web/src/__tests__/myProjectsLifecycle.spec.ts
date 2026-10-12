import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
    listProjects,
    ProjectRequestError,
    ProjectResponseError,
    type Project,
} from '../api/projects'
import { createAppRouter } from '../router'
import { useSessionStore } from '../stores/session'
import MyProjectsView from '../views/MyProjectsView.vue'

vi.mock('../api/projects', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/projects')>()

    return {
        ...original,
        listProjects: vi.fn<typeof original.listProjects>(),
    }
})

const user = {
    id: '11111111-1111-4111-8111-111111111111',
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-09-11T12:00:00Z',
    updatedAt: '2026-09-11T12:01:00Z',
}

const project: Project = {
    id: '22222222-2222-4222-8222-222222222222',
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private',
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:01:00Z',
}

async function mountAuthenticatedView() {
    const pinia = createPinia()
    setActivePinia(pinia)

    const session = useSessionStore()
    session.status = 'authenticated'
    session.user = user

    const router = createAppRouter(createMemoryHistory())
    await router.push('/projects')

    const wrapper = mount(MyProjectsView, {
        global: {
            plugins: [pinia, router],
        },
    })

    return { wrapper, router, session }
}

describe('My Projects request failures and lifecycle', () => {
    beforeEach(() => {
        vi.resetAllMocks()
    })

    it('navigates to Create Project from the listing header', async () => {
        vi.mocked(listProjects).mockResolvedValueOnce([project])

        const { wrapper, router } = await mountAuthenticatedView()
        await flushPromises()

        await wrapper.get('header a[href="/projects/new"]').trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('create-project')
            expect(router.currentRoute.value.path).toBe('/projects/new')
        })

        wrapper.unmount()
    })

    it('preserves the session after an HTTP 500 failure', async () => {
        vi.mocked(listProjects).mockRejectedValueOnce(
            new ProjectRequestError(500, 'Internal Server Error'),
        )

        const { wrapper, session } = await mountAuthenticatedView()
        await flushPromises()

        expect(wrapper.get('[role="alert"]').text()).toContain('Unable to load projects')
        expect(wrapper.text()).not.toContain('No projects yet')
        expect(session.status).toBe('authenticated')
        expect(session.user).toEqual(user)

        wrapper.unmount()
    })

    it('preserves the session after a network failure', async () => {
        vi.mocked(listProjects).mockRejectedValueOnce(new TypeError('Failed to fetch'))

        const { wrapper, session } = await mountAuthenticatedView()
        await flushPromises()

        expect(wrapper.get('[role="alert"]').text()).toContain('Unable to load projects')
        expect(session.status).toBe('authenticated')
        expect(session.user).toEqual(user)

        wrapper.unmount()
    })

    it('retries a failed request and displays the projects', async () => {
        vi.mocked(listProjects)
            .mockRejectedValueOnce(new ProjectRequestError(500, 'Internal Server Error'))
            .mockResolvedValueOnce([project])

        const { wrapper } = await mountAuthenticatedView()
        await flushPromises()

        expect(wrapper.get('[role="alert"]').text()).toContain('Unable to load projects')

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(listProjects).toHaveBeenCalledTimes(2)
        expect(wrapper.text()).toContain('Robotic Gripper')
        expect(wrapper.find('[role="alert"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('allows recovery from a malformed project response', async () => {
        vi.mocked(listProjects)
            .mockRejectedValueOnce(
                new ProjectResponseError('Project response did not contain a valid project list'),
            )
            .mockResolvedValueOnce([project])

        const { wrapper, session } = await mountAuthenticatedView()
        await flushPromises()

        expect(wrapper.get('[role="alert"]').text()).toContain('Unable to load projects')
        expect(session.status).toBe('authenticated')

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(listProjects).toHaveBeenCalledTimes(2)
        expect(wrapper.text()).toContain('Robotic Gripper')
        expect(session.status).toBe('authenticated')

        wrapper.unmount()
    })

    it('redirects to login if retry discovers an expired session', async () => {
        vi.mocked(listProjects)
            .mockRejectedValueOnce(new ProjectRequestError(500, 'Internal Server Error'))
            .mockRejectedValueOnce(new ProjectRequestError(401, 'Unauthorized'))

        const { wrapper, router, session } = await mountAuthenticatedView()
        await flushPromises()

        await wrapper.get('button').trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('login')
        })

        expect(listProjects).toHaveBeenCalledTimes(2)
        expect(session.status).toBe('unauthenticated')
        expect(session.user).toBeNull()
        expect(router.currentRoute.value.query.redirect).toBe('/projects')

        wrapper.unmount()
    })

    it('invalidates an expired session and redirects to login', async () => {
        vi.mocked(listProjects).mockRejectedValueOnce(new ProjectRequestError(401, 'Unauthorized'))

        const { wrapper, router, session } = await mountAuthenticatedView()
        await flushPromises()

        expect(session.status).toBe('unauthenticated')
        expect(session.user).toBeNull()
        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('login')
        })
        expect(router.currentRoute.value.query.redirect).toBe('/projects')

        wrapper.unmount()
    })

    it('aborts a pending project request when unmounted', async () => {
        let resolveRequest!: (projects: Project[]) => void

        const pendingRequest = new Promise<Project[]>((resolve) => {
            resolveRequest = resolve
        })

        vi.mocked(listProjects).mockReturnValueOnce(pendingRequest)

        const { wrapper } = await mountAuthenticatedView()

        const signal = vi.mocked(listProjects).mock.calls[0]?.[0]

        expect(signal).toBeInstanceOf(AbortSignal)
        expect(signal?.aborted).toBe(false)

        wrapper.unmount()

        expect(signal?.aborted).toBe(true)

        resolveRequest([project])
        await flushPromises()
    })

    it('ignores a late HTTP 401 after the view is unmounted', async () => {
        let rejectRequest!: (reason?: unknown) => void

        const pendingRequest = new Promise<Project[]>((_resolve, reject) => {
            rejectRequest = reject
        })

        vi.mocked(listProjects).mockReturnValueOnce(pendingRequest)

        const { wrapper, router, session } = await mountAuthenticatedView()
        const replaceSpy = vi.spyOn(router, 'replace')

        expect(router.currentRoute.value.name).toBe('my-projects')

        wrapper.unmount()

        rejectRequest(new ProjectRequestError(401, 'Unauthorized'))
        await flushPromises()

        expect(session.status).toBe('authenticated')
        expect(session.user).toEqual(user)
        expect(replaceSpy).not.toHaveBeenCalled()
    })
})
