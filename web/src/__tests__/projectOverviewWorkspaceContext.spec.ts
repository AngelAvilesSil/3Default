import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
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
const designBranchId = '33333333-3333-4333-8333-333333333333'
const designHeadId = '44444444-4444-4444-8444-444444444444'

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

const user = {
    id: '55555555-5555-4555-8555-555555555555',
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:01:00Z',
}

let pinia: Pinia

const WorkspaceContextStub = defineComponent({
    name: 'ProjectFileWorkspace',
    props: {
        projectId: { type: String, required: true },
        branchId: { type: String, default: null },
        headRevisionId: { type: String, default: null },
    },
    emits: ['unauthorized', 'add-file'],
    template: `
        <div
            data-testid="overview-workspace"
            :data-project-id="projectId"
            :data-branch-id="branchId ?? ''"
            :data-head-revision-id="headRevisionId ?? ''"
        ></div>
    `,
})

async function mountOverview(branchId?: string) {
    const router = createAppRouter(createMemoryHistory())

    await router.push({
        name: 'project-overview',
        params: { projectId },
        query: branchId === undefined ? {} : { branchId },
    })
    await router.isReady()

    const wrapper = mount(ProjectOverviewView, {
        global: {
            plugins: [pinia, router],
            stubs: {
                ProjectFileWorkspace: WorkspaceContextStub,
                BranchHeadFiles: true,
            },
        },
    })

    await vi.waitFor(() => {
        expect(listProjectBranches).toHaveBeenCalledWith(projectId, expect.any(AbortSignal))
    })

    await flushPromises()

    return { wrapper, router }
}

describe('project overview workspace branch context', () => {
    beforeEach(() => {
        vi.resetAllMocks()

        pinia = createPinia()
        setActivePinia(pinia)

        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user,
        })

        vi.mocked(getProject).mockResolvedValue(project)

        // main must be selected by name, not by response ordering.
        vi.mocked(listProjectBranches).mockResolvedValue([designBranch, mainBranch])
    })

    it('passes the default headless main branch to the file workspace', async () => {
        const { wrapper } = await mountOverview()

        try {
            const workspace = wrapper.get('[data-testid="overview-workspace"]')

            expect(workspace.attributes('data-project-id')).toBe(projectId)
            expect(workspace.attributes('data-branch-id')).toBe(mainBranchId)
            expect(workspace.attributes('data-head-revision-id')).toBe('')
            expect(wrapper.text()).toContain('No revisions yet')
        } finally {
            wrapper.unmount()
        }
    })

    it('passes an explicit branch and its exact head revision to the workspace', async () => {
        const { wrapper } = await mountOverview(designBranchId)

        try {
            const workspace = wrapper.get('[data-testid="overview-workspace"]')

            expect(workspace.attributes('data-project-id')).toBe(projectId)
            expect(workspace.attributes('data-branch-id')).toBe(designBranchId)
            expect(workspace.attributes('data-head-revision-id')).toBe(designHeadId)
            expect(wrapper.text()).toContain('gripper-redesign')
        } finally {
            wrapper.unmount()
        }
    })

    it('keeps workspace context synchronized through branch URL navigation', async () => {
        const { wrapper, router } = await mountOverview(designBranchId)

        try {
            await vi.waitFor(() => {
                expect(
                    wrapper.get('[data-testid="overview-workspace"]').attributes('data-branch-id'),
                ).toBe(designBranchId)
            })

            await router.push({
                name: 'project-overview',
                params: { projectId },
            })

            await vi.waitFor(() => {
                const workspace = wrapper.get('[data-testid="overview-workspace"]')

                expect(workspace.attributes('data-branch-id')).toBe(mainBranchId)
                expect(workspace.attributes('data-head-revision-id')).toBe('')
            })

            await router.push({
                name: 'project-overview',
                params: { projectId },
                query: { branchId: designBranchId },
            })

            await vi.waitFor(() => {
                const workspace = wrapper.get('[data-testid="overview-workspace"]')

                expect(workspace.attributes('data-branch-id')).toBe(designBranchId)
                expect(workspace.attributes('data-head-revision-id')).toBe(designHeadId)
            })

            expect(getProject).toHaveBeenCalledTimes(1)
            expect(listProjectBranches).toHaveBeenCalledTimes(1)
        } finally {
            wrapper.unmount()
        }
    })

    it('does not mount a file workspace for an unavailable explicit branch', async () => {
        const missingBranchId = '99999999-9999-4999-8999-999999999999'
        const { wrapper, router } = await mountOverview(missingBranchId)

        try {
            expect(router.currentRoute.value.query.branchId).toBe(missingBranchId)
            expect(wrapper.text()).toContain('The branch in this link is unavailable')
            expect(wrapper.find('[data-testid="overview-workspace"]').exists()).toBe(false)
        } finally {
            wrapper.unmount()
        }
    })
})
