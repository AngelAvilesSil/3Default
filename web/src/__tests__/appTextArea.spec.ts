import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppTextArea from '../components/ui/AppTextArea.vue'

describe('AppTextArea', () => {
    it('associates its label with the textarea and shows the initial value', () => {
        const wrapper = mount(AppTextArea, {
            props: {
                id: 'project-description',
                label: 'Description',
                modelValue: 'Mechanical design',
            },
        })

        expect(wrapper.get('label').attributes('for')).toBe('project-description')
        expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe(
            'Mechanical design',
        )
        expect(wrapper.find('#project-description-error').exists()).toBe(false)
    })

    it('emits model updates when the user types', async () => {
        const wrapper = mount(AppTextArea, {
            props: {
                id: 'project-description',
                label: 'Description',
                modelValue: '',
            },
        })

        await wrapper.get('textarea').setValue('Updated description')

        expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['Updated description'])
    })

    it('supports disabled state and textarea configuration', () => {
        const wrapper = mount(AppTextArea, {
            props: {
                id: 'project-description',
                label: 'Description',
                modelValue: '',
                name: 'description',
                placeholder: 'Describe your project',
                rows: 5,
                disabled: true,
            },
        })

        const textarea = wrapper.get('textarea')

        expect(textarea.attributes('disabled')).toBeDefined()
        expect(textarea.attributes('rows')).toBe('5')
        expect(textarea.attributes('name')).toBe('description')
        expect(textarea.attributes('placeholder')).toBe('Describe your project')
    })

    it('exposes validation errors accessibly', () => {
        const wrapper = mount(AppTextArea, {
            props: {
                id: 'project-description',
                label: 'Description',
                modelValue: '',
                error: 'Description is too long.',
            },
        })

        const textarea = wrapper.get('textarea')

        expect(textarea.attributes('aria-invalid')).toBe('true')
        expect(textarea.attributes('aria-describedby')).toBe('project-description-error')
        expect(wrapper.get('#project-description-error').text()).toBe('Description is too long.')
    })
})
