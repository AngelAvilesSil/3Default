import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import type { ProjectFile } from '../api/projectFiles'
import { getProject, listProjectBranches, type Project, type ProjectBranch } from '../api/projects'
import { listProjectRevisionFiles, RevisionRequestError } from '../api/revisions'
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

vi.mock('../api/revisions', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/revisions')>()

    return {
        ...original,
        listProjectRevisionFiles: vi.fn<typeof original.listProjectRevisionFiles>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const mainBranchId = '33333333-3333-4333-8333-333333333333'
const designBranchId = '44444444-4444-4444-8444-444444444444'
const headRevisionId = '55555555-5555-4555-8555-555555555555'

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

const firstFile: ProjectFile = {
    id: '77777777-7777-4777-8777-777777777777',
    projectId,
    uploadedByUserId: '66666666-6666-4666-8666-666666666666',
    contentSha256: 'a'.repeat(64),
    originalFilename: 'gripper-body.step',
    mediaType: null,
    createdAt: '2026-10-11T12:00:00Z',
}

const secondFile: ProjectFile = {
    ...firstFile,
    id: '88888888-8888-4888-8888-888888888888',
    contentSha256: 'b'.repeat(64),
    originalFilename: 'mounting-plate.step',
}

const user = {
    id: firstFile.uploadedByUserId,
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:00:00Z',
}

let pinia: Pinia

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
                ProjectFileWorkspace: true,
            },
        },
    })

    await vi.waitFor(() => {
        expect(listProjectBranches).toHaveBeenCalledWith(projectId, expect.any(AbortSignal))
    })

    await flushPromises()

    return { wrapper, router }
}

describe('project overview branch-head revision files', () => {
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

        vi.mocked(listProjectRevisionFiles).mockResolvedValue([firstFile, secondFile])
    })

    it('does not request revision files for a branch without a head', async () => {
        const { wrapper } = await mountOverview()

        expect(wrapper.get('button[aria-label="Choose design branch"]').text()).toContain('main')

        expect(wrapper.text()).toContain('No revisions yet')
        expect(listProjectRevisionFiles).not.toHaveBeenCalled()

        expect(wrapper.find('ul[aria-label="Files in head revision"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('does not load a snapshot for an unavailable explicit branch', async () => {
        const missingBranchId = '99999999-9999-4999-8999-999999999999'

        const { wrapper, router } = await mountOverview(missingBranchId)

        expect(wrapper.text()).toContain('The branch in this link is unavailable')
        expect(router.currentRoute.value.query.branchId).toBe(missingBranchId)
        expect(listProjectRevisionFiles).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('loads only the exact files in the selected head revision', async () => {
        const { wrapper } = await mountOverview(designBranchId)

        await vi.waitFor(() => {
            expect(listProjectRevisionFiles).toHaveBeenCalledWith(
                projectId,
                headRevisionId,
                expect.any(AbortSignal),
            )
        })

        const files = wrapper.get('ul[aria-label="Files in head revision"]')
        const entries = files.findAll('li')

        expect(entries).toHaveLength(2)
        expect(entries[0]!.text()).toContain(firstFile.originalFilename)
        expect(entries[1]!.text()).toContain(secondFile.originalFilename)

        expect(wrapper.text()).toContain('gripper-redesign')
        expect(wrapper.text()).toContain('Head revision')

        // The project-wide uploaded-file workspace remains independent.
        expect(wrapper.find('project-file-workspace-stub').exists()).toBe(true)

        wrapper.unmount()
    })

    it('distinguishes an empty revision snapshot from no head revision', async () => {
        vi.mocked(listProjectRevisionFiles).mockResolvedValue([])

        const { wrapper } = await mountOverview(designBranchId)

        await vi.waitFor(() => {
            expect(listProjectRevisionFiles).toHaveBeenCalledWith(
                projectId,
                headRevisionId,
                expect.any(AbortSignal),
            )
        })

        expect(wrapper.text()).toContain('This revision contains no source files.')
        expect(wrapper.text()).toContain('Head revision')
        expect(wrapper.text()).not.toContain('No revisions yet')

        expect(wrapper.findAll('ul[aria-label="Files in head revision"] > li')).toHaveLength(0)

        wrapper.unmount()
    })

    it('shows loading and aborts a pending snapshot request on unmount', async () => {
        let observedSignal: AbortSignal | undefined

        vi.mocked(listProjectRevisionFiles).mockImplementation(
            (_projectId, _revisionId, signal) => {
                observedSignal = signal
                return new Promise<ProjectFile[]>(() => {})
            },
        )

        const { wrapper } = await mountOverview(designBranchId)

        await vi.waitFor(() => {
            expect(observedSignal).toBeDefined()
        })

        expect(wrapper.get('.branch-head-files [role="status"]').text()).toContain(
            'Loading head revision files',
        )

        expect(wrapper.find('ul[aria-label="Files in head revision"]').exists()).toBe(false)

        expect(observedSignal?.aborted).toBe(false)

        wrapper.unmount()

        expect(observedSignal?.aborted).toBe(true)
    })

    it('retries a recoverable snapshot failure without reloading project metadata', async () => {
        vi.mocked(listProjectRevisionFiles)
            .mockRejectedValueOnce(new RevisionRequestError(503, 'Unavailable'))
            .mockResolvedValueOnce([firstFile])

        const { wrapper } = await mountOverview(designBranchId)

        await vi.waitFor(() => {
            expect(wrapper.get('.branch-head-files [role="alert"]').text()).toContain(
                'Unable to load head revision files',
            )
        })

        expect(wrapper.text()).not.toContain(firstFile.originalFilename)

        await wrapper.get('.branch-head-files__feedback button').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.get('ul[aria-label="Files in head revision"]').text()).toContain(
                firstFile.originalFilename,
            )
        })

        expect(listProjectRevisionFiles).toHaveBeenCalledTimes(2)
        expect(getProject).toHaveBeenCalledTimes(1)
        expect(listProjectBranches).toHaveBeenCalledTimes(1)

        wrapper.unmount()
    })

    it('shows an unavailable state for a missing head-revision snapshot', async () => {
        vi.mocked(listProjectRevisionFiles).mockRejectedValue(
            new RevisionRequestError(404, 'Not Found'),
        )

        const { wrapper } = await mountOverview(designBranchId)

        await vi.waitFor(() => {
            expect(wrapper.get('.branch-head-files [role="alert"]').text()).toContain(
                'not accessible',
            )
        })

        expect(wrapper.find('ul[aria-label="Files in head revision"]').exists()).toBe(false)

        expect(wrapper.find('.branch-head-files__feedback button').exists()).toBe(false)

        wrapper.unmount()
    })

    it('rejects snapshot records belonging to another project', async () => {
        vi.mocked(listProjectRevisionFiles).mockResolvedValue([
            {
                ...firstFile,
                projectId: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
            },
        ])

        const { wrapper } = await mountOverview(designBranchId)

        await vi.waitFor(() => {
            expect(wrapper.get('.branch-head-files [role="alert"]').text()).toContain(
                'Unable to load head revision files',
            )
        })

        expect(wrapper.text()).not.toContain(firstFile.originalFilename)
        expect(wrapper.find('ul[aria-label="Files in head revision"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('redirects an expired snapshot session while preserving branch context', async () => {
        vi.mocked(listProjectRevisionFiles).mockRejectedValue(
            new RevisionRequestError(401, 'Unauthorized'),
        )

        const { wrapper, router } = await mountOverview(designBranchId)

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('login')
        })

        expect(router.currentRoute.value.query.redirect).toBe(
            `/projects/${projectId}?branchId=${designBranchId}`,
        )

        expect(listProjectRevisionFiles).toHaveBeenCalledWith(
            projectId,
            headRevisionId,
            expect.any(AbortSignal),
        )

        wrapper.unmount()
    })

    it('aborts and ignores an obsolete head snapshot after switching branches', async () => {
        const alternateBranch: ProjectBranch = {
            ...designBranch,
            id: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
            name: 'mounting-update',
            headRevisionId: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
        }

        vi.mocked(listProjectBranches).mockResolvedValue([
            designBranch,
            alternateBranch,
            mainBranch,
        ])

        let resolveOld!: (files: ProjectFile[]) => void

        vi.mocked(listProjectRevisionFiles)
            .mockImplementationOnce(
                () =>
                    new Promise<ProjectFile[]>((resolve) => {
                        resolveOld = resolve
                    }),
            )
            .mockResolvedValueOnce([secondFile])

        const { wrapper, router } = await mountOverview(designBranchId)

        await vi.waitFor(() => {
            expect(listProjectRevisionFiles).toHaveBeenCalledTimes(1)
        })

        const oldSignal = vi.mocked(listProjectRevisionFiles).mock.calls[0]?.[2]

        expect(oldSignal).toBeDefined()
        expect(oldSignal?.aborted).toBe(false)

        await wrapper.get('button[aria-label="Choose design branch"]').trigger('click')

        const choice = wrapper
            .findAll('#project-branch-choices button')
            .find((button) => button.text().includes(alternateBranch.name))

        if (!choice) {
            throw new Error('Expected alternate branch choice was not found')
        }

        await choice.trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.query.branchId).toBe(alternateBranch.id)

            expect(listProjectRevisionFiles).toHaveBeenCalledWith(
                projectId,
                alternateBranch.headRevisionId,
                expect.any(AbortSignal),
            )
        })

        await vi.waitFor(() => {
            expect(wrapper.get('ul[aria-label="Files in head revision"]').text()).toContain(
                secondFile.originalFilename,
            )
        })

        expect(oldSignal?.aborted).toBe(true)

        // Simulate an obsolete response resolving despite cancellation.
        resolveOld([firstFile])
        await flushPromises()

        expect(wrapper.text()).toContain(secondFile.originalFilename)
        expect(wrapper.text()).not.toContain(firstFile.originalFilename)
        expect(wrapper.text()).toContain(alternateBranch.name)

        wrapper.unmount()
    })
})
