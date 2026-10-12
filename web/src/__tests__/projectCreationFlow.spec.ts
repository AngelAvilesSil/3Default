import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, RouterView } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createProject, listProjects, type Project } from '../api/projects'
import { createAppRouter } from '../router'
import { useSessionStore } from '../stores/session'

vi.mock('../api/projects', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/projects')>()

    return {
        ...original,
        createProject: vi.fn<typeof original.createProject>(),
        listProjects: vi.fn<typeof original.listProjects>(),
    }
})

const user = {
    id: '11111111-1111-4111-8111-111111111111',
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:01:00Z',
}

const createdProject: Project = {
    id: '22222222-2222-4222-8222-222222222222',
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private',
    createdAt: '2026-10-12T12:00:00Z',
    updatedAt: '2026-10-12T12:00:00Z',
}

describe('project creation navigation flow', () => {
    beforeEach(() => {
        vi.resetAllMocks()
    })

    it('refreshes My Projects after creating a project', async () => {
        vi.mocked(listProjects).mockResolvedValueOnce([]).mockResolvedValueOnce([createdProject])

        vi.mocked(createProject).mockResolvedValueOnce(createdProject)

        const pinia = createPinia()
        setActivePinia(pinia)

        const session = useSessionStore()
        session.status = 'authenticated'
        session.user = user

        const router = createAppRouter(createMemoryHistory())

        await router.push('/projects')
        await router.isReady()

        const wrapper = mount(RouterView, {
            global: {
                plugins: [pinia, router],
            },
        })

        // The initial listing is empty.
        await vi.waitFor(() => {
            expect(wrapper.get('.my-projects__feedback').text()).toContain('No projects yet.')
        })

        expect(listProjects).toHaveBeenCalledTimes(1)

        // Navigate using the actual empty-state link.
        await wrapper.get('.my-projects__empty-link[href="/projects/new"]').trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('create-project')
            expect(wrapper.find('#project-name').exists()).toBe(true)
        })

        // Submit the actual creation form.
        await wrapper.get('#project-name').setValue('Robotic Gripper')
        await wrapper.get('#project-description').setValue('Mechanical gripper development')

        await wrapper.get('form').trigger('submit')

        // Returning to My Projects must load a fresh listing.
        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('my-projects')
            expect(wrapper.get('ul[aria-label="Your projects"]').text()).toContain(
                'Robotic Gripper',
            )
        })

        expect(createProject).toHaveBeenCalledTimes(1)
        expect(vi.mocked(createProject).mock.calls[0]?.[0]).toEqual({
            name: 'Robotic Gripper',
            description: 'Mechanical gripper development',
        })

        expect(listProjects).toHaveBeenCalledTimes(2)

        const projectList = wrapper.get('ul[aria-label="Your projects"]')

        expect(projectList.text()).toContain('Mechanical gripper development')
        expect(wrapper.find('.my-projects__feedback').exists()).toBe(false)

        wrapper.unmount()
    })
})
