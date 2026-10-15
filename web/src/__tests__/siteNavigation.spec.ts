import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import App from '../App.vue'
import { getCurrentUser, loginUser, logoutUser, type CurrentUserResult } from '../api/auth'
import SiteHeader from '../components/SiteHeader.vue'
import { createAppRouter } from '../router'
import { useSessionStore } from '../stores/session'

vi.mock('../api/auth', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/auth')>()

    return {
        ...original,
        getCurrentUser: vi.fn<typeof getCurrentUser>(),
        loginUser: vi.fn<typeof loginUser>(),
        logoutUser: vi.fn<typeof logoutUser>(),
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

function deferred<T>() {
    let resolve!: (value: T) => void

    const promise = new Promise<T>((resolvePromise) => {
        resolve = resolvePromise
    })

    return { promise, resolve }
}

describe('shared site navigation', () => {
    let pinia: Pinia

    beforeEach(() => {
        vi.resetAllMocks()
        pinia = createPinia()
        setActivePinia(pinia)
    })

    it('keeps the public landing page available for anonymous visitors', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })

        const router = createAppRouter(createMemoryHistory())
        await router.push('/')

        const wrapper = mount(App, {
            global: { plugins: [pinia, router] },
        })

        const header = wrapper.getComponent(SiteHeader)

        await vi.waitFor(() => {
            expect(useSessionStore().status).toBe('unauthenticated')
            expect(wrapper.get('main').text()).toContain(
                'Engineering projects, with revision history.',
            )
        })

        expect(router.currentRoute.value.path).toBe('/')
        expect(header.get('nav').attributes('aria-label')).toBe('Main navigation')
        expect(header.get('a[href="/"]').text()).toBe('3Default')
        expect(header.findAll('a[href="/login"]')).toHaveLength(1)
        expect(header.find('a[href="/projects"]').exists()).toBe(false)
        expect(header.find('button').exists()).toBe(false)
        expect(getCurrentUser).toHaveBeenCalledTimes(1)

        wrapper.unmount()
    })

    it('does not show incorrect authentication actions while restoring', async () => {
        const pending = deferred<CurrentUserResult>()

        vi.mocked(getCurrentUser).mockReturnValue(pending.promise)

        const router = createAppRouter(createMemoryHistory())
        await router.push('/')

        const wrapper = mount(App, {
            global: { plugins: [pinia, router] },
        })

        const header = wrapper.getComponent(SiteHeader)

        await vi.waitFor(() => {
            expect(getCurrentUser).toHaveBeenCalledTimes(1)
            expect(useSessionStore().status).toBe('checking')
        })

        await vi.waitFor(() => {
            expect(wrapper.get('main').text()).toContain(
                'Engineering projects, with revision history.',
            )
        })

        expect(header.find('a[href="/login"]').exists()).toBe(false)
        expect(header.find('a[href="/projects"]').exists()).toBe(false)
        expect(header.find('button').exists()).toBe(false)

        pending.resolve({
            status: 'authenticated',
            user,
        })

        await vi.waitFor(() => {
            expect(useSessionStore().status).toBe('authenticated')
            expect(header.get('a[href="/projects"]').text()).toBe('My Projects')
            expect(header.get('button').text()).toContain('Sign out')
        })

        expect(header.text()).toContain('Person')
        expect(header.find('a[href="/login"]').exists()).toBe(false)
        expect(router.currentRoute.value.path).toBe('/')
        expect(getCurrentUser).toHaveBeenCalledTimes(1)

        wrapper.unmount()
    })

    it('keeps the public landing page usable after session verification fails', async () => {
        vi.mocked(getCurrentUser).mockRejectedValue(new TypeError('Failed to fetch'))

        const router = createAppRouter(createMemoryHistory())
        await router.push('/')

        const wrapper = mount(App, {
            global: { plugins: [pinia, router] },
        })

        const header = wrapper.getComponent(SiteHeader)

        await vi.waitFor(() => {
            expect(useSessionStore().status).toBe('error')
            expect(wrapper.get('main').text()).toContain(
                'Engineering projects, with revision history.',
            )
        })

        expect(router.currentRoute.value.path).toBe('/')
        expect(header.get('a[href="/login"]').text()).toBe('Sign in')
        expect(header.find('a[href="/projects"]').exists()).toBe(false)
        expect(header.find('button').exists()).toBe(false)
        expect(useSessionStore().user).toBeNull()

        wrapper.unmount()
    })

    it('shows a neutral header before session verification begins', async () => {
        const router = createAppRouter(createMemoryHistory())
        await router.push('/')

        const wrapper = mount(SiteHeader, {
            global: { plugins: [pinia, router] },
        })

        expect(useSessionStore().status).toBe('unknown')
        expect(wrapper.get('a[href="/"]').text()).toBe('3Default')
        expect(wrapper.find('a[href="/login"]').exists()).toBe(false)
        expect(wrapper.find('a[href="/projects"]').exists()).toBe(false)
        expect(wrapper.find('button').exists()).toBe(false)
        expect(getCurrentUser).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('signs out through the shared header and returns to the landing page', async () => {
        vi.mocked(loginUser).mockResolvedValue(user)
        vi.mocked(logoutUser).mockResolvedValue(undefined)

        const session = useSessionStore()
        await session.login(credentials)

        const router = createAppRouter(createMemoryHistory())
        await router.push('/projects')

        const wrapper = mount(SiteHeader, {
            global: { plugins: [pinia, router] },
        })

        expect(wrapper.get('a[href="/projects"]').text()).toBe('My Projects')
        expect(wrapper.text()).toContain('Person')

        await wrapper.get('button').trigger('click')

        await vi.waitFor(() => {
            expect(logoutUser).toHaveBeenCalledTimes(1)
            expect(session.status).toBe('unauthenticated')
            expect(router.currentRoute.value.name).toBe('landing')
        })

        expect(session.user).toBeNull()
        expect(wrapper.get('a[href="/login"]').text()).toBe('Sign in')
        expect(wrapper.find('a[href="/projects"]').exists()).toBe(false)
        expect(wrapper.find('button').exists()).toBe(false)

        wrapper.unmount()
    })

    it('preserves the authenticated session when shared sign-out fails', async () => {
        vi.mocked(loginUser).mockResolvedValue(user)
        vi.mocked(logoutUser).mockRejectedValue(new TypeError('Failed to fetch'))

        const session = useSessionStore()
        await session.login(credentials)

        const router = createAppRouter(createMemoryHistory())
        await router.push('/projects')

        const wrapper = mount(SiteHeader, {
            global: { plugins: [pinia, router] },
        })

        await wrapper.get('button').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toBe(
                'Sign out is temporarily unavailable. Please try again.',
            )
        })

        expect(logoutUser).toHaveBeenCalledTimes(1)
        expect(session.status).toBe('authenticated')
        expect(session.user).toEqual(user)
        expect(router.currentRoute.value.path).toBe('/projects')
        expect(wrapper.get('a[href="/projects"]').text()).toBe('My Projects')
        expect(wrapper.get('button').text()).toContain('Sign out')
        expect(wrapper.find('a[href="/login"]').exists()).toBe(false)

        wrapper.unmount()
    })
})
