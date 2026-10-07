import { defineComponent, nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ProjectFilePreview from '../components/ProjectFilePreview.vue'

type PreviewResult =
    | {
          status: 'available'
          data: ArrayBuffer
      }
    | {
          status: 'not-found'
      }

const previewApiMocks = vi.hoisted(() => ({
    fetchProjectFilePreview:
        vi.fn<
            (
                projectId: string,
                projectFileId: string,
                signal?: AbortSignal,
            ) => Promise<PreviewResult>
        >(),
}))

vi.mock('../api/projectFilePreview', () => ({
    fetchProjectFilePreview: previewApiMocks.fetchProjectFilePreview,
}))

const ThreePreviewCanvasStub = defineComponent({
    name: 'ThreePreviewCanvas',
    props: {
        glbData: {
            default: null,
        },
    },
    emits: ['load-error'],
    template: '<div data-testid="preview-viewer"></div>',
})

function mountPreview(projectId = 'project-id', projectFileId = 'project-file-id') {
    return mount(ProjectFilePreview, {
        props: {
            projectId,
            projectFileId,
        },
        global: {
            stubs: {
                ThreePreviewCanvas: ThreePreviewCanvasStub,
            },
        },
    })
}

function deferred<T>() {
    let resolve!: (value: T | PromiseLike<T>) => void
    let reject!: (reason?: unknown) => void

    const promise = new Promise<T>((resolvePromise, rejectPromise) => {
        resolve = resolvePromise
        reject = rejectPromise
    })

    return {
        promise,
        resolve,
        reject,
    }
}

describe('ProjectFilePreview', () => {
    beforeEach(() => {
        previewApiMocks.fetchProjectFilePreview.mockReset()
    })

    it('shows a loading state while the preview request is pending', () => {
        const request = deferred<PreviewResult>()

        previewApiMocks.fetchProjectFilePreview.mockReturnValueOnce(request.promise)

        const wrapper = mountPreview()

        expect(wrapper.get('[data-testid="preview-loading"]').text()).toBe('Loading preview…')

        expect(previewApiMocks.fetchProjectFilePreview).toHaveBeenCalledWith(
            'project-id',
            'project-file-id',
            expect.any(AbortSignal),
        )

        wrapper.unmount()
    })

    it('passes successful preview bytes to the Three.js viewer', async () => {
        const data = new ArrayBuffer(16)

        previewApiMocks.fetchProjectFilePreview.mockResolvedValueOnce({
            status: 'available',
            data,
        })

        const wrapper = mountPreview()

        await flushPromises()

        expect(wrapper.find('[data-testid="preview-loading"]').exists()).toBe(false)
        expect(wrapper.find('[data-testid="preview-viewer"]').exists()).toBe(true)

        const viewer = wrapper.findComponent(ThreePreviewCanvasStub)

        expect(viewer.props('glbData')).toBe(data)

        wrapper.unmount()
    })

    it('shows a no-preview state for a 404 result', async () => {
        previewApiMocks.fetchProjectFilePreview.mockResolvedValueOnce({
            status: 'not-found',
        })

        const wrapper = mountPreview()

        await flushPromises()

        expect(wrapper.get('[data-testid="preview-not-found"]').text()).toBe(
            'No preview is available for this file yet.',
        )
        expect(wrapper.find('[data-testid="preview-viewer"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('shows a request error when preview loading fails', async () => {
        previewApiMocks.fetchProjectFilePreview.mockRejectedValueOnce(new Error('request failed'))

        const wrapper = mountPreview()

        await flushPromises()

        expect(wrapper.get('[data-testid="preview-error"]').text()).toBe(
            'The preview could not be loaded.',
        )
        expect(wrapper.find('[data-testid="preview-viewer"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('aborts the previous request and ignores its stale result', async () => {
        const firstRequest = deferred<PreviewResult>()

        previewApiMocks.fetchProjectFilePreview
            .mockReturnValueOnce(firstRequest.promise)
            .mockResolvedValueOnce({
                status: 'not-found',
            })

        const wrapper = mountPreview('project-one', 'file-one')

        const firstSignal = previewApiMocks.fetchProjectFilePreview.mock.calls[0]?.[2]

        expect(firstSignal?.aborted).toBe(false)

        await wrapper.setProps({
            projectId: 'project-two',
            projectFileId: 'file-two',
        })
        await flushPromises()

        expect(firstSignal?.aborted).toBe(true)
        expect(wrapper.get('[data-testid="preview-not-found"]').text()).toBe(
            'No preview is available for this file yet.',
        )

        firstRequest.resolve({
            status: 'available',
            data: new ArrayBuffer(8),
        })

        await flushPromises()

        expect(wrapper.get('[data-testid="preview-not-found"]').text()).toBe(
            'No preview is available for this file yet.',
        )
        expect(wrapper.find('[data-testid="preview-viewer"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('aborts an in-flight request when unmounted', () => {
        const request = deferred<PreviewResult>()

        previewApiMocks.fetchProjectFilePreview.mockReturnValueOnce(request.promise)

        const wrapper = mountPreview()

        const signal = previewApiMocks.fetchProjectFilePreview.mock.calls[0]?.[2]

        expect(signal?.aborted).toBe(false)

        wrapper.unmount()

        expect(signal?.aborted).toBe(true)
    })

    it('shows a render error when the Three.js viewer rejects the GLB', async () => {
        previewApiMocks.fetchProjectFilePreview.mockResolvedValueOnce({
            status: 'available',
            data: new ArrayBuffer(16),
        })

        const wrapper = mountPreview()

        await flushPromises()

        const viewer = wrapper.findComponent(ThreePreviewCanvasStub)

        viewer.vm.$emit('load-error', new Error('invalid GLB'))
        await nextTick()

        expect(wrapper.get('[data-testid="preview-error"]').text()).toBe(
            'The preview could not be rendered.',
        )
        expect(wrapper.find('[data-testid="preview-viewer"]').exists()).toBe(false)

        wrapper.unmount()
    })
})
