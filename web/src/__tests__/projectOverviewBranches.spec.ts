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

const mainBranch: ProjectBranch = {
    id: '33333333-3333-4333-8333-333333333333',
    projectId: project.id,
    name: 'main',
    headRevisionId: null,
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:00:00Z',
}

const featureBranch: ProjectBranch = {
    ...mainBranch,
    id: '44444444-4444-4444-8444-444444444444',
    name: 'gripper-redesign',
    headRevisionId: '55555555-5555-4555-8555-555555555555',
}

const user = {
    id: '66666666-6666-4666-8666-666666666666',
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-09-11T12:00:00Z',
    updatedAt: '2026-09-11T12:01:00Z',
}

async function mountOverview(projectId = project.id) {
    const router = createAppRouter(createMemoryHistory())

    await router.push(`/projects/${projectId}`)

    const wrapper = mount(ProjectOverviewView, {
        global: {
            plugins: [pinia, router],
            stubs: {
                ProjectFileWorkspace: true,
            },
        },
    })

    return { wrapper, router }
}

let pinia: Pinia

describe('project overview branch summaries', () => {
    beforeEach(() => {
        vi.resetAllMocks()
        pinia = createPinia()
        setActivePinia(pinia)

        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user,
        })

        vi.mocked(getProject).mockImplementation(async (projectId) =>
            projectId === secondProject.id ? secondProject : project,
        )
    })

    it('displays a branch with no head revision', async () => {
        vi.mocked(listProjectBranches).mockResolvedValueOnce([mainBranch])

        const { wrapper } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('No revisions yet')
        })

        expect(listProjectBranches).toHaveBeenCalledWith(project.id, expect.any(AbortSignal))

        const branches = wrapper.get('ul[aria-label="Project branches"]')

        expect(branches.text()).toContain('main')
        expect(branches.text()).toContain('No revisions yet')

        wrapper.unmount()
    })

    it('shows branch loading without hiding project metadata', async () => {
        vi.mocked(listProjectBranches).mockReturnValueOnce(new Promise<ProjectBranch[]>(() => {}))

        const { wrapper } = await mountOverview()

        await vi.waitFor(() => {
            expect(listProjectBranches).toHaveBeenCalledTimes(1)
        })
        await flushPromises()

        expect(wrapper.get('h2').text()).toBe(project.name)
        expect(wrapper.text()).toContain(project.description)
        expect(wrapper.text()).toContain('Loading branches')

        wrapper.unmount()
    })

    it('shows multiple branches and existing head revision information', async () => {
        vi.mocked(listProjectBranches).mockResolvedValueOnce([mainBranch, featureBranch])

        const { wrapper } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.findAll('ul[aria-label="Project branches"] > li')).toHaveLength(2)
        })

        const branches = wrapper.findAll('ul[aria-label="Project branches"] > li')

        expect(branches[0]!.text()).toContain('main')
        expect(branches[0]!.text()).toContain('No revisions yet')

        expect(branches[1]!.text()).toContain('gripper-redesign')
        expect(branches[1]!.text()).toContain('Head revision')
        expect(branches[1]!.text()).toContain('55555555')

        wrapper.unmount()
    })

    it('handles an empty branch list without inventing a default branch', async () => {
        vi.mocked(listProjectBranches).mockResolvedValueOnce([])

        const { wrapper } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('No branches')
        })

        expect(wrapper.find('ul[aria-label="Project branches"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('retries a failed branch request without reloading project metadata', async () => {
        vi.mocked(listProjectBranches)
            .mockRejectedValueOnce(new TypeError('Failed to fetch'))
            .mockResolvedValueOnce([mainBranch])

        const { wrapper } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('Unable to load branches')
        })

        expect(wrapper.get('h2').text()).toBe(project.name)

        await wrapper.get('button').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('No revisions yet')
        })

        expect(getProject).toHaveBeenCalledTimes(1)
        expect(listProjectBranches).toHaveBeenCalledTimes(2)

        wrapper.unmount()
    })

    it('does not display branches associated with another project', async () => {
        vi.mocked(listProjectBranches).mockResolvedValueOnce([
            {
                ...mainBranch,
                projectId: secondProject.id,
                name: 'foreign-branch',
            },
        ])

        const { wrapper } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('Unable to load branches')
        })

        expect(wrapper.get('h2').text()).toBe(project.name)
        expect(wrapper.text()).not.toContain('foreign-branch')

        wrapper.unmount()
    })

    it('redirects to login when branch access returns HTTP 401', async () => {
        vi.mocked(listProjectBranches).mockRejectedValueOnce(
            new ProjectRequestError(401, 'Unauthorized'),
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

    it('ignores obsolete branch results after switching projects', async () => {
        let resolveFirst!: (branches: ProjectBranch[]) => void

        const firstRequest = new Promise<ProjectBranch[]>((resolve) => {
            resolveFirst = resolve
        })

        const secondBranch: ProjectBranch = {
            ...mainBranch,
            id: '77777777-7777-4777-8777-777777777777',
            projectId: secondProject.id,
            name: 'gearbox-development',
        }

        vi.mocked(listProjectBranches).mockImplementation((projectId) =>
            projectId === project.id ? firstRequest : Promise.resolve([secondBranch]),
        )

        const { wrapper, router } = await mountOverview()

        await vi.waitFor(() => {
            expect(listProjectBranches).toHaveBeenCalledWith(project.id, expect.any(AbortSignal))
        })

        const oldSignal = vi.mocked(listProjectBranches).mock.calls[0]?.[1]

        await router.push(`/projects/${secondProject.id}`)

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('gearbox-development')
        })

        expect(oldSignal?.aborted).toBe(true)
        expect(wrapper.get('h2').text()).toBe('Gearbox')

        resolveFirst([mainBranch])
        await flushPromises()

        expect(wrapper.get('h2').text()).toBe('Gearbox')

        const displayedBranches = wrapper.findAll('ul[aria-label="Project branches"] > li')

        expect(displayedBranches).toHaveLength(1)
        expect(displayedBranches[0]!.text()).toContain('gearbox-development')
        expect(displayedBranches[0]!.text()).not.toContain('main')

        wrapper.unmount()
    })
})
