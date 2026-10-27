import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import CopyableId from '../components/ui/CopyableId.vue'

const authorId = '66666666-6666-4666-8666-666666666666'
const nextAuthorId = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'

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

describe('CopyableId user identity presentation', () => {
    it('shows a short user ID with the full value available on hover', () => {
        const wrapper = mount(CopyableId, {
            props: { id: authorId, label: 'user', kind: 'user' },
        })

        expect(wrapper.get('.user-id__short').text()).toBe('66666666')
        expect(wrapper.get('.user-id__short').attributes('title')).toBe(authorId)
        expect(wrapper.text()).not.toContain(authorId)
        expect(wrapper.get('button').attributes('aria-label')).toBe('Copy full user ID 66666666')

        wrapper.unmount()
    })

    it('copies the complete user UUID, not the abbreviated value', async () => {
        const wrapper = mount(CopyableId, {
            props: { id: authorId, label: 'user', kind: 'user' },
        })

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(writeText).toHaveBeenCalledExactlyOnceWith(authorId)
        expect(wrapper.get('[role="status"]').text()).toBe('Copied')

        wrapper.unmount()
    })

    it('reveals the full user ID when clipboard access fails', async () => {
        writeText.mockRejectedValueOnce(new Error('Clipboard denied'))

        const wrapper = mount(CopyableId, {
            props: { id: authorId, label: 'user', kind: 'user' },
        })

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(wrapper.get('.user-id__fallback code').text()).toBe(authorId)

        wrapper.unmount()
    })

    it('resets clipboard state when the user ID changes', async () => {
        const wrapper = mount(CopyableId, {
            props: { id: authorId, label: 'user', kind: 'user' },
        })

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(wrapper.get('[role="status"]').text()).toBe('Copied')

        await wrapper.setProps({ id: nextAuthorId })

        expect(wrapper.find('[role="status"]').exists()).toBe(false)
        expect(wrapper.get('.user-id__short').text()).toBe('aaaaaaaa')

        wrapper.unmount()
    })
})
