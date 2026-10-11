import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppButton from '../components/ui/AppButton.vue'
import AppTextField from '../components/ui/AppTextField.vue'

describe('AppButton', () => {
    it('renders a native button with a safe default type', () => {
        const wrapper = mount(AppButton, {
            slots: {
                default: 'Sign in',
            },
        })

        const button = wrapper.get('button')

        expect(button.text()).toBe('Sign in')
        expect(button.attributes('type')).toBe('button')
    })

    it('supports a submit button', () => {
        const wrapper = mount(AppButton, {
            props: {
                type: 'submit',
            },
            slots: {
                default: 'Sign in',
            },
        })

        expect(wrapper.get('button').attributes('type')).toBe('submit')
    })

    it('supports reusable visual variants', () => {
        const wrapper = mount(AppButton, {
            props: {
                variant: 'secondary',
            },
            slots: {
                default: 'Cancel',
            },
        })

        expect(wrapper.get('button').classes()).toContain('ui-button--secondary')
    })

    it('prevents interaction when disabled or loading', async () => {
        const wrapper = mount(AppButton, {
            props: {
                loading: true,
            },
            slots: {
                default: 'Sign in',
            },
        })

        const button = wrapper.get('button')

        expect(button.attributes('disabled')).toBeDefined()
        expect(button.attributes('aria-busy')).toBe('true')

        await wrapper.setProps({
            loading: false,
            disabled: true,
        })

        expect(button.attributes('disabled')).toBeDefined()
    })
})

describe('AppTextField', () => {
    it('associates its visible label with the input', () => {
        const wrapper = mount(AppTextField, {
            props: {
                id: 'login-email',
                label: 'Email address',
                modelValue: '',
                type: 'email',
                autocomplete: 'username',
            },
        })

        expect(wrapper.get('label').attributes('for')).toBe('login-email')

        const input = wrapper.get('input')

        expect(input.attributes('id')).toBe('login-email')
        expect(input.attributes('type')).toBe('email')
        expect(input.attributes('autocomplete')).toBe('username')
    })

    it('emits updates without owning application state', async () => {
        const wrapper = mount(AppTextField, {
            props: {
                id: 'login-email',
                label: 'Email address',
                modelValue: '',
            },
        })

        await wrapper.get('input').setValue('person@example.com')

        expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['person@example.com'])
    })

    it('connects validation feedback to the input', () => {
        const wrapper = mount(AppTextField, {
            props: {
                id: 'login-email',
                label: 'Email address',
                modelValue: '',
                error: 'Enter a valid email address.',
            },
        })

        const input = wrapper.get('input')

        expect(input.attributes('aria-invalid')).toBe('true')
        expect(input.attributes('aria-describedby')).toBe('login-email-error')

        expect(wrapper.get('#login-email-error').text()).toBe('Enter a valid email address.')
    })

    it('supports native required and disabled states', () => {
        const wrapper = mount(AppTextField, {
            props: {
                id: 'login-password',
                label: 'Password',
                modelValue: '',
                type: 'password',
                required: true,
                disabled: true,
            },
        })

        const input = wrapper.get('input')

        expect(input.attributes('required')).toBeDefined()
        expect(input.attributes('disabled')).toBeDefined()
        expect(input.attributes('type')).toBe('password')
    })
})
