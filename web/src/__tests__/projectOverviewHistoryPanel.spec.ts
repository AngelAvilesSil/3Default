import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import { getProject, listProjectBranches, type Project, type ProjectBranch } from '../api/projects'
import { loadProjectHistoryGraph } from '../history/projectHistoryProvider'
import { buildRevisionGraph } from '../history/revisionGraph'
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

vi.mock('../history/projectHistoryProvider', async (importOriginal) => {
    const original = await importOriginal<typeof import('../history/projectHistoryProvider')>()
    return {
        ...original,
        loadProjectHistoryGraph: vi.fn<typeof original.loadProjectHistoryGraph>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const mainBranchId = '22222222-2222-4222-8222-222222222222'
const designBranchId = '33333333-3333-4333-8333-333333333333'
const designHeadId = '44444444-4444-4444-8444-444444444444'
const authorId = '55555555-5555-4555-8555-555555555555'

const project: Project = {
    id: projectId,
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private',
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:01:00Z',
}

const mainBranch: ProjectBranch = {
    id: mainBranchId,
    projectId,
    name: 'main',
    headRevisionId: null,
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:00:00Z',
}

const designBranch: ProjectBranch = {
    ...mainBranch,
    id: designBranchId,
    name: 'gripper-redesign',
    headRevisionId: designHeadId,
}

const WorkspaceStub = defineComponent({
    name: 'ProjectFileWorkspace',
    props: {
        projectId: { type: String, required: true },
        branchId: { type: String, default: null },
        headRevisionId: { type: String, default: null },
    },
    template: `
        <div
            data-testid="overview-workspace"
            :data-project-id="projectId"
            :data-branch-id="branchId ?? ''"
            :data-head-revision-id="headRevisionId ?? ''"
        ></div>
    `,
})

let pinia: Pinia

beforeEach(() => {
    vi.resetAllMocks()

    pinia = createPinia()
    setActivePinia(pinia)

    vi.mocked(getCurrentUser).mockResolvedValue({
        status: 'authenticated',
        user: {
            id: authorId,
            email: 'engineer@example.com',
            displayName: 'Engineer',
            createdAt: '2026-10-11T12:00:00Z',
            updatedAt: '2026-10-11T12:00:00Z',
        },
    })

    vi.mocked(getProject).mockResolvedValue(project)
    vi.mocked(listProjectBranches).mockResolvedValue([mainBranch, designBranch])

    vi.mocked(loadProjectHistoryGraph).mockResolvedValue(
        buildRevisionGraph({
            projectId,
            revisions: [
                {
                    id: designHeadId,
                    projectId,
                    authorUserId: authorId,
                    authorDisplayName: 'Engineer',
                    message: 'Alternative gripper design',
                    createdAt: '2026-10-11T12:30:00Z',
                    parentRevisionId: null,
                    mergeParentRevisionId: null,
                },
            ],
            branches: [mainBranch, designBranch],
            coverage: {
                source: 'branch-head-reachable',
                status: 'complete',
                failedBranchIds: [],
            },
        }),
    )
})

async function mountOverview() {
    const router = createAppRouter(createMemoryHistory())

    await router.push({
        name: 'project-overview',
        params: { projectId },
        query: { branchId: designBranchId },
    })
    await router.isReady()

    const wrapper = mount(ProjectOverviewView, {
        global: {
            plugins: [pinia, router],
            stubs: {
                ProjectFileWorkspace: WorkspaceStub,
            },
        },
    })

    await vi.waitFor(() => {
        expect(wrapper.get('[data-testid="overview-workspace"]').attributes('data-branch-id')).toBe(
            designBranchId,
        )
    })

    await flushPromises()

    return { wrapper, router }
}

describe('Project Overview history panel shell', () => {
    it('starts collapsed without eagerly loading history', async () => {
        const { wrapper } = await mountOverview()

        try {
            const toggle = wrapper.get('button[aria-controls="project-overview-history-panel"]')

            expect(toggle.attributes('aria-expanded')).toBe('false')

            const panel = wrapper.get('#project-overview-history-panel')
            expect(panel.isVisible()).toBe(false)

            expect(loadProjectHistoryGraph).not.toHaveBeenCalled()
        } finally {
            wrapper.unmount()
        }
    })

    it('opens and closes without changing the workspace branch', async () => {
        const { wrapper, router } = await mountOverview()

        try {
            const toggle = wrapper.get('button[aria-controls="project-overview-history-panel"]')

            const workspace = wrapper.get('[data-testid="overview-workspace"]')

            await toggle.trigger('click')

            expect(toggle.attributes('aria-expanded')).toBe('true')
            expect(wrapper.get('#project-overview-history-panel').isVisible()).toBe(true)

            expect(router.currentRoute.value.query.branchId).toBe(designBranchId)
            expect(workspace.attributes('data-branch-id')).toBe(designBranchId)
            expect(workspace.attributes('data-head-revision-id')).toBe(designHeadId)

            await toggle.trigger('click')

            expect(toggle.attributes('aria-expanded')).toBe('false')
            expect(wrapper.get('#project-overview-history-panel').isVisible()).toBe(false)

            expect(router.currentRoute.value.query.branchId).toBe(designBranchId)
            expect(workspace.attributes('data-branch-id')).toBe(designBranchId)
        } finally {
            wrapper.unmount()
        }
    })
})
