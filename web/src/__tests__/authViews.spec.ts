import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser, loginUser, logoutUser } from '../api/auth'
import { safeReturnPath } from '../router/authRedirect'
import { createAppRouter } from '../router'
import { useSessionStore } from '../stores/session'
import AppHomeView from '../views/AppHomeView.vue'
import LoginView from '../views/LoginView.vue'
import SessionUnavailableView from '../views/SessionUnavailableView.vue'

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

const credentials = {
    email: 'person@example.com',
    password: 'my password',
}

describe('authentication views', () => {
    let pinia: Pinia

    beforeEach(() => {
        vi.resetAllMocks()
        pinia = createPinia()
        setActivePinia(pinia)
    })

    it('returns to the intended preview after successful login', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })
        vi.mocked(loginUser).mockResolvedValue(user)

        const router = createAppRouter(createMemoryHistory())

        await router.push('/login?redirect=%2Fprojects%2Fp%2Ffiles%2Ff%2Fpreview%3Fcamera%3Dfront')

        const wrapper = mount(LoginView, {
            global: { plugins: [pinia, router] },
        })

        await wrapper.get('#login-email').setValue(credentials.email)
        await wrapper.get('#login-password').setValue(credentials.password)
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(loginUser).toHaveBeenCalledWith(credentials)
        await vi.waitFor(
            () => {
                expect(router.currentRoute.value.name).toBe('project-file-preview')
            },
            { timeout: 3000 },
        )
        expect(router.currentRoute.value.fullPath).toBe('/projects/p/files/f/preview?camera=front')
        expect(useSessionStore().user).toEqual(user)

        wrapper.unmount()
    })

    it('uses the authenticated home when login has no return URL', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })
        vi.mocked(loginUser).mockResolvedValue(user)

        const router = createAppRouter(createMemoryHistory())
        await router.push('/login')

        const wrapper = mount(LoginView, {
            global: { plugins: [pinia, router] },
        })

        await wrapper.get('#login-email').setValue(credentials.email)
        await wrapper.get('#login-password').setValue(credentials.password)
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(router.currentRoute.value.name).toBe('app-home')
        expect(router.currentRoute.value.path).toBe('/app')

        wrapper.unmount()
    })

    it('recovers from a failed session check when retry succeeds', async () => {
        vi.mocked(getCurrentUser)
            .mockRejectedValueOnce(new TypeError('Failed to fetch'))
            .mockResolvedValueOnce({
                status: 'authenticated',
                user,
            })

        const router = createAppRouter(createMemoryHistory())
        await router.push('/app')

        expect(router.currentRoute.value.name).toBe('session-unavailable')

        const wrapper = mount(SessionUnavailableView, {
            global: { plugins: [pinia, router] },
        })

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(getCurrentUser).toHaveBeenCalledTimes(2)
        expect(router.currentRoute.value.name).toBe('app-home')
        expect(useSessionStore().user).toEqual(user)

        wrapper.unmount()
    })

    it('redirects to login if retry confirms no session', async () => {
        vi.mocked(getCurrentUser)
            .mockRejectedValueOnce(new TypeError('Failed to fetch'))
            .mockResolvedValueOnce({
                status: 'unauthenticated',
            })

        const router = createAppRouter(createMemoryHistory())
        await router.push('/app')

        const wrapper = mount(SessionUnavailableView, {
            global: { plugins: [pinia, router] },
        })

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(router.currentRoute.value.name).toBe('login')
        expect(router.currentRoute.value.query.redirect).toBe('/app')
        expect(useSessionStore().status).toBe('unauthenticated')

        wrapper.unmount()
    })

    it('preserves recoverable feedback when retry fails again', async () => {
        vi.mocked(getCurrentUser).mockRejectedValue(new TypeError('Failed to fetch'))

        const router = createAppRouter(createMemoryHistory())
        await router.push('/app')

        const wrapper = mount(SessionUnavailableView, {
            global: { plugins: [pinia, router] },
        })

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(router.currentRoute.value.name).toBe('session-unavailable')
        expect(wrapper.get('[role="alert"]').text()).toBe(
            'Your session could not be verified. Please try again.',
        )
        expect(useSessionStore().status).toBe('error')

        wrapper.unmount()
    })

    it('signs out from the authenticated home', async () => {
        vi.mocked(loginUser).mockResolvedValue(user)
        vi.mocked(logoutUser).mockResolvedValue(undefined)

        await useSessionStore().login(credentials)

        const router = createAppRouter(createMemoryHistory())
        await router.push('/app')

        const wrapper = mount(AppHomeView, {
            global: { plugins: [pinia, router] },
        })

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(logoutUser).toHaveBeenCalledTimes(1)
        expect(useSessionStore().status).toBe('unauthenticated')
        expect(useSessionStore().user).toBeNull()
        expect(router.currentRoute.value.name).toBe('login')

        wrapper.unmount()
    })

    it('preserves the session when sign-out fails', async () => {
        vi.mocked(loginUser).mockResolvedValue(user)
        vi.mocked(logoutUser).mockRejectedValue(new TypeError('Failed to fetch'))

        await useSessionStore().login(credentials)

        const router = createAppRouter(createMemoryHistory())
        await router.push('/app')

        const wrapper = mount(AppHomeView, {
            global: { plugins: [pinia, router] },
        })

        await wrapper.get('button').trigger('click')
        await flushPromises()

        expect(router.currentRoute.value.name).toBe('app-home')
        expect(useSessionStore().status).toBe('authenticated')
        expect(useSessionStore().user).toEqual(user)
        expect(wrapper.get('[role="alert"]').text()).toBe(
            'Sign out is temporarily unavailable. Please try again.',
        )

        wrapper.unmount()
    })

    it('rejects unsafe and non-protected return destinations', () => {
        const router = createAppRouter(createMemoryHistory())

        const unsafePaths = [
            'https://evil.example',
            '//evil.example',
            '/\\evil.example',
            '/login',
            '/session-unavailable',
            '/',
            '/unknown',
            '/%2F%2Fevil.example',
        ]

        for (const path of unsafePaths) {
            expect(safeReturnPath(router, path)).toBe('/app')
        }

        expect(safeReturnPath(router, '/projects/p/files/f/preview?camera=front')).toBe(
            '/projects/p/files/f/preview?camera=front',
        )
    })
})
