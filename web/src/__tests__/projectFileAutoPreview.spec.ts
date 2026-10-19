import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
    createProjectFileConversionJob,
    listProjectFileConversionJobs,
    type ConversionJob,
} from '../api/conversionJobs'
import { fetchProjectFilePreview } from '../api/projectFilePreview'
import ProjectFilePreview from '../components/ProjectFilePreview.vue'

vi.mock('../api/conversionJobs', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/conversionJobs')>()

    return {
        ...original,
        createProjectFileConversionJob: vi.fn<typeof original.createProjectFileConversionJob>(),
        listProjectFileConversionJobs: vi.fn<typeof original.listProjectFileConversionJobs>(),
    }
})

vi.mock('../api/projectFilePreview', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/projectFilePreview')>()

    return {
        ...original,
        fetchProjectFilePreview: vi.fn<typeof original.fetchProjectFilePreview>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const fileId = '22222222-2222-4222-8222-222222222222'
const secondFileId = '33333333-3333-4333-8333-333333333333'

function conversionJob(status: ConversionJob['status'], projectFileId = fileId): ConversionJob {
    return {
        id: '44444444-4444-4444-8444-444444444444',
        projectId,
        projectFileId,
        status,
        attemptCount: status === 'pending' ? 0 : 1,
        lastError: status === 'failed' ? 'Converter rejected the source' : null,
        createdAt: '2026-10-18T12:00:00Z',
        startedAt: status === 'pending' ? null : '2026-10-18T12:01:00Z',
        finishedAt: status === 'failed' || status === 'succeeded' ? '2026-10-18T12:02:00Z' : null,
        updatedAt: '2026-10-18T12:02:00Z',
    }
}

const ThreePreviewCanvasStub = defineComponent({
    name: 'ThreePreviewCanvas',
    props: {
        glbData: { default: null },
    },
    template: '<div data-testid="preview-viewer"></div>',
})

function mountPreview(sourceFilename = 'gripper.step') {
    return mount(ProjectFilePreview, {
        props: {
            projectId,
            projectFileId: fileId,
            sourceFilename,
        },
        global: {
            stubs: {
                ThreePreviewCanvas: ThreePreviewCanvasStub,
            },
        },
    })
}

function deferred<T>() {
    let resolve!: (value: T) => void

    const promise = new Promise<T>((resolvePromise) => {
        resolve = resolvePromise
    })

    return { promise, resolve }
}

describe('automatic file preview preparation', () => {
    beforeEach(() => {
        vi.resetAllMocks()

        vi.mocked(fetchProjectFilePreview).mockResolvedValue({
            status: 'not-found',
        })
    })

    it('uses an existing preview without creating another conversion job', async () => {
        const data = new ArrayBuffer(16)

        vi.mocked(fetchProjectFilePreview).mockResolvedValueOnce({
            status: 'available',
            data,
        })

        const wrapper = mountPreview()

        await flushPromises()

        expect(wrapper.find('[data-testid="preview-viewer"]').exists()).toBe(true)
        expect(listProjectFileConversionJobs).not.toHaveBeenCalled()
        expect(createProjectFileConversionJob).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it.each(['gripper.step', 'housing.glb'])(
        'automatically requests a preview for %s when no job exists',
        async (filename) => {
            vi.mocked(listProjectFileConversionJobs).mockResolvedValueOnce([])
            vi.mocked(createProjectFileConversionJob).mockResolvedValueOnce(
                conversionJob('pending'),
            )

            const wrapper = mountPreview(filename)

            await vi.waitFor(() => {
                expect(createProjectFileConversionJob).toHaveBeenCalledTimes(1)
            })

            expect(fetchProjectFilePreview).toHaveBeenCalledWith(
                projectId,
                fileId,
                expect.any(AbortSignal),
            )

            expect(listProjectFileConversionJobs).toHaveBeenCalledWith(
                projectId,
                fileId,
                expect.any(AbortSignal),
            )

            expect(createProjectFileConversionJob).toHaveBeenCalledWith(
                projectId,
                fileId,
                expect.any(AbortSignal),
            )

            expect(wrapper.get('[data-testid="preview-preparing"]').text()).toContain(
                'Preparing preview',
            )

            expect(wrapper.find('button').exists()).toBe(false)

            wrapper.unmount()
        },
    )

    it.each(['pending', 'running'] as const)(
        'resumes an existing %s job without creating a duplicate',
        async (status) => {
            vi.mocked(listProjectFileConversionJobs).mockResolvedValueOnce([conversionJob(status)])

            const wrapper = mountPreview()

            await vi.waitFor(() => {
                expect(wrapper.find('[data-testid="preview-preparing"]').exists()).toBe(true)
            })

            expect(createProjectFileConversionJob).not.toHaveBeenCalled()

            wrapper.unmount()
        },
    )

    it('does not automatically retry a previously failed conversion', async () => {
        vi.mocked(listProjectFileConversionJobs).mockResolvedValueOnce([conversionJob('failed')])

        const wrapper = mountPreview()

        await vi.waitFor(() => {
            expect(wrapper.get('[data-testid="preview-error"]').text()).toContain(
                'Preview generation failed',
            )
        })

        expect(createProjectFileConversionJob).not.toHaveBeenCalled()
        expect(wrapper.find('button').exists()).toBe(false)

        wrapper.unmount()
    })

    it('does not recreate a successful job when its preview is missing', async () => {
        vi.mocked(listProjectFileConversionJobs).mockResolvedValueOnce([conversionJob('succeeded')])

        const wrapper = mountPreview()

        await vi.waitFor(() => {
            expect(wrapper.get('[data-testid="preview-error"]').text()).toContain(
                'preview is unavailable',
            )
        })

        expect(createProjectFileConversionJob).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('does not request conversion for unsupported file formats', async () => {
        const wrapper = mountPreview('engineering-notes.pdf')

        await flushPromises()

        expect(wrapper.get('[data-testid="preview-not-found"]').text()).toContain('not supported')

        expect(listProjectFileConversionJobs).not.toHaveBeenCalled()
        expect(createProjectFileConversionJob).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('does not create a job when checking history fails', async () => {
        vi.mocked(listProjectFileConversionJobs).mockRejectedValueOnce(
            new TypeError('Failed to fetch'),
        )

        const wrapper = mountPreview()

        await vi.waitFor(() => {
            expect(wrapper.get('[data-testid="preview-error"]').text()).toContain(
                'Unable to check preview preparation',
            )
        })

        expect(createProjectFileConversionJob).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('aborts stale history requests when switching files', async () => {
        const firstHistory = deferred<ConversionJob[]>()

        vi.mocked(listProjectFileConversionJobs).mockReturnValueOnce(firstHistory.promise)

        const wrapper = mountPreview()

        await vi.waitFor(() => {
            expect(listProjectFileConversionJobs).toHaveBeenCalledTimes(1)
        })

        const firstSignal = vi.mocked(listProjectFileConversionJobs).mock.calls[0]?.[2]

        expect(firstSignal?.aborted).toBe(false)

        vi.mocked(fetchProjectFilePreview).mockResolvedValueOnce({
            status: 'available',
            data: new ArrayBuffer(16),
        })

        await wrapper.setProps({
            projectFileId: secondFileId,
            sourceFilename: 'second.glb',
        })

        await flushPromises()

        expect(firstSignal?.aborted).toBe(true)

        firstHistory.resolve([])
        await flushPromises()

        expect(createProjectFileConversionJob).not.toHaveBeenCalled()
        expect(wrapper.find('[data-testid="preview-viewer"]').exists()).toBe(true)

        wrapper.unmount()
    })

    it('aborts an active history request when the preview closes', async () => {
        const history = deferred<ConversionJob[]>()

        vi.mocked(listProjectFileConversionJobs).mockReturnValueOnce(history.promise)

        const wrapper = mountPreview()

        await vi.waitFor(() => {
            expect(listProjectFileConversionJobs).toHaveBeenCalledTimes(1)
        })

        const signal = vi.mocked(listProjectFileConversionJobs).mock.calls[0]?.[2]

        expect(signal?.aborted).toBe(false)

        wrapper.unmount()

        expect(signal?.aborted).toBe(true)

        history.resolve([])
        await flushPromises()

        expect(createProjectFileConversionJob).not.toHaveBeenCalled()
    })
})
