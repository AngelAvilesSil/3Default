import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ProjectFileRequestError, uploadProjectFile, type ProjectFile } from '../api/projectFiles'
import { createAppRouter } from '../router'
import { useSessionStore } from '../stores/session'
import ProjectFileUploadView from '../views/ProjectFileUploadView.vue'

vi.mock('../api/projectFiles', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/projectFiles')>()

    return {
        ...original,
        uploadProjectFile: vi.fn<typeof original.uploadProjectFile>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const otherProjectId = '22222222-2222-4222-8222-222222222222'

const user = {
    id: '33333333-3333-4333-8333-333333333333',
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:01:00Z',
}

beforeEach(() => {
    vi.resetAllMocks()
})

async function renderView() {
    const pinia = createPinia()
    setActivePinia(pinia)

    const session = useSessionStore(pinia)
    session.status = 'authenticated'
    session.user = user

    const router = createAppRouter(createMemoryHistory())
    const uploadPath = `/projects/${projectId}/files/upload`

    await router.push(uploadPath)

    const wrapper = mount(ProjectFileUploadView, {
        props: { projectId },
        global: { plugins: [pinia, router] },
    })

    return { wrapper, router, session, uploadPath }
}

function makeFile(name: string): File {
    return new File(['CAD source'], name, {
        type: 'application/octet-stream',
        lastModified: 1700000000000,
    })
}

function uploadedFile(file: File, id: string): ProjectFile {
    return {
        id,
        projectId,
        uploadedByUserId: user.id,
        contentSha256: 'a'.repeat(64),
        originalFilename: file.name,
        mediaType: file.type,
        createdAt: '2026-10-18T12:00:00Z',
    }
}

type UploadWrapper = Awaited<ReturnType<typeof renderView>>['wrapper']

async function chooseFiles(wrapper: UploadWrapper, files: File[]) {
    const input = wrapper.get('input[type="file"]')

    Object.defineProperty(input.element, 'files', {
        configurable: true,
        value: files,
    })

    await input.trigger('change')
}

async function submitUploads(wrapper: UploadWrapper) {
    await wrapper.get('button[aria-label="Upload selected files"]').trigger('click')
}

describe('project file upload recovery and lifecycle', () => {
    it('stops the batch and redirects to login on HTTP 401', async () => {
        const { wrapper, router, session, uploadPath } = await renderView()

        const first = makeFile('gripper.step')
        const second = makeFile('housing.glb')

        vi.mocked(uploadProjectFile)
            .mockRejectedValueOnce(new ProjectFileRequestError(401, 'Unauthorized'))
            .mockResolvedValueOnce(uploadedFile(second, '44444444-4444-4444-8444-444444444444'))

        await chooseFiles(wrapper, [first, second])
        await submitUploads(wrapper)

        await vi.waitFor(() => {
            expect(session.status).toBe('unauthenticated')
            expect(router.currentRoute.value.name).toBe('login')
        })

        expect(router.currentRoute.value.query.redirect).toBe(uploadPath)
        expect(uploadProjectFile).toHaveBeenCalledTimes(1)

        wrapper.unmount()
    })

    it('stops the batch after an uncertain HTTP 500 outcome', async () => {
        const { wrapper } = await renderView()

        const first = makeFile('gripper.step')
        const second = makeFile('housing.glb')

        vi.mocked(uploadProjectFile)
            .mockRejectedValueOnce(new ProjectFileRequestError(500, 'Internal Server Error'))
            .mockResolvedValueOnce(uploadedFile(second, '55555555-5555-4555-8555-555555555555'))

        await chooseFiles(wrapper, [first, second])
        await submitUploads(wrapper)

        await vi.waitFor(() => {
            expect(wrapper.findAll('ul[aria-label="Selected files"] li')[0]!.text()).toContain(
                'Needs review',
            )
        })

        await flushPromises()

        expect(uploadProjectFile).toHaveBeenCalledTimes(1)
        expect(wrapper.text()).toContain('Check the project')

        // The remaining selected file must not bypass review.
        expect(
            wrapper.get('button[aria-label="Upload selected files"]').attributes('disabled'),
        ).toBeDefined()
        expect(wrapper.find('button[aria-label="Retry failed uploads"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('does not automatically retry an uncertain network outcome', async () => {
        const { wrapper } = await renderView()

        vi.mocked(uploadProjectFile).mockRejectedValueOnce(new TypeError('Failed to fetch'))

        await chooseFiles(wrapper, [makeFile('gripper.step')])
        await submitUploads(wrapper)

        await vi.waitFor(() => {
            expect(wrapper.get('ul[aria-label="Selected files"] li').text()).toContain(
                'Needs review',
            )
        })

        expect(wrapper.find('button[aria-label="Retry failed uploads"]').exists()).toBe(false)

        expect(
            wrapper.get('button[aria-label="Upload selected files"]').attributes('disabled'),
        ).toBeDefined()

        await submitUploads(wrapper)

        expect(uploadProjectFile).toHaveBeenCalledTimes(1)

        wrapper.unmount()
    })

    it('aborts the active upload when switching projects', async () => {
        const { wrapper } = await renderView()

        const first = makeFile('gripper.step')
        const second = makeFile('housing.glb')

        let resolveFirst!: (value: ProjectFile) => void

        const pending = new Promise<ProjectFile>((resolve) => {
            resolveFirst = resolve
        })

        vi.mocked(uploadProjectFile)
            .mockReturnValueOnce(pending)
            .mockResolvedValueOnce(uploadedFile(second, '66666666-6666-4666-8666-666666666666'))

        await chooseFiles(wrapper, [first, second])
        await submitUploads(wrapper)

        const signal = vi.mocked(uploadProjectFile).mock.calls[0]?.[2]

        expect(signal).toBeInstanceOf(AbortSignal)
        expect(signal?.aborted).toBe(false)

        await wrapper.setProps({ projectId: otherProjectId })

        expect(signal?.aborted).toBe(true)
        expect(wrapper.find('ul[aria-label="Selected files"]').exists()).toBe(false)

        resolveFirst(uploadedFile(first, '77777777-7777-4777-8777-777777777777'))

        await flushPromises()

        expect(uploadProjectFile).toHaveBeenCalledTimes(1)
        expect(wrapper.get('a').attributes('href')).toBe(`/projects/${otherProjectId}`)

        wrapper.unmount()
    })

    it('aborts on unmount and prevents the next file from uploading', async () => {
        const { wrapper } = await renderView()

        const first = makeFile('gripper.step')
        const second = makeFile('housing.glb')

        let resolveFirst!: (value: ProjectFile) => void

        const pending = new Promise<ProjectFile>((resolve) => {
            resolveFirst = resolve
        })

        vi.mocked(uploadProjectFile)
            .mockReturnValueOnce(pending)
            .mockResolvedValueOnce(uploadedFile(second, '88888888-8888-4888-8888-888888888888'))

        await chooseFiles(wrapper, [first, second])
        await submitUploads(wrapper)

        const signal = vi.mocked(uploadProjectFile).mock.calls[0]?.[2]

        expect(signal?.aborted).toBe(false)

        wrapper.unmount()

        expect(signal?.aborted).toBe(true)

        resolveFirst(uploadedFile(first, '99999999-9999-4999-8999-999999999999'))

        await flushPromises()

        expect(uploadProjectFile).toHaveBeenCalledTimes(1)
    })

    it('offers project-directory navigation after a successful upload', async () => {
        const { wrapper, router } = await renderView()
        const file = makeFile('gripper.step')

        vi.mocked(uploadProjectFile).mockResolvedValueOnce(
            uploadedFile(file, 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb'),
        )

        expect(wrapper.find('a[aria-label="View project files"]').exists()).toBe(false)

        await chooseFiles(wrapper, [file])
        await submitUploads(wrapper)

        await vi.waitFor(() => {
            expect(wrapper.get('ul[aria-label="Selected files"] li').text()).toContain('Uploaded')
        })

        // Upload results remain visible until the user chooses to leave.
        expect(router.currentRoute.value.name).toBe('project-file-upload')

        const link = wrapper.get('a[aria-label="View project files"]')

        expect(link.attributes('href')).toBe(`/projects/${projectId}`)

        await link.trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('project-overview')
        })

        expect(uploadProjectFile).toHaveBeenCalledTimes(1)

        wrapper.unmount()
    })

    it('offers directory inspection after an uncertain upload result', async () => {
        const { wrapper, router } = await renderView()

        const first = makeFile('gripper.step')
        const second = makeFile('housing.glb')

        vi.mocked(uploadProjectFile).mockRejectedValueOnce(
            new ProjectFileRequestError(500, 'Internal Server Error'),
        )

        await chooseFiles(wrapper, [first, second])
        await submitUploads(wrapper)

        await vi.waitFor(() => {
            expect(wrapper.findAll('ul[aria-label="Selected files"] li')[0]!.text()).toContain(
                'Needs review',
            )
        })

        expect(uploadProjectFile).toHaveBeenCalledTimes(1)
        expect(router.currentRoute.value.name).toBe('project-file-upload')

        const link = wrapper.get('a[aria-label="View project files"]')

        expect(link.attributes('href')).toBe(`/projects/${projectId}`)

        await link.trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('project-overview')
        })

        // Inspecting project files must not retry the uncertain upload.
        expect(uploadProjectFile).toHaveBeenCalledTimes(1)

        wrapper.unmount()
    })

    it('prevents changing the file queue during an active upload', async () => {
        const { wrapper } = await renderView()

        const first = makeFile('gripper.step')
        const second = makeFile('housing.glb')
        const third = makeFile('new.step')

        let resolveFirst!: (value: ProjectFile) => void

        const pending = new Promise<ProjectFile>((resolve) => {
            resolveFirst = resolve
        })

        vi.mocked(uploadProjectFile).mockReturnValueOnce(pending)

        await chooseFiles(wrapper, [first, second])
        await submitUploads(wrapper)

        expect(
            wrapper.get('button[aria-label="Choose files"]').attributes('disabled'),
        ).toBeDefined()

        await wrapper.get('[data-testid="file-drop-zone"]').trigger('drop', {
            dataTransfer: { files: [third] },
        })

        await wrapper.get('button[aria-label="Remove housing.glb"]').trigger('click')

        expect(wrapper.findAll('ul[aria-label="Selected files"] li')).toHaveLength(2)

        expect(wrapper.text()).not.toContain('new.step')
        expect(uploadProjectFile).toHaveBeenCalledTimes(1)

        wrapper.unmount()

        resolveFirst(uploadedFile(first, 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'))

        await flushPromises()
    })
})
