import { defineComponent, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
    ConversionJobRequestError,
    createProjectFileConversionJob,
    getProjectConversionJob,
    listProjectFileConversionJobs,
    type ConversionJob,
    type ConversionJobStatus,
} from '../api/conversionJobs'
import { fetchProjectFilePreview } from '../api/projectFilePreview'
import ProjectFilePreview from '../components/ProjectFilePreview.vue'

vi.mock('../api/conversionJobs', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/conversionJobs')>()

    return {
        ...original,
        createProjectFileConversionJob: vi.fn<typeof original.createProjectFileConversionJob>(),
        getProjectConversionJob: vi.fn<typeof original.getProjectConversionJob>(),
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
const otherFileId = '33333333-3333-4333-8333-333333333333'
const jobId = '44444444-4444-4444-8444-444444444444'

function job(status: ConversionJobStatus, projectFileId = fileId): ConversionJob {
    return {
        id: jobId,
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

function mountPreview() {
    return mount(ProjectFilePreview, {
        props: {
            projectId,
            projectFileId: fileId,
            sourceFilename: 'gripper.step',
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

async function advancePolling(milliseconds = 10000) {
    await vi.advanceTimersByTimeAsync(milliseconds)
    await nextTick()
}

beforeEach(() => {
    vi.resetAllMocks()

    vi.mocked(fetchProjectFilePreview).mockResolvedValue({
        status: 'not-found',
    })

    vi.mocked(listProjectFileConversionJobs).mockResolvedValue([])
})

afterEach(() => {
    vi.useRealTimers()
})

describe('automatic preview conversion lifecycle', () => {
    it('polls a newly created job and displays the completed preview', async () => {
        vi.useFakeTimers()

        const glbData = new ArrayBuffer(32)

        vi.mocked(fetchProjectFilePreview)
            .mockResolvedValueOnce({ status: 'not-found' })
            .mockResolvedValueOnce({
                status: 'available',
                data: glbData,
            })

        vi.mocked(createProjectFileConversionJob).mockResolvedValueOnce(job('pending'))

        vi.mocked(getProjectConversionJob)
            .mockResolvedValueOnce(job('running'))
            .mockResolvedValueOnce(job('succeeded'))

        const wrapper = mountPreview()

        await advancePolling()

        expect(createProjectFileConversionJob).toHaveBeenCalledTimes(1)
        expect(getProjectConversionJob).toHaveBeenCalledTimes(2)
        expect(getProjectConversionJob).toHaveBeenCalledWith(
            projectId,
            jobId,
            expect.any(AbortSignal),
        )

        expect(fetchProjectFilePreview).toHaveBeenCalledTimes(2)
        expect(wrapper.find('[data-testid="preview-viewer"]').exists()).toBe(true)

        expect(wrapper.findComponent(ThreePreviewCanvasStub).props('glbData')).toBe(glbData)

        wrapper.unmount()
    })

    it('resumes an existing running job and displays its result', async () => {
        vi.useFakeTimers()

        vi.mocked(fetchProjectFilePreview)
            .mockResolvedValueOnce({ status: 'not-found' })
            .mockResolvedValueOnce({
                status: 'available',
                data: new ArrayBuffer(16),
            })

        vi.mocked(listProjectFileConversionJobs).mockResolvedValueOnce([job('running')])

        vi.mocked(getProjectConversionJob).mockResolvedValueOnce(job('succeeded'))

        const wrapper = mountPreview()

        await advancePolling()

        expect(createProjectFileConversionJob).not.toHaveBeenCalled()
        expect(getProjectConversionJob).toHaveBeenCalledTimes(1)
        expect(wrapper.find('[data-testid="preview-viewer"]').exists()).toBe(true)

        wrapper.unmount()
    })

    it('stops polling when a conversion job fails', async () => {
        vi.useFakeTimers()

        vi.mocked(listProjectFileConversionJobs).mockResolvedValueOnce([job('running')])

        vi.mocked(getProjectConversionJob).mockResolvedValueOnce(job('failed'))

        const wrapper = mountPreview()

        await advancePolling()

        expect(getProjectConversionJob).toHaveBeenCalledTimes(1)
        expect(wrapper.get('[data-testid="preview-error"]').text()).toContain(
            'Preview generation failed',
        )

        expect(createProjectFileConversionJob).not.toHaveBeenCalled()
        expect(fetchProjectFilePreview).toHaveBeenCalledTimes(1)

        wrapper.unmount()
    })

    it('reconciles HTTP 409 with the existing active job', async () => {
        vi.useFakeTimers()

        vi.mocked(fetchProjectFilePreview)
            .mockResolvedValueOnce({ status: 'not-found' })
            .mockResolvedValueOnce({
                status: 'available',
                data: new ArrayBuffer(16),
            })

        vi.mocked(listProjectFileConversionJobs)
            .mockResolvedValueOnce([])
            .mockResolvedValueOnce([job('pending')])

        vi.mocked(createProjectFileConversionJob).mockRejectedValueOnce(
            new ConversionJobRequestError(409, 'Conflict'),
        )

        vi.mocked(getProjectConversionJob).mockResolvedValueOnce(job('succeeded'))

        const wrapper = mountPreview()

        await advancePolling()

        expect(listProjectFileConversionJobs).toHaveBeenCalledTimes(2)
        expect(createProjectFileConversionJob).toHaveBeenCalledTimes(1)
        expect(getProjectConversionJob).toHaveBeenCalledTimes(1)

        expect(wrapper.find('[data-testid="preview-viewer"]').exists()).toBe(true)

        wrapper.unmount()
    })

    it('rechecks history after an uncertain POST without resubmitting', async () => {
        vi.useFakeTimers()

        vi.mocked(listProjectFileConversionJobs).mockResolvedValueOnce([]).mockResolvedValueOnce([])

        vi.mocked(createProjectFileConversionJob).mockRejectedValueOnce(
            new TypeError('Failed to fetch'),
        )

        const wrapper = mountPreview()

        await advancePolling()

        expect(createProjectFileConversionJob).toHaveBeenCalledTimes(1)
        expect(listProjectFileConversionJobs).toHaveBeenCalledTimes(2)

        expect(wrapper.get('[data-testid="preview-error"]').text()).toContain(
            'could not be confirmed',
        )

        wrapper.unmount()
    })

    it('reports an expired session during job polling', async () => {
        vi.useFakeTimers()

        vi.mocked(listProjectFileConversionJobs).mockResolvedValueOnce([job('running')])

        vi.mocked(getProjectConversionJob).mockRejectedValueOnce(
            new ConversionJobRequestError(401, 'Unauthorized'),
        )

        const wrapper = mountPreview()

        await advancePolling()

        expect(wrapper.emitted('unauthorized')).toHaveLength(1)
        expect(getProjectConversionJob).toHaveBeenCalledTimes(1)
        expect(createProjectFileConversionJob).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('rejects a job status belonging to a different file', async () => {
        vi.useFakeTimers()

        vi.mocked(listProjectFileConversionJobs).mockResolvedValueOnce([job('pending')])

        vi.mocked(getProjectConversionJob).mockResolvedValueOnce(job('succeeded', otherFileId))

        const wrapper = mountPreview()

        await advancePolling()

        expect(getProjectConversionJob).toHaveBeenCalledTimes(1)
        expect(wrapper.get('[data-testid="preview-error"]').text()).toContain('Unable to verify')

        expect(fetchProjectFilePreview).toHaveBeenCalledTimes(1)
        expect(createProjectFileConversionJob).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('ignores a late polling response after switching files', async () => {
        vi.useFakeTimers()

        const statusRequest = deferred<ConversionJob>()

        vi.mocked(listProjectFileConversionJobs).mockResolvedValueOnce([job('running')])

        vi.mocked(getProjectConversionJob).mockReturnValueOnce(statusRequest.promise)

        const wrapper = mountPreview()

        await advancePolling(5000)

        expect(getProjectConversionJob).toHaveBeenCalledTimes(1)

        const signal = vi.mocked(getProjectConversionJob).mock.calls[0]?.[2]

        expect(signal?.aborted).toBe(false)

        await wrapper.setProps({
            projectFileId: otherFileId,
            sourceFilename: 'notes.pdf',
        })

        await nextTick()

        expect(signal?.aborted).toBe(true)

        statusRequest.resolve(job('succeeded'))
        await advancePolling(5000)

        expect(getProjectConversionJob).toHaveBeenCalledTimes(1)
        expect(fetchProjectFilePreview).toHaveBeenCalledTimes(2)

        expect(wrapper.get('[data-testid="preview-not-found"]').text()).toContain('not supported')

        wrapper.unmount()
    })

    it('stops polling and aborts requests when the preview closes', async () => {
        vi.useFakeTimers()

        const statusRequest = deferred<ConversionJob>()

        vi.mocked(listProjectFileConversionJobs).mockResolvedValueOnce([job('pending')])

        vi.mocked(getProjectConversionJob).mockReturnValueOnce(statusRequest.promise)

        const wrapper = mountPreview()

        await advancePolling(5000)

        expect(getProjectConversionJob).toHaveBeenCalledTimes(1)

        const signal = vi.mocked(getProjectConversionJob).mock.calls[0]?.[2]

        expect(signal?.aborted).toBe(false)

        wrapper.unmount()

        expect(signal?.aborted).toBe(true)

        statusRequest.resolve(job('succeeded'))

        await advancePolling(5000)

        expect(getProjectConversionJob).toHaveBeenCalledTimes(1)
        expect(fetchProjectFilePreview).toHaveBeenCalledTimes(1)
    })
})
