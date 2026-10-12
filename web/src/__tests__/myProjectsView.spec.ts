import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { listProjects, type Project } from '../api/projects'
import { createAppRouter } from '../router'
import MyProjectsView from '../views/MyProjectsView.vue'

vi.mock('../api/projects', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/projects')>()

    return {
        ...original,
        listProjects: vi.fn<typeof original.listProjects>(),
    }
})

const newerProject: Project = {
    id: '11111111-1111-4111-8111-111111111111',
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private',
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:01:00Z',
}

const olderProject: Project = {
    id: '22222222-2222-4222-8222-222222222222',
    name: 'Gearbox',
    description: null,
    visibility: 'private',
    createdAt: '2026-09-01T12:00:00Z',
    updatedAt: '2026-09-01T12:00:00Z',
}

function mountView() {
    const pinia = createPinia()
    setActivePinia(pinia)

    const router = createAppRouter(createMemoryHistory())

    return mount(MyProjectsView, {
        global: {
            plugins: [pinia, router],
        },
    })
}

describe('My Projects view', () => {
    beforeEach(() => {
        vi.resetAllMocks()
    })

    it('shows a loading state while retrieving projects', async () => {
        let resolveRequest!: (projects: Project[]) => void

        const pendingRequest = new Promise<Project[]>((resolve) => {
            resolveRequest = resolve
        })

        vi.mocked(listProjects).mockReturnValueOnce(pendingRequest)

        const wrapper = mountView()

        expect(wrapper.get('[role="status"]').text()).toContain('Loading projects')
        expect(listProjects).toHaveBeenCalledTimes(1)
        expect(listProjects).toHaveBeenCalledWith(expect.any(AbortSignal))

        resolveRequest([])
        await flushPromises()

        expect(wrapper.find('[role="status"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('renders projects in the order returned by the API', async () => {
        vi.mocked(listProjects).mockResolvedValueOnce([newerProject, olderProject])

        const wrapper = mountView()
        await flushPromises()

        const content = wrapper.text()

        expect(content).toContain('Robotic Gripper')
        expect(content).toContain('Mechanical gripper development')
        expect(content).toContain('Gearbox')

        expect(content.indexOf('Robotic Gripper')).toBeLessThan(content.indexOf('Gearbox'))

        expect(wrapper.find('[role="status"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('renders project metadata without creating premature workspace links', async () => {
        vi.mocked(listProjects).mockResolvedValueOnce([newerProject, olderProject])

        const wrapper = mountView()
        await flushPromises()

        const rows = wrapper.findAll('ul[aria-label="Your projects"] > li')

        expect(rows).toHaveLength(2)

        expect(rows[0]!.get('h3').text()).toBe('Robotic Gripper')
        expect(rows[0]!.get('.my-projects__description').text()).toBe(
            'Mechanical gripper development',
        )
        expect(rows[0]!.get('time').attributes('datetime')).toBe(newerProject.createdAt)
        expect(rows[0]!.get('time').text()).toMatch(/^Created .*2026$/)
        expect(rows[0]!.get('[aria-hidden="true"]').text()).toBe('3D')

        expect(rows[1]!.get('h3').text()).toBe('Gearbox')
        expect(rows[1]!.find('.my-projects__description').exists()).toBe(false)
        expect(rows[1]!.get('time').attributes('datetime')).toBe(olderProject.createdAt)
        expect(rows[1]!.get('time').text()).toMatch(/^Created .*2026$/)

        expect(wrapper.find('ul[aria-label="Your projects"] a').exists()).toBe(false)
        expect(wrapper.find('ul[aria-label="Your projects"] button').exists()).toBe(false)

        wrapper.unmount()
    })

    it('offers project creation when projects already exist', async () => {
        vi.mocked(listProjects).mockResolvedValueOnce([newerProject])

        const wrapper = mountView()
        await flushPromises()

        const createLink = wrapper.get('header a[href="/projects/new"]')

        expect(createLink.text()).toBe('Create project')
        expect(wrapper.get('ul[aria-label="Your projects"]').text()).toContain('Robotic Gripper')

        wrapper.unmount()
    })

    it('offers project creation in the empty state', async () => {
        vi.mocked(listProjects).mockResolvedValueOnce([])

        const wrapper = mountView()
        await flushPromises()

        expect(wrapper.get('header a[href="/projects/new"]').text()).toBe('Create project')

        const emptyStateLink = wrapper.get('.my-projects__feedback a[href="/projects/new"]')

        expect(emptyStateLink.text()).toBe('Create your first project')
        expect(wrapper.text()).toContain('No projects yet')
        expect(wrapper.text()).not.toContain('Project creation is coming next.')

        wrapper.unmount()
    })

    it('shows a helpful empty state when no projects exist', async () => {
        vi.mocked(listProjects).mockResolvedValueOnce([])

        const wrapper = mountView()
        await flushPromises()

        expect(wrapper.text()).toContain('No projects yet')
        expect(wrapper.find('[role="status"]').exists()).toBe(false)

        wrapper.unmount()
    })
})
