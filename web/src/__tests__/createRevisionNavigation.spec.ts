import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import { getProject, listProjectBranches, type Project, type ProjectBranch } from '../api/projects'
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

const projectId = '11111111-1111-4111-8111-111111111111'
const mainBranchId = '22222222-2222-4222-8222-222222222222'
const featureBranchId = '33333333-3333-4333-8333-333333333333'

const project: Project = {
    id: projectId,
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private',
    createdAt: '2026-10-19T12:00:00Z',
    updatedAt: '2026-10-19T12:00:00Z',
}

const mainBranch: ProjectBranch = {
    id: mainBranchId,
    projectId,
    name: 'main',
    headRevisionId: null,
    createdAt: '2026-10-19T12:00:00Z',
    updatedAt: '2026-10-19T12:00:00Z',
}

const featureBranch: ProjectBranch = {
    ...mainBranch,
    id: featureBranchId,
    name: 'gripper-redesign',
    headRevisionId: '44444444-4444-4444-8444-444444444444',
}

const user = {
    id: '55555555-5555-4555-8555-555555555555',
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-10-19T12:00:00Z',
    updatedAt: '2026-10-19T12:00:00Z',
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
    vi.mocked(listProjectBranches).mockResolvedValue([mainBranch])
})

async function mountOverview() {
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

describe('Create Revision navigation', () => {
    it('registers a protected lazy-loaded route with both identifiers', () => {
        const router = createAppRouter(createMemoryHistory())

        const route = router
            .getRoutes()
            .find((candidate) => candidate.name === 'create-project-revision')

        expect(route).toBeDefined()
        expect(route?.path).toBe('/projects/:projectId/branches/:branchId/revisions/new')
        expect(route?.meta.requiresAuth).toBe(true)
        expect(route?.props.default).toBe(true)
        expect(typeof route?.components?.default).toBe('function')
    })

    it('resolves the selected project and branch into the revision URL', () => {
        const router = createAppRouter(createMemoryHistory())

        const destination = router.resolve({
            name: 'create-project-revision',
            params: {
                projectId,
                branchId: featureBranchId,
            },
        })

        expect(destination.path).toBe(
            `/projects/${projectId}/branches/${featureBranchId}/revisions/new`,
        )
    })

    it('requires login for a direct Create Revision URL', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })

        const router = createAppRouter(createMemoryHistory())
        const path = `/projects/${projectId}/branches/${mainBranchId}/revisions/new`

        await router.push(path)

        expect(router.currentRoute.value.name).toBe('login')
        expect(router.currentRoute.value.query.redirect).toBe(path)
    })

    it('offers an explicit revision action for a branch without a head', async () => {
        const { wrapper } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('No revisions yet')
        })

        const action = wrapper.get('button[aria-label="Create revision on main"]')

        expect(action.text()).toContain('Create revision')
        expect(action.attributes('disabled')).toBeUndefined()

        wrapper.unmount()
    })

    it('targets the specific branch selected from the project overview', async () => {
        vi.mocked(listProjectBranches).mockResolvedValue([mainBranch, featureBranch])

        const { wrapper, router } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.findAll('button[aria-label^="Create revision on "]')).toHaveLength(2)
        })

        await wrapper
            .get('button[aria-label="Create revision on gripper-redesign"]')
            .trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('create-project-revision')
        })

        expect(router.currentRoute.value.params).toMatchObject({
            projectId,
            branchId: featureBranchId,
        })

        wrapper.unmount()
    })

    it('does not invent a revision action when no branches exist', async () => {
        vi.mocked(listProjectBranches).mockResolvedValue([])

        const { wrapper } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('No branches are available for this project.')
        })

        expect(wrapper.findAll('button[aria-label^="Create revision on "]')).toHaveLength(0)

        wrapper.unmount()
    })

    it('does not offer revision creation while branches are loading', async () => {
        vi.mocked(listProjectBranches).mockReturnValue(new Promise<ProjectBranch[]>(() => {}))

        const { wrapper } = await mountOverview()

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('Loading branches')
        })

        expect(wrapper.findAll('button[aria-label^="Create revision on "]')).toHaveLength(0)

        wrapper.unmount()
    })
})
