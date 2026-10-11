import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { AuthenticationRequestError, loginUser } from '../api/auth'
import LoginForm from '../components/auth/LoginForm.vue'
import AppButton from '../components/ui/AppButton.vue'
import AppTextField from '../components/ui/AppTextField.vue'
import { useSessionStore } from '../stores/session'

vi.mock('../api/auth', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/auth')>()

    return {
        ...original,
        getCurrentUser: vi.fn(),
        loginUser: vi.fn(),
        logoutUser: vi.fn(),
    }
})

const user = {
    id: '11111111-1111-4111-8111-111111111111',
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-09-11T12:00:00Z',
    updatedAt: '2026-09-11T12:01:00Z',
}

function deferred<T>() {
    let resolve!: (value: T) => void

    const promise = new Promise<T>((resolvePromise) => {
        resolve = resolvePromise
    })

    return { promise, resolve }
}

describe('LoginForm', () => {
    beforeEach(() => {
        vi.resetAllMocks()
        setActivePinia(createPinia())
    })

    it('renders accessible fields using shared UI components', () => {
        const wrapper = mount(LoginForm)

        expect(wrapper.text()).toContain('Sign in to 3Default')

        expect(wrapper.findAllComponents(AppTextField)).toHaveLength(2)
        expect(wrapper.findComponent(AppButton).exists()).toBe(true)

        const email = wrapper.get('input#login-email')
        const password = wrapper.get('input#login-password')

        expect(email.attributes('type')).toBe('email')
        expect(email.attributes('autocomplete')).toBe('username')
        expect(email.attributes('required')).toBeDefined()

        expect(password.attributes('type')).toBe('password')
        expect(password.attributes('autocomplete')).toBe('current-password')
        expect(password.attributes('required')).toBeDefined()

        expect(wrapper.get('button[type="submit"]').text()).toBe('Sign in')
    })

    it('authenticates and emits success without controlling navigation', async () => {
        vi.mocked(loginUser).mockResolvedValue(user)

        const wrapper = mount(LoginForm)

        await wrapper.get('#login-email').setValue(' person@example.com ')
        await wrapper.get('#login-password').setValue('my password')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(loginUser).toHaveBeenCalledOnce()
        expect(loginUser).toHaveBeenCalledWith({
            email: 'person@example.com',
            password: 'my password',
        })

        expect(useSessionStore().status).toBe('authenticated')
        expect(wrapper.emitted('authenticated')).toHaveLength(1)
        expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    })

    it('shows an appropriate message for rejected credentials', async () => {
        vi.mocked(loginUser).mockRejectedValue(new AuthenticationRequestError(401, 'Unauthorized'))

        const wrapper = mount(LoginForm)

        await wrapper.get('#login-email').setValue('person@example.com')
        await wrapper.get('#login-password').setValue('wrong password')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(wrapper.get('[role="alert"]').text()).toBe('Incorrect email or password.')
        expect(wrapper.emitted('authenticated')).toBeUndefined()
    })

    it('explains rate limiting without exposing backend details', async () => {
        vi.mocked(loginUser).mockRejectedValue(
            new AuthenticationRequestError(429, 'Too Many Requests'),
        )

        const wrapper = mount(LoginForm)

        await wrapper.get('#login-email').setValue('person@example.com')
        await wrapper.get('#login-password').setValue('my password')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(wrapper.get('[role="alert"]').text()).toBe(
            'Too many sign-in attempts. Please try again later.',
        )
    })

    it('shows a recoverable message for an unexpected request failure', async () => {
        vi.mocked(loginUser).mockRejectedValue(new TypeError('Failed to fetch'))

        const wrapper = mount(LoginForm)

        await wrapper.get('#login-email').setValue('person@example.com')
        await wrapper.get('#login-password').setValue('my password')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(wrapper.get('[role="alert"]').text()).toBe(
            'Sign in is temporarily unavailable. Please try again.',
        )
        expect(wrapper.emitted('authenticated')).toBeUndefined()
    })

    it('disables submission and prevents duplicates while loading', async () => {
        const pending = deferred<typeof user>()

        vi.mocked(loginUser).mockReturnValue(pending.promise)

        const wrapper = mount(LoginForm)

        await wrapper.get('#login-email').setValue('person@example.com')
        await wrapper.get('#login-password').setValue('my password')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        const button = wrapper.get('button[type="submit"]')

        expect(button.attributes('disabled')).toBeDefined()
        expect(button.attributes('aria-busy')).toBe('true')
        expect(wrapper.get('#login-email').attributes('disabled')).toBeDefined()
        expect(wrapper.get('#login-password').attributes('disabled')).toBeDefined()

        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(loginUser).toHaveBeenCalledTimes(1)

        pending.resolve(user)
        await flushPromises()

        expect(button.attributes('disabled')).toBeUndefined()
        expect(wrapper.emitted('authenticated')).toHaveLength(1)
    })

    it('clears a previous failure when the user retries', async () => {
        vi.mocked(loginUser)
            .mockRejectedValueOnce(new TypeError('Failed to fetch'))
            .mockResolvedValueOnce(user)

        const wrapper = mount(LoginForm)

        await wrapper.get('#login-email').setValue('person@example.com')
        await wrapper.get('#login-password').setValue('my password')

        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(wrapper.find('[role="alert"]').exists()).toBe(true)

        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(loginUser).toHaveBeenCalledTimes(2)
        expect(wrapper.find('[role="alert"]').exists()).toBe(false)
        expect(wrapper.emitted('authenticated')).toHaveLength(1)
    })
})
