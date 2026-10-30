import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import { listProjectFiles, type ProjectFile } from '../api/projectFiles'
import { getProject, listProjectBranches, type Project, type ProjectBranch } from '../api/projects'
import { listProjectRevisionFiles } from '../api/revisions'
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

vi.mock('../api/projectFiles', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/projectFiles')>()

    return {
        ...original,
        listProjectFiles: vi.fn<typeof original.listProjectFiles>(),
    }
})

vi.mock('../api/revisions', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/revisions')>()

    return {
        ...original,
        listProjectRevisionFiles: vi.fn<typeof original.listProjectRevisionFiles>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const mainBranchId = '22222222-2222-4222-8222-222222222222'
const designBranchId = '33333333-3333-4333-8333-333333333333'
const headRevisionId = '44444444-4444-4444-8444-444444444444'

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
    headRevisionId,
}

const revisionFile: ProjectFile = {
    id: '55555555-5555-4555-8555-555555555555',
    projectId,
    uploadedByUserId: '66666666-6666-4666-8666-666666666666',
    contentSha256: 'a'.repeat(64),
    originalFilename: 'gripper-body.step',
    mediaType: 'model/step',
    createdAt: '2026-10-11T12:00:00Z',
}

const user = {
    id: revisionFile.uploadedByUserId,
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:00:00Z',
}

let pinia: Pinia

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
        },
    })

    await vi.waitFor(() => {
        expect(listProjectBranches).toHaveBeenCalledWith(projectId, expect.any(AbortSignal))
    })

    await flushPromises()

    return wrapper
}

describe('project overview unified revision directory', () => {
    beforeEach(() => {
        vi.resetAllMocks()

        pinia = createPinia()
        setActivePinia(pinia)

        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user,
        })

        vi.mocked(getProject).mockResolvedValue(project)
        vi.mocked(listProjectBranches).mockResolvedValue([designBranch, mainBranch])
        vi.mocked(listProjectRevisionFiles).mockResolvedValue([revisionFile])
        vi.mocked(listProjectFiles).mockResolvedValue([])
    })

    it('shows one revision file directory without a duplicate head-revision list', async () => {
        const wrapper = await mountOverview()

        try {
            await vi.waitFor(() => {
                expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(1)
            })

            await flushPromises()

            expect(
                wrapper.get('button[aria-label="Current revision"]').attributes('aria-pressed'),
            ).toBe('true')
            expect(wrapper.get('ul[aria-label="Project files"]').text()).toContain(
                revisionFile.originalFilename,
            )

            expect(wrapper.find('ul[aria-label="Files in head revision"]').exists()).toBe(false)
        } finally {
            wrapper.unmount()
        }
    })

    it('requests the selected head revision exactly once', async () => {
        const wrapper = await mountOverview()

        try {
            await vi.waitFor(() => {
                expect(wrapper.get('ul[aria-label="Project files"]').text()).toContain(
                    revisionFile.originalFilename,
                )
            })

            await flushPromises()

            expect(listProjectRevisionFiles).toHaveBeenCalledWith(
                projectId,
                headRevisionId,
                expect.any(AbortSignal),
            )

            expect(listProjectRevisionFiles).toHaveBeenCalledTimes(1)
            expect(listProjectFiles).not.toHaveBeenCalled()
        } finally {
            wrapper.unmount()
        }
    })
})
