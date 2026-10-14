import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import { listProjects, type Project } from '../api/projects'
import { createAppRouter } from '../router'
import MyProjectsView from '../views/MyProjectsView.vue'

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
        listProjects: vi.fn<typeof original.listProjects>(),
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
}

const user = {
    id: '33333333-3333-4333-8333-333333333333',
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-09-11T12:00:00Z',
    updatedAt: '2026-09-11T12:01:00Z',
}

describe('project workspace navigation', () => {
    let pinia: Pinia

    beforeEach(() => {
        vi.resetAllMocks()
        pinia = createPinia()
        setActivePinia(pinia)
    })

    it('registers a protected project overview route', () => {
        const router = createAppRouter(createMemoryHistory())
        const destination = router.resolve(`/projects/${project.id}`)

        expect(destination.name).toBe('project-overview')
        expect(destination.params.projectId).toBe(project.id)
        expect(destination.meta.requiresAuth).toBe(true)
    })

    it('continues to resolve /projects/new as project creation', () => {
        const router = createAppRouter(createMemoryHistory())
        const destination = router.resolve('/projects/new')

        expect(destination.name).toBe('create-project')
        expect(destination.meta.requiresAuth).toBe(true)
    })

    it('redirects unauthenticated project visitors to login', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })

        const router = createAppRouter(createMemoryHistory())
        const projectPath = `/projects/${project.id}`

        await router.push(projectPath)

        expect(router.currentRoute.value.name).toBe('login')
        expect(router.currentRoute.value.query.redirect).toBe(projectPath)
    })

    it('allows authenticated users to open a direct project URL', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user,
        })

        const router = createAppRouter(createMemoryHistory())

        await router.push(`/projects/${project.id}`)

        expect(router.currentRoute.value.name).toBe('project-overview')
        expect(router.currentRoute.value.params.projectId).toBe(project.id)
        expect(getCurrentUser).toHaveBeenCalledTimes(1)
    })

    it('renders direct project links in My Projects', async () => {
        vi.mocked(listProjects).mockResolvedValueOnce([project, secondProject])

        const router = createAppRouter(createMemoryHistory())

        const wrapper = mount(MyProjectsView, {
            global: {
                plugins: [pinia, router],
            },
        })

        await flushPromises()

        const links = wrapper.findAll('ul[aria-label="Your projects"] h3 a')

        expect(links).toHaveLength(2)
        expect(links[0]!.text()).toBe('Robotic Gripper')
        expect(links[0]!.attributes('href')).toBe(`/projects/${project.id}`)
        expect(links[1]!.text()).toBe('Gearbox')
        expect(links[1]!.attributes('href')).toBe(`/projects/${secondProject.id}`)

        wrapper.unmount()
    })
})
