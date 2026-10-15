import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser, loginUser } from '../api/auth'
import { createAppRouter } from '../router'
import { useSessionStore } from '../stores/session'
import SiteHeader from '../components/SiteHeader.vue'
import LoginView from '../views/LoginView.vue'

vi.mock('../api/auth', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/auth')>()

    return {
        ...original,
        getCurrentUser: vi.fn<typeof original.getCurrentUser>(),
        loginUser: vi.fn<typeof original.loginUser>(),
    }
})

const user = {
    id: '11111111-1111-4111-8111-111111111111',
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-09-11T12:00:00Z',
    updatedAt: '2026-09-11T12:01:00Z',
}

describe('My Projects navigation', () => {
    let pinia: Pinia

    beforeEach(() => {
        vi.resetAllMocks()
        pinia = createPinia()
        setActivePinia(pinia)
    })

    it('navigates from the shared header to My Projects', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user,
        })

        const router = createAppRouter(createMemoryHistory())
        await router.push('/')

        await useSessionStore().restore()

        expect(useSessionStore().status).toBe('authenticated')
        expect(getCurrentUser).toHaveBeenCalledTimes(1)

        const wrapper = mount(SiteHeader, {
            global: { plugins: [pinia, router] },
        })

        const projectsLink = wrapper.get('a[href="/projects"]')

        expect(projectsLink.text()).toBe('My Projects')

        await projectsLink.trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('my-projects')
        })

        wrapper.unmount()
    })

    it('registers My Projects as a protected route', () => {
        const router = createAppRouter(createMemoryHistory())
        const destination = router.resolve('/projects')

        expect(destination.name).toBe('my-projects')
        expect(destination.meta.requiresAuth).toBe(true)
    })

    it('redirects unauthenticated users to login with the return destination', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })

        const router = createAppRouter(createMemoryHistory())

        await router.push('/projects')

        expect(router.currentRoute.value.name).toBe('login')
        expect(router.currentRoute.value.query.redirect).toBe('/projects')
    })

    it('allows authenticated users to access My Projects', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user,
        })

        const router = createAppRouter(createMemoryHistory())

        await router.push('/projects')

        expect(router.currentRoute.value.name).toBe('my-projects')
        expect(router.currentRoute.value.path).toBe('/projects')
        expect(getCurrentUser).toHaveBeenCalledTimes(1)
    })

    it('returns to My Projects after successful login', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })
        vi.mocked(loginUser).mockResolvedValue(user)

        const router = createAppRouter(createMemoryHistory())

        await router.push('/projects')

        expect(router.currentRoute.value.name).toBe('login')
        expect(router.currentRoute.value.query.redirect).toBe('/projects')

        const wrapper = mount(LoginView, {
            global: { plugins: [pinia, router] },
        })

        await wrapper.get('#login-email').setValue(user.email)
        await wrapper.get('#login-password').setValue('my password')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(loginUser).toHaveBeenCalledWith({
            email: user.email,
            password: 'my password',
        })
        expect(router.currentRoute.value.name).toBe('my-projects')
        expect(router.currentRoute.value.path).toBe('/projects')

        wrapper.unmount()
    })

    it('registers Create Project as a protected route', () => {
        const router = createAppRouter(createMemoryHistory())
        const destination = router.resolve('/projects/new')

        expect(destination.name).toBe('create-project')
        expect(destination.meta.requiresAuth).toBe(true)
    })

    it('redirects unauthenticated Create Project visitors to login', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })

        const router = createAppRouter(createMemoryHistory())

        await router.push('/projects/new')

        expect(router.currentRoute.value.name).toBe('login')
        expect(router.currentRoute.value.query.redirect).toBe('/projects/new')
    })

    it('allows authenticated users to access Create Project', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user,
        })

        const router = createAppRouter(createMemoryHistory())

        await router.push('/projects/new')

        expect(router.currentRoute.value.name).toBe('create-project')
        expect(router.currentRoute.value.path).toBe('/projects/new')
        expect(getCurrentUser).toHaveBeenCalledTimes(1)
    })

    it('returns to Create Project after successful login', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })
        vi.mocked(loginUser).mockResolvedValue(user)

        const router = createAppRouter(createMemoryHistory())

        await router.push('/projects/new')

        expect(router.currentRoute.value.name).toBe('login')
        expect(router.currentRoute.value.query.redirect).toBe('/projects/new')

        const wrapper = mount(LoginView, {
            global: { plugins: [pinia, router] },
        })

        await wrapper.get('#login-email').setValue(user.email)
        await wrapper.get('#login-password').setValue('my password')
        await wrapper.get('form').trigger('submit')
        await flushPromises()

        expect(router.currentRoute.value.name).toBe('create-project')
        expect(router.currentRoute.value.path).toBe('/projects/new')

        wrapper.unmount()
    })
})
