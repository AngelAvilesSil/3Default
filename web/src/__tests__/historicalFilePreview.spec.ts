import { defineComponent, nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
    createProjectFileConversionJob,
    getProjectConversionJob,
    listProjectFileConversionJobs,
} from '../api/conversionJobs'
import { fetchProjectFilePreview, ProjectFilePreviewRequestError } from '../api/projectFilePreview'
import HistoricalFilePreview from '../components/HistoricalFilePreview.vue'

vi.mock('../api/projectFilePreview', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/projectFilePreview')>()

    return {
        ...original,
        fetchProjectFilePreview: vi.fn<typeof original.fetchProjectFilePreview>(),
    }
})

vi.mock('../api/conversionJobs', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/conversionJobs')>()

    return {
        ...original,
        createProjectFileConversionJob: vi.fn<typeof original.createProjectFileConversionJob>(),
        getProjectConversionJob: vi.fn<typeof original.getProjectConversionJob>(),
        listProjectFileConversionJobs: vi.fn<typeof original.listProjectFileConversionJobs>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const fileId = '22222222-2222-4222-8222-222222222222'
const otherFileId = '33333333-3333-4333-8333-333333333333'

const ThreePreviewCanvasStub = defineComponent({
    name: 'ThreePreviewCanvas',
    props: {
        glbData: { default: null },
    },
    template: '<div data-testid="historical-preview-viewer"></div>',
})

function mountPreview() {
    return mount(HistoricalFilePreview, {
        props: { projectId, projectFileId: fileId },
        global: {
            stubs: {
                ThreePreviewCanvas: ThreePreviewCanvasStub,
            },
        },
    })
}

function expectNoConversionCalls() {
    expect(createProjectFileConversionJob).not.toHaveBeenCalled()
    expect(getProjectConversionJob).not.toHaveBeenCalled()
    expect(listProjectFileConversionJobs).not.toHaveBeenCalled()
}

function deferred<T>() {
    let resolve!: (value: T) => void

    const promise = new Promise<T>((resolvePromise) => {
        resolve = resolvePromise
    })

    return { promise, resolve }
}

beforeEach(() => {
    vi.resetAllMocks()

    vi.mocked(fetchProjectFilePreview).mockResolvedValue({
        status: 'not-found',
    })
})

describe('read-only historical file preview', () => {
    it('retrieves an existing GLB preview using the exact source file ID', async () => {
        const glbData = new ArrayBuffer(32)

        vi.mocked(fetchProjectFilePreview).mockResolvedValueOnce({
            status: 'available',
            data: glbData,
        })

        const wrapper = mountPreview()

        await vi.waitFor(() => {
            expect(wrapper.find('[data-testid="historical-preview-viewer"]').exists()).toBe(true)
        })

        expect(fetchProjectFilePreview).toHaveBeenCalledExactlyOnceWith(
            projectId,
            fileId,
            expect.any(AbortSignal),
        )
        expect(wrapper.findComponent(ThreePreviewCanvasStub).props('glbData')).toBe(glbData)
        expectNoConversionCalls()

        wrapper.unmount()
    })

    it('reports a missing preview without creating conversion jobs', async () => {
        const wrapper = mountPreview()

        await vi.waitFor(() => {
            expect(wrapper.get('[data-testid="historical-preview-missing"]').text()).toContain(
                'No existing GLB preview',
            )
        })

        expect(wrapper.find('[data-testid="historical-preview-viewer"]').exists()).toBe(false)
        expect(fetchProjectFilePreview).toHaveBeenCalledTimes(1)
        expectNoConversionCalls()

        wrapper.unmount()
    })

    it('retries a missing preview using GET-only behavior', async () => {
        const glbData = new ArrayBuffer(24)

        vi.mocked(fetchProjectFilePreview)
            .mockResolvedValueOnce({ status: 'not-found' })
            .mockResolvedValueOnce({ status: 'available', data: glbData })

        const wrapper = mountPreview()

        await vi.waitFor(() => {
            expect(wrapper.find('[data-testid="historical-preview-missing"]').exists()).toBe(true)
        })

        await wrapper.get('button').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.find('[data-testid="historical-preview-viewer"]').exists()).toBe(true)
        })

        expect(fetchProjectFilePreview).toHaveBeenCalledTimes(2)
        expect(wrapper.findComponent(ThreePreviewCanvasStub).props('glbData')).toBe(glbData)
        expectNoConversionCalls()

        wrapper.unmount()
    })

    it('reports request failures and supports a safe retry', async () => {
        vi.mocked(fetchProjectFilePreview)
            .mockRejectedValueOnce(new TypeError('Network request failed'))
            .mockResolvedValueOnce({ status: 'not-found' })

        const wrapper = mountPreview()

        await vi.waitFor(() => {
            expect(wrapper.get('[data-testid="historical-preview-error"]').text()).toContain(
                'Unable to load',
            )
        })

        await wrapper.get('button').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.find('[data-testid="historical-preview-missing"]').exists()).toBe(true)
        })

        expect(fetchProjectFilePreview).toHaveBeenCalledTimes(2)
        expectNoConversionCalls()

        wrapper.unmount()
    })

    it('emits unauthorized once on an expired session', async () => {
        vi.mocked(fetchProjectFilePreview).mockRejectedValueOnce(
            new ProjectFilePreviewRequestError(401, 'Unauthorized'),
        )

        const wrapper = mountPreview()

        await vi.waitFor(() => {
            expect(wrapper.emitted('unauthorized')).toHaveLength(1)
        })

        expect(wrapper.get('[data-testid="historical-preview-unauthorized"]').text()).toContain(
            'Session expired',
        )
        expect(wrapper.find('[data-testid="historical-preview-viewer"]').exists()).toBe(false)
        expectNoConversionCalls()

        wrapper.unmount()
    })

    it('ignores an obsolete result after switching historical files', async () => {
        const oldRequest = deferred<Awaited<ReturnType<typeof fetchProjectFilePreview>>>()
        const currentGlb = new ArrayBuffer(16)

        vi.mocked(fetchProjectFilePreview)
            .mockReturnValueOnce(oldRequest.promise)
            .mockResolvedValueOnce({
                status: 'available',
                data: currentGlb,
            })

        const wrapper = mountPreview()

        expect(wrapper.find('[data-testid="historical-preview-loading"]').exists()).toBe(true)

        const oldSignal = vi.mocked(fetchProjectFilePreview).mock.calls[0]?.[2]

        await wrapper.setProps({ projectFileId: otherFileId })

        await vi.waitFor(() => {
            expect(wrapper.find('[data-testid="historical-preview-viewer"]').exists()).toBe(true)
        })

        expect(oldSignal?.aborted).toBe(true)
        expect(fetchProjectFilePreview).toHaveBeenNthCalledWith(
            2,
            projectId,
            otherFileId,
            expect.any(AbortSignal),
        )

        oldRequest.resolve({
            status: 'available',
            data: new ArrayBuffer(64),
        })

        await flushPromises()

        expect(wrapper.findComponent(ThreePreviewCanvasStub).props('glbData')).toBe(currentGlb)
        expectNoConversionCalls()

        wrapper.unmount()
    })

    it('aborts pending preview retrieval when inspection closes', async () => {
        const pending = deferred<Awaited<ReturnType<typeof fetchProjectFilePreview>>>()

        vi.mocked(fetchProjectFilePreview).mockReturnValueOnce(pending.promise)

        const wrapper = mountPreview()

        const signal = vi.mocked(fetchProjectFilePreview).mock.calls[0]?.[2]

        expect(signal?.aborted).toBe(false)

        wrapper.unmount()

        expect(signal?.aborted).toBe(true)

        pending.resolve({
            status: 'available',
            data: new ArrayBuffer(32),
        })

        await flushPromises()

        expect(wrapper.emitted('unauthorized')).toBeUndefined()
        expectNoConversionCalls()
    })

    it('reports WebGL rendering failure without modifying the source file', async () => {
        vi.mocked(fetchProjectFilePreview).mockResolvedValueOnce({
            status: 'available',
            data: new ArrayBuffer(32),
        })

        const wrapper = mountPreview()

        await vi.waitFor(() => {
            expect(wrapper.find('[data-testid="historical-preview-viewer"]').exists()).toBe(true)
        })

        wrapper
            .findComponent(ThreePreviewCanvasStub)
            .vm.$emit('load-error', new Error('Invalid GLB'))
        await nextTick()

        expect(wrapper.get('[data-testid="historical-preview-error"]').text()).toContain(
            'could not be rendered',
        )
        expect(wrapper.find('[data-testid="historical-preview-viewer"]').exists()).toBe(false)
        expectNoConversionCalls()

        wrapper.unmount()
    })
})
