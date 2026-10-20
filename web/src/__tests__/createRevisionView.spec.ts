import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import { listProjectFiles, ProjectFileRequestError, type ProjectFile } from '../api/projectFiles'
import { getProject, listProjectBranches, type Project, type ProjectBranch } from '../api/projects'
import { createProjectRevision, listProjectRevisionFiles } from '../api/revisions'
import { createAppRouter } from '../router'
import CreateRevisionView from '../views/CreateRevisionView.vue'

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
        createProjectRevision: vi.fn<typeof original.createProjectRevision>(),
        listProjectRevisionFiles: vi.fn<typeof original.listProjectRevisionFiles>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const branchId = '22222222-2222-4222-8222-222222222222'
const revisionId = '33333333-3333-4333-8333-333333333333'

const project: Project = {
    id: projectId,
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private',
    createdAt: '2026-10-19T12:00:00Z',
    updatedAt: '2026-10-19T12:00:00Z',
}

const emptyBranch: ProjectBranch = {
    id: branchId,
    projectId,
    name: 'main',
    headRevisionId: null,
    createdAt: '2026-10-19T12:00:00Z',
    updatedAt: '2026-10-19T12:00:00Z',
}

const branchWithHead: ProjectBranch = {
    ...emptyBranch,
    name: 'gripper-redesign',
    headRevisionId: revisionId,
}

const firstFile: ProjectFile = {
    id: '44444444-4444-4444-8444-444444444444',
    projectId,
    uploadedByUserId: '55555555-5555-4555-8555-555555555555',
    contentSha256: 'a'.repeat(64),
    originalFilename: 'base.step',
    mediaType: null,
    createdAt: '2026-10-19T11:00:00Z',
}

const secondFile: ProjectFile = {
    ...firstFile,
    id: '66666666-6666-4666-8666-666666666666',
    contentSha256: 'b'.repeat(64),
    originalFilename: 'gripper.step',
    createdAt: '2026-10-19T11:30:00Z',
}

const thirdFile: ProjectFile = {
    ...firstFile,
    id: '77777777-7777-4777-8777-777777777777',
    contentSha256: 'c'.repeat(64),
    originalFilename: 'new-cover.step',
    createdAt: '2026-10-19T12:00:00Z',
}

const user = {
    id: firstFile.uploadedByUserId,
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-10-19T10:00:00Z',
    updatedAt: '2026-10-19T10:00:00Z',
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
    vi.mocked(listProjectBranches).mockResolvedValue([emptyBranch])
    vi.mocked(listProjectFiles).mockResolvedValue([firstFile, secondFile])
    vi.mocked(listProjectRevisionFiles).mockResolvedValue([secondFile])
})

async function renderView(branch: ProjectBranch = emptyBranch) {
    vi.mocked(listProjectBranches).mockResolvedValue([branch])

    const router = createAppRouter(createMemoryHistory())

    await router.push(`/projects/${projectId}/branches/${branch.id}/revisions/new`)
    await router.isReady()

    const wrapper = mount(CreateRevisionView, {
        props: {
            projectId,
            branchId: branch.id,
        },
        global: {
            plugins: [pinia, router],
        },
    })

    await flushPromises()

    return { wrapper, router }
}

function fileCheckbox(wrapper: ReturnType<typeof mount>, fileId: string) {
    return wrapper.get(`input[type="checkbox"][value="${fileId}"]`)
}

describe('Create Revision draft initialization', () => {
    it('loads project and branch context without preselecting uploaded files on an empty branch', async () => {
        const { wrapper } = await renderView()

        await vi.waitFor(() => {
            expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(2)
        })

        expect(wrapper.text()).toContain('Robotic Gripper')
        expect(wrapper.text()).toContain('main')
        expect(wrapper.text()).toContain('base.step')
        expect(wrapper.text()).toContain('gripper.step')

        expect((fileCheckbox(wrapper, firstFile.id).element as HTMLInputElement).checked).toBe(
            false,
        )

        expect((fileCheckbox(wrapper, secondFile.id).element as HTMLInputElement).checked).toBe(
            false,
        )

        expect(listProjectRevisionFiles).not.toHaveBeenCalled()
        expect(createProjectRevision).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('preselects exactly the current head snapshot and not newly uploaded files', async () => {
        vi.mocked(listProjectFiles).mockResolvedValue([firstFile, secondFile, thirdFile])

        const { wrapper } = await renderView(branchWithHead)

        await vi.waitFor(() => {
            expect(listProjectRevisionFiles).toHaveBeenCalledWith(
                projectId,
                revisionId,
                expect.any(AbortSignal),
            )
            expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(3)
        })

        expect(wrapper.text()).toContain('gripper-redesign')

        expect((fileCheckbox(wrapper, firstFile.id).element as HTMLInputElement).checked).toBe(
            false,
        )

        expect((fileCheckbox(wrapper, secondFile.id).element as HTMLInputElement).checked).toBe(
            true,
        )

        expect((fileCheckbox(wrapper, thirdFile.id).element as HTMLInputElement).checked).toBe(
            false,
        )

        expect(createProjectRevision).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('allows explicit inclusion and exclusion without creating a revision', async () => {
        const { wrapper, router } = await renderView(branchWithHead)

        await vi.waitFor(() => {
            expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(2)
            expect((fileCheckbox(wrapper, secondFile.id).element as HTMLInputElement).checked).toBe(
                true,
            )
        })

        await fileCheckbox(wrapper, firstFile.id).setValue(true)
        await fileCheckbox(wrapper, secondFile.id).setValue(false)

        expect((fileCheckbox(wrapper, firstFile.id).element as HTMLInputElement).checked).toBe(true)

        expect((fileCheckbox(wrapper, secondFile.id).element as HTMLInputElement).checked).toBe(
            false,
        )

        expect(router.currentRoute.value.name).toBe('create-project-revision')
        expect(createProjectRevision).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('shows the required message field and disabled creation action', async () => {
        const { wrapper } = await renderView()

        await vi.waitFor(() => {
            expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(2)
        })

        expect(wrapper.get('label[for="revision-message"]').text()).toMatch(/revision message/i)

        expect(wrapper.get('#revision-message').element.tagName).toBe('TEXTAREA')
        expect(wrapper.get('#revision-message').attributes('required')).toBeDefined()

        const button = wrapper.get('button[aria-label="Create revision unavailable"]')

        expect(button.attributes('disabled')).toBeDefined()
        expect(createProjectRevision).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('validates whitespace-only messages and clears the error on input', async () => {
        const { wrapper } = await renderView()

        await vi.waitFor(() => {
            expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(2)
        })

        const field = wrapper.get('#revision-message')

        await field.setValue('   ')

        expect(wrapper.get('#revision-message-error').text()).toContain(
            'Revision message is required',
        )

        expect(field.attributes('aria-invalid')).toBe('true')

        await field.setValue('Update mounting bracket geometry')

        expect(wrapper.find('#revision-message-error').exists()).toBe(false)
        expect(createProjectRevision).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('preserves file selection while editing the revision message', async () => {
        const { wrapper } = await renderView(branchWithHead)

        await vi.waitFor(() => {
            expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(2)
            expect((fileCheckbox(wrapper, secondFile.id).element as HTMLInputElement).checked).toBe(
                true,
            )
        })

        await fileCheckbox(wrapper, firstFile.id).setValue(true)
        await fileCheckbox(wrapper, secondFile.id).setValue(false)

        await wrapper.get('#revision-message').setValue('Revise mounting bracket')

        expect((fileCheckbox(wrapper, firstFile.id).element as HTMLInputElement).checked).toBe(true)

        expect((fileCheckbox(wrapper, secondFile.id).element as HTMLInputElement).checked).toBe(
            false,
        )

        expect(createProjectRevision).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('keeps the draft hidden while project context is loading', async () => {
        vi.mocked(getProject).mockReturnValueOnce(new Promise<Project>(() => {}))

        const { wrapper } = await renderView()

        expect(wrapper.get('[role="status"]').text()).toContain('Loading revision draft')
        expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(0)
        expect(wrapper.find('#revision-message').exists()).toBe(false)
        expect(createProjectRevision).not.toHaveBeenCalled()

        const signal = vi.mocked(getProject).mock.calls[0]?.[1]
        wrapper.unmount()
        expect(signal?.aborted).toBe(true)
    })

    it('rejects a target branch that is no longer present', async () => {
        vi.mocked(listProjectBranches).mockResolvedValueOnce([])

        const { wrapper } = await renderView()

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toMatch(/branch was not found/i)
        })

        expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(0)
        expect(wrapper.find('#revision-message').exists()).toBe(false)
        expect(createProjectRevision).not.toHaveBeenCalled()
        wrapper.unmount()
    })

    it('retries failed project-file loading without exposing a partial draft', async () => {
        vi.mocked(listProjectFiles).mockRejectedValueOnce(new Error('Network unavailable'))

        const { wrapper } = await renderView()

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toMatch(/unable to prepare/i)
        })

        expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(0)
        await wrapper.get('button').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(2)
        })

        expect(listProjectFiles).toHaveBeenCalledTimes(2)
        expect(createProjectRevision).not.toHaveBeenCalled()
        wrapper.unmount()
    })

    it('redirects with the original draft URL on confirmed session expiration', async () => {
        vi.mocked(listProjectFiles).mockRejectedValueOnce(
            new ProjectFileRequestError(401, 'Unauthorized'),
        )

        const { wrapper, router } = await renderView()

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('login')
        })

        expect(router.currentRoute.value.query.redirect).toBe(
            `/projects/${projectId}/branches/${branchId}/revisions/new`,
        )
        expect(createProjectRevision).not.toHaveBeenCalled()
        wrapper.unmount()
    })

    it('rejects a head snapshot referencing a file absent from the project directory', async () => {
        vi.mocked(listProjectRevisionFiles).mockResolvedValueOnce([thirdFile])

        const { wrapper } = await renderView(branchWithHead)

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toMatch(/unable to prepare/i)
        })

        expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(0)
        expect(createProjectRevision).not.toHaveBeenCalled()
        wrapper.unmount()
    })

    it('rejects a head snapshot with a cross-project file record even if its ID is present', async () => {
        vi.mocked(listProjectRevisionFiles).mockResolvedValueOnce([
            { ...secondFile, projectId: 'different-project' },
        ])

        const { wrapper } = await renderView(branchWithHead)

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toMatch(/unable to prepare/i)
        })

        expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(0)
        expect(createProjectRevision).not.toHaveBeenCalled()
        wrapper.unmount()
    })

    it('aborts obsolete branch requests and ignores late results after changing target branch', async () => {
        let resolveOldBranches!: (value: ProjectBranch[]) => void
        const pending = new Promise<ProjectBranch[]>((resolve) => {
            resolveOldBranches = resolve
        })
        vi.mocked(listProjectBranches).mockReturnValueOnce(pending)

        const { wrapper } = await renderView()
        const oldSignal = vi.mocked(listProjectBranches).mock.calls[0]?.[1]
        expect(oldSignal).toBeInstanceOf(AbortSignal)

        const alternateBranch: ProjectBranch = {
            ...emptyBranch,
            id: '88888888-8888-4888-8888-888888888888',
            name: 'alternate-design',
        }
        vi.mocked(listProjectBranches).mockResolvedValue([alternateBranch])

        await wrapper.setProps({ branchId: alternateBranch.id })

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('alternate-design')
            expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(2)
        })
        expect(oldSignal?.aborted).toBe(true)

        resolveOldBranches([emptyBranch])
        await flushPromises()

        expect(wrapper.text()).toContain('alternate-design')
        expect(wrapper.get('.create-revision__context').text()).not.toContain('main')
        expect(createProjectRevision).not.toHaveBeenCalled()
        wrapper.unmount()
    })

    it('aborts pending head-snapshot loading when the draft is unmounted', async () => {
        let resolveSnapshot!: (value: ProjectFile[]) => void
        vi.mocked(listProjectRevisionFiles).mockReturnValueOnce(
            new Promise<ProjectFile[]>((resolve) => {
                resolveSnapshot = resolve
            }),
        )

        const { wrapper } = await renderView(branchWithHead)

        await vi.waitFor(() => {
            expect(listProjectRevisionFiles).toHaveBeenCalledTimes(1)
        })
        const signal = vi.mocked(listProjectRevisionFiles).mock.calls[0]?.[2]
        expect(signal).toBeInstanceOf(AbortSignal)

        wrapper.unmount()
        expect(signal?.aborted).toBe(true)

        resolveSnapshot([secondFile])
        await flushPromises()
        expect(createProjectRevision).not.toHaveBeenCalled()
    })
})
