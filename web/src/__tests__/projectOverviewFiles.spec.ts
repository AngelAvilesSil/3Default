import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import { listProjectFiles, ProjectFileRequestError, type ProjectFile } from '../api/projectFiles'
import { getProject, listProjectBranches, ProjectRequestError, type Project } from '../api/projects'
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
        listProjectBranches: vi.fn<typeof original.listProjectBranches>(),
    }
})

vi.mock('../api/projectFiles', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/projectFiles')>()
    return {
        ...original,
        listProjectFiles: vi.fn<typeof original.listProjectFiles>(),
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

const projectFile: ProjectFile = {
    id: '22222222-2222-4222-8222-222222222222',
    projectId: project.id,
    uploadedByUserId: '33333333-3333-4333-8333-333333333333',
    contentSha256: 'a'.repeat(64),
    originalFilename: 'gripper.step',
    mediaType: 'model/step',
    createdAt: '2026-10-15T12:00:00Z',
}

const branch = {
    id: '44444444-4444-4444-8444-444444444444',
    projectId: project.id,
    name: 'main',
    headRevisionId: null,
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:00:00Z',
}

const user = {
    id: '55555555-5555-4555-8555-555555555555',
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-09-11T12:00:00Z',
    updatedAt: '2026-09-11T12:01:00Z',
}

let pinia: Pinia

async function mountOverview() {
    const router = createAppRouter(createMemoryHistory())
    await router.push(`/projects/${project.id}`)

    const wrapper = mount(ProjectOverviewView, {
        global: {
            plugins: [pinia, router],
        },
    })

    return { wrapper, router }
}

describe('project overview file integration', () => {
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
        vi.mocked(listProjectFiles).mockResolvedValue([projectFile])
    })

    it('renders uploaded files alongside project metadata and branches', async () => {
        const { wrapper } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(1)
        })

        expect(wrapper.get('h2').text()).toBe(project.name)
        expect(wrapper.text()).toContain('gripper.step')
        expect(wrapper.text()).toContain('Uploaded')
        expect(wrapper.text()).toContain('No revisions yet')
        expect(listProjectFiles).toHaveBeenCalledWith(project.id, expect.any(AbortSignal))

        wrapper.unmount()
    })

    it('places branch context before the project file list', async () => {
        const { wrapper } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.find('ul[aria-label="Project files"]').exists()).toBe(true)
        })

        const sections = wrapper.findAll('.project-overview__branches, .project-files-panel')

        expect(sections).toHaveLength(2)
        expect(sections[0]!.classes()).toContain('project-overview__branches')
        expect(sections[1]!.classes()).toContain('project-files-panel')

        wrapper.unmount()
    })

    it('retries file failures independently of project and branch data', async () => {
        vi.mocked(listProjectFiles)
            .mockRejectedValueOnce(new TypeError('Failed to fetch'))
            .mockResolvedValueOnce([projectFile])

        const { wrapper } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('Unable to load files')
        })

        expect(wrapper.get('h2').text()).toBe(project.name)
        expect(wrapper.text()).toContain('No revisions yet')

        await wrapper.get('.project-files-panel__feedback button').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('gripper.step')
        })

        expect(getProject).toHaveBeenCalledTimes(1)
        expect(listProjectBranches).toHaveBeenCalledTimes(1)
        expect(listProjectFiles).toHaveBeenCalledTimes(2)

        wrapper.unmount()
    })

    it('redirects an expired file session to login with the return path', async () => {
        vi.mocked(listProjectFiles).mockRejectedValueOnce(
            new ProjectFileRequestError(401, 'Unauthorized'),
        )

        const { wrapper, router } = await mountOverview()
        const session = useSessionStore(pinia)

        await vi.waitFor(() => {
            expect(session.status).toBe('unauthenticated')
            expect(router.currentRoute.value.name).toBe('login')
        })

        expect(router.currentRoute.value.query.redirect).toBe(`/projects/${project.id}`)

        wrapper.unmount()
    })

    it('does not request files when project access fails', async () => {
        vi.mocked(getProject).mockRejectedValueOnce(new ProjectRequestError(404, 'Not Found'))

        const { wrapper } = await mountOverview()
        await flushPromises()

        expect(wrapper.text()).toContain('not found or is not accessible')
        expect(listProjectFiles).not.toHaveBeenCalled()

        wrapper.unmount()
    })
})
