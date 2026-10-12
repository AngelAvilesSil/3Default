import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import { createProject, ProjectRequestError } from '../api/projects'
import { createAppRouter } from '../router'
import CreateProjectView from '../views/CreateProjectView.vue'

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
        createProject: vi.fn<typeof original.createProject>(),
    }
})

const user = {
    id: '11111111-1111-4111-8111-111111111111',
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:01:00Z',
}

const createdProject = {
    id: '22222222-2222-4222-8222-222222222222',
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private' as const,
    createdAt: '2026-10-12T12:00:00Z',
    updatedAt: '2026-10-12T12:00:00Z',
}

describe('Create Project view', () => {
    let pinia: Pinia

    beforeEach(() => {
        vi.resetAllMocks()
        pinia = createPinia()
        setActivePinia(pinia)
    })

    async function renderView() {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user,
        })

        const router = createAppRouter(createMemoryHistory())
        await router.push('/projects/new')
        await router.isReady()

        const wrapper = mount(CreateProjectView, {
            global: { plugins: [pinia, router] },
        })

        return { wrapper, router }
    }

    it('displays the project details form', async () => {
        const { wrapper } = await renderView()

        expect(wrapper.get('#create-project-heading').text()).toBe('Create a project')
        expect(wrapper.text()).toContain('Project details')
        expect(wrapper.get('#project-name').attributes('required')).toBeDefined()
        expect(wrapper.get('#project-description').element.tagName).toBe('TEXTAREA')
        expect(wrapper.get('button[type="submit"]').text()).toContain('Create project')

        wrapper.unmount()
    })

    it('uses inline validation for an empty project name', async () => {
        const { wrapper } = await renderView()

        expect(wrapper.get('form').attributes('novalidate')).toBeDefined()

        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(createProject).not.toHaveBeenCalled()
        expect(wrapper.get('#project-name-error').text()).toContain('Project name is required')

        wrapper.unmount()
    })

    it('rejects a whitespace-only project name', async () => {
        const { wrapper } = await renderView()

        await wrapper.get('#project-name').setValue('   ')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(createProject).not.toHaveBeenCalled()
        expect(wrapper.get('#project-name-error').text()).toContain('Project name is required')

        wrapper.unmount()
    })

    it('creates a project with normalized values and returns to My Projects', async () => {
        vi.mocked(createProject).mockResolvedValueOnce(createdProject)

        const { wrapper, router } = await renderView()

        await wrapper.get('#project-name').setValue('  Robotic Gripper  ')
        await wrapper.get('#project-description').setValue('  Mechanical gripper development  ')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(createProject).toHaveBeenCalledTimes(1)
        expect(vi.mocked(createProject).mock.calls[0]?.[0]).toEqual({
            name: 'Robotic Gripper',
            description: 'Mechanical gripper development',
        })
        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('my-projects')
            expect(router.currentRoute.value.path).toBe('/projects')
        })

        wrapper.unmount()
    })

    it('omits the description when it is empty', async () => {
        vi.mocked(createProject).mockResolvedValueOnce({
            ...createdProject,
            description: null,
        })

        const { wrapper } = await renderView()

        await wrapper.get('#project-name').setValue('Robotic Gripper')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(vi.mocked(createProject).mock.calls[0]?.[0]).toEqual({
            name: 'Robotic Gripper',
        })

        wrapper.unmount()
    })

    it('shows a duplicate-name error without clearing the form', async () => {
        vi.mocked(createProject).mockRejectedValueOnce(new ProjectRequestError(409, 'Conflict'))

        const { wrapper, router } = await renderView()

        await wrapper.get('#project-name').setValue('Robotic Gripper')
        await wrapper.get('#project-description').setValue('Mechanical gripper development')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(wrapper.get('#project-name-error').text()).toMatch(/already exists/i)
        expect((wrapper.get('#project-name').element as HTMLInputElement).value).toBe(
            'Robotic Gripper',
        )
        expect((wrapper.get('#project-description').element as HTMLTextAreaElement).value).toBe(
            'Mechanical gripper development',
        )
        expect(router.currentRoute.value.name).toBe('create-project')

        wrapper.unmount()
    })

    it('clears the duplicate-name error and allows another submission', async () => {
        vi.mocked(createProject)
            .mockRejectedValueOnce(new ProjectRequestError(409, 'Conflict'))
            .mockResolvedValueOnce({
                ...createdProject,
                name: 'Updated Gripper',
            })

        const { wrapper, router } = await renderView()

        await wrapper.get('#project-name').setValue('Robotic Gripper')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(wrapper.get('#project-name-error').text()).toMatch(/already exists/i)

        await wrapper.get('#project-name').setValue('Updated Gripper')

        expect(wrapper.find('#project-name-error').exists()).toBe(false)

        await wrapper.get('form').trigger('submit')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('my-projects')
        })

        expect(createProject).toHaveBeenCalledTimes(2)
        expect(vi.mocked(createProject).mock.calls[1]?.[0]).toEqual({
            name: 'Updated Gripper',
        })

        wrapper.unmount()
    })

    it('returns to My Projects when Cancel is clicked', async () => {
        const { wrapper, router } = await renderView()

        await wrapper.get('button[type="button"]').trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('my-projects')
        })

        expect(createProject).not.toHaveBeenCalled()
        wrapper.unmount()
    })

    it('redirects to login when project creation returns HTTP 401', async () => {
        vi.mocked(createProject).mockRejectedValueOnce(new ProjectRequestError(401, 'Unauthorized'))

        const { wrapper, router } = await renderView()

        await wrapper.get('#project-name').setValue('Robotic Gripper')
        await wrapper.get('form').trigger('submit')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('login')
            expect(router.currentRoute.value.query.redirect).toBe('/projects/new')
        })

        wrapper.unmount()
    })

    it('shows a validation message when the backend returns HTTP 400', async () => {
        vi.mocked(createProject).mockRejectedValueOnce(new ProjectRequestError(400, 'Bad Request'))

        const { wrapper } = await renderView()

        await wrapper.get('#project-name').setValue('Robotic Gripper')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(wrapper.get('[role="alert"]').text()).toMatch(/project details were not accepted/i)
        expect((wrapper.get('#project-name').element as HTMLInputElement).value).toBe(
            'Robotic Gripper',
        )

        wrapper.unmount()
    })

    it.each([
        ['server failure', new ProjectRequestError(500, 'Internal Server Error')],
        ['network failure', new TypeError('Failed to fetch')],
    ])('shows a retryable message for creation failures: %s', async (_reason, error) => {
        vi.mocked(createProject).mockRejectedValueOnce(error)

        const { wrapper, router } = await renderView()

        await wrapper.get('#project-name').setValue('Robotic Gripper')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(wrapper.get('[role="alert"]').text()).toMatch(/unable to create project/i)
        expect(router.currentRoute.value.name).toBe('create-project')
        expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeUndefined()

        wrapper.unmount()
    })

    it('prevents duplicate submissions while creation is pending', async () => {
        let resolveCreation!: (value: typeof createdProject) => void

        const pending = new Promise<typeof createdProject>((resolve) => {
            resolveCreation = resolve
        })

        vi.mocked(createProject).mockReturnValueOnce(pending)

        const { wrapper, router } = await renderView()

        await wrapper.get('#project-name').setValue('Robotic Gripper')
        await wrapper.get('form').trigger('submit')
        await wrapper.get('form').trigger('submit')

        expect(createProject).toHaveBeenCalledTimes(1)
        expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
        expect(wrapper.get('button[type="button"]').attributes('disabled')).toBeDefined()
        expect(wrapper.get('#project-name').attributes('disabled')).toBeDefined()
        expect(wrapper.get('#project-description').attributes('disabled')).toBeDefined()

        resolveCreation(createdProject)

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('my-projects')
        })

        wrapper.unmount()
    })

    it('aborts an in-flight request when the view is unmounted', async () => {
        let resolveCreation!: (value: typeof createdProject) => void

        const pending = new Promise<typeof createdProject>((resolve) => {
            resolveCreation = resolve
        })

        vi.mocked(createProject).mockReturnValueOnce(pending)

        const { wrapper, router } = await renderView()
        const pushSpy = vi.spyOn(router, 'push')

        await wrapper.get('#project-name').setValue('Robotic Gripper')
        await wrapper.get('form').trigger('submit')

        expect(createProject).toHaveBeenCalledTimes(1)

        const signal = vi.mocked(createProject).mock.calls[0]?.[1]

        expect(signal).toBeInstanceOf(AbortSignal)
        expect(signal?.aborted).toBe(false)

        wrapper.unmount()

        expect(signal?.aborted).toBe(true)

        resolveCreation(createdProject)
        await flushPromises()

        // A late response must not navigate after the view is destroyed.
        expect(pushSpy).not.toHaveBeenCalledWith({
            name: 'my-projects',
        })
    })
})
