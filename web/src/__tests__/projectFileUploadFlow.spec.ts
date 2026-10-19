import { mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ProjectFileRequestError, uploadProjectFile, type ProjectFile } from '../api/projectFiles'
import { createAppRouter } from '../router'
import ProjectFileUploadView from '../views/ProjectFileUploadView.vue'

vi.mock('../api/projectFiles', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/projectFiles')>()

    return {
        ...original,
        uploadProjectFile: vi.fn<typeof original.uploadProjectFile>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'

beforeEach(() => {
    vi.resetAllMocks()
})

function renderView() {
    const router = createAppRouter(createMemoryHistory())

    const wrapper = mount(ProjectFileUploadView, {
        props: { projectId },
        global: { plugins: [router] },
    })

    return { wrapper, router }
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
        uploadedByUserId: '33333333-3333-4333-8333-333333333333',
        contentSha256: 'a'.repeat(64),
        originalFilename: file.name,
        mediaType: file.type,
        createdAt: '2026-10-18T12:00:00Z',
    }
}

async function chooseFiles(wrapper: ReturnType<typeof renderView>['wrapper'], files: File[]) {
    const input = wrapper.get('input[type="file"]')

    Object.defineProperty(input.element, 'files', {
        configurable: true,
        value: files,
    })

    await input.trigger('change')
}

function uploadButton(wrapper: ReturnType<typeof renderView>['wrapper']) {
    return wrapper.get('button[aria-label="Upload selected files"]')
}

describe('project file upload flow', () => {
    it('requires explicit confirmation before uploading', async () => {
        const { wrapper } = renderView()
        const file = makeFile('gripper.step')

        expect(uploadButton(wrapper).attributes('disabled')).toBeDefined()

        await chooseFiles(wrapper, [file])

        expect(uploadProjectFile).not.toHaveBeenCalled()
        expect(uploadButton(wrapper).attributes('disabled')).toBeUndefined()

        wrapper.unmount()
    })

    it('uploads one file to the correct project and shows success', async () => {
        const { wrapper } = renderView()
        const file = makeFile('gripper.step')

        vi.mocked(uploadProjectFile).mockResolvedValueOnce(
            uploadedFile(file, '22222222-2222-4222-8222-222222222222'),
        )

        await chooseFiles(wrapper, [file])
        await uploadButton(wrapper).trigger('click')

        await vi.waitFor(() => {
            expect(uploadProjectFile).toHaveBeenCalledTimes(1)
            expect(wrapper.get('ul[aria-label="Selected files"] li').text()).toContain('Uploaded')
        })

        expect(uploadProjectFile).toHaveBeenCalledWith(projectId, file, expect.any(AbortSignal))

        expect(wrapper.get('h2').text()).toBe('Upload files')
        expect(wrapper.get('a').attributes('href')).toBe(`/projects/${projectId}`)

        wrapper.unmount()
    })

    it('uploads multiple files sequentially', async () => {
        const { wrapper } = renderView()

        const first = makeFile('gripper.step')
        const second = makeFile('housing.glb')

        let resolveFirst!: (result: ProjectFile) => void

        const firstRequest = new Promise<ProjectFile>((resolve) => {
            resolveFirst = resolve
        })

        vi.mocked(uploadProjectFile)
            .mockReturnValueOnce(firstRequest)
            .mockResolvedValueOnce(uploadedFile(second, '55555555-5555-4555-8555-555555555555'))

        await chooseFiles(wrapper, [first, second])
        await uploadButton(wrapper).trigger('click')

        expect(uploadProjectFile).toHaveBeenCalledTimes(1)
        expect(vi.mocked(uploadProjectFile).mock.calls[0]?.[1]).toBe(first)

        const rows = wrapper.findAll('ul[aria-label="Selected files"] li')

        expect(rows[0]!.text()).toContain('Uploading')

        resolveFirst(uploadedFile(first, '44444444-4444-4444-8444-444444444444'))

        await vi.waitFor(() => {
            expect(uploadProjectFile).toHaveBeenCalledTimes(2)
            expect(rows[0]!.text()).toContain('Uploaded')
            expect(rows[1]!.text()).toContain('Uploaded')
        })

        expect(vi.mocked(uploadProjectFile).mock.calls[1]?.[1]).toBe(second)

        wrapper.unmount()
    })

    it('prevents duplicate submissions while an upload is pending', async () => {
        const { wrapper } = renderView()
        const file = makeFile('gripper.step')

        let resolveUpload!: (result: ProjectFile) => void

        const pending = new Promise<ProjectFile>((resolve) => {
            resolveUpload = resolve
        })

        vi.mocked(uploadProjectFile).mockReturnValueOnce(pending)

        await chooseFiles(wrapper, [file])
        await uploadButton(wrapper).trigger('click')

        expect(uploadButton(wrapper).attributes('disabled')).toBeDefined()

        await uploadButton(wrapper).trigger('click')

        expect(uploadProjectFile).toHaveBeenCalledTimes(1)

        resolveUpload(uploadedFile(file, '66666666-6666-4666-8666-666666666666'))

        await vi.waitFor(() => {
            expect(wrapper.get('ul[aria-label="Selected files"] li').text()).toContain('Uploaded')
        })

        wrapper.unmount()
    })

    it('retries a confirmed failed upload without repeating successes', async () => {
        const { wrapper } = renderView()

        const first = makeFile('gripper.step')
        const second = makeFile('housing.glb')

        vi.mocked(uploadProjectFile)
            .mockResolvedValueOnce(uploadedFile(first, '77777777-7777-4777-8777-777777777777'))
            .mockRejectedValueOnce(new ProjectFileRequestError(400, 'Bad Request'))
            .mockResolvedValueOnce(uploadedFile(second, '88888888-8888-4888-8888-888888888888'))

        await chooseFiles(wrapper, [first, second])
        await uploadButton(wrapper).trigger('click')

        await vi.waitFor(() => {
            expect(uploadProjectFile).toHaveBeenCalledTimes(2)
            expect(wrapper.find('button[aria-label="Retry failed uploads"]').exists()).toBe(true)
        })

        const rows = wrapper.findAll('ul[aria-label="Selected files"] li')

        expect(rows[0]!.text()).toContain('Uploaded')
        expect(rows[1]!.text()).toContain('Failed')

        await wrapper.get('button[aria-label="Retry failed uploads"]').trigger('click')

        await vi.waitFor(() => {
            expect(uploadProjectFile).toHaveBeenCalledTimes(3)
            expect(rows[1]!.text()).toContain('Uploaded')
        })

        expect(vi.mocked(uploadProjectFile).mock.calls[2]?.[1]).toBe(second)
        expect(rows[0]!.text()).toContain('Uploaded')

        wrapper.unmount()
    })
})
