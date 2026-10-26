import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import RevisionId from '../components/ui/RevisionId.vue'

const revisionId = '33333333-3333-4333-8333-333333333333'
const nextRevisionId = '44444444-4444-4444-8444-444444444444'

let writeText = vi.fn<(text: string) => Promise<void>>()

beforeEach(() => {
    writeText = vi.fn<(text: string) => Promise<void>>()
    writeText.mockResolvedValue(undefined)

    vi.stubGlobal('navigator', {
        clipboard: { writeText },
    })
})

afterEach(() => {
    vi.unstubAllGlobals()
})

describe('RevisionId presentation and copying', () => {
    it('displays eight characters while preserving the full ID in the title', () => {
        const wrapper = mount(RevisionId, {
            props: { id: revisionId },
        })

        expect(wrapper.get('code.revision-id__short').text()).toBe('33333333')
        expect(wrapper.get('code.revision-id__short').attributes('title')).toBe(revisionId)
        expect(wrapper.text()).not.toContain(revisionId)
        expect(wrapper.get('button').attributes('type')).toBe('button')
        expect(wrapper.get('button').attributes('aria-label')).toContain(
            'Copy full revision ID 33333333',
        )

        wrapper.unmount()
    })

    it('copies the complete UUID, never the abbreviated display value', async () => {
        const wrapper = mount(RevisionId, {
            props: { id: revisionId, label: 'parent revision' },
        })

        expect(wrapper.get('button').attributes('aria-label')).toContain(
            'Copy full parent revision ID',
        )

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(writeText).toHaveBeenCalledExactlyOnceWith(revisionId)
        expect(wrapper.get('[role="status"]').text()).toBe('Copied')
        expect(wrapper.get('code.revision-id__short').text()).toBe('33333333')

        wrapper.unmount()
    })

    it('reveals the full ID when clipboard permission is denied', async () => {
        writeText.mockRejectedValueOnce(new Error('Permission denied'))

        const wrapper = mount(RevisionId, {
            props: { id: revisionId },
        })

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(wrapper.get('[role="status"]').text()).toContain('Clipboard unavailable')
        expect(wrapper.get('.revision-id__fallback code').text()).toBe(revisionId)

        wrapper.unmount()
    })

    it('reveals the full ID when the Clipboard API is unavailable', async () => {
        vi.stubGlobal('navigator', { clipboard: undefined })

        const wrapper = mount(RevisionId, {
            props: { id: revisionId },
        })

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(wrapper.get('.revision-id__fallback code').text()).toBe(revisionId)

        wrapper.unmount()
    })

    it('resets a copied state when the supplied revision changes', async () => {
        const wrapper = mount(RevisionId, {
            props: { id: revisionId },
        })

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(wrapper.get('[role="status"]').text()).toBe('Copied')

        await wrapper.setProps({ id: nextRevisionId })

        expect(wrapper.find('[role="status"]').exists()).toBe(false)
        expect(wrapper.get('code.revision-id__short').text()).toBe('44444444')

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(writeText).toHaveBeenNthCalledWith(2, nextRevisionId)

        wrapper.unmount()
    })

    it('does not start duplicate clipboard writes while one is pending', async () => {
        let complete!: () => void

        writeText.mockImplementationOnce(
            () =>
                new Promise<void>((resolve) => {
                    complete = resolve
                }),
        )

        const wrapper = mount(RevisionId, {
            props: { id: revisionId },
        })

        await wrapper.get('button').trigger('click')
        await wrapper.get('button').trigger('click')

        expect(writeText).toHaveBeenCalledTimes(1)
        expect(wrapper.get('button').attributes('disabled')).toBeDefined()

        complete()
        await flushPromises()

        expect(wrapper.get('[role="status"]').text()).toBe('Copied')

        wrapper.unmount()
    })

    it('ignores an obsolete clipboard result after the ID changes', async () => {
        let complete!: () => void

        writeText.mockImplementationOnce(
            () =>
                new Promise<void>((resolve) => {
                    complete = resolve
                }),
        )

        const wrapper = mount(RevisionId, {
            props: { id: revisionId },
        })

        await wrapper.get('button').trigger('click')
        await wrapper.setProps({ id: nextRevisionId })

        complete()
        await flushPromises()

        expect(wrapper.find('[role="status"]').exists()).toBe(false)
        expect(wrapper.get('code.revision-id__short').text()).toBe('44444444')

        wrapper.unmount()
    })

    it('ignores a clipboard result after unmount', async () => {
        let complete!: () => void

        writeText.mockImplementationOnce(
            () =>
                new Promise<void>((resolve) => {
                    complete = resolve
                }),
        )

        const wrapper = mount(RevisionId, {
            props: { id: revisionId },
        })

        await wrapper.get('button').trigger('click')
        wrapper.unmount()

        complete()
        await flushPromises()

        expect(writeText).toHaveBeenCalledTimes(1)
    })
})
