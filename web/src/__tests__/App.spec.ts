import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { describe, expect, it, vi } from 'vitest'

import App from '../App.vue'
import { getCurrentUser, type CurrentUserResult } from '../api/auth'
import SiteHeader from '../components/SiteHeader.vue'
import { createAppRouter } from '../router'
import { useSessionStore } from '../stores/session'

vi.mock('../api/auth', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/auth')>()

    return {
        ...original,
        getCurrentUser: vi.fn<typeof getCurrentUser>(),
    }
})

describe('App', () => {
    it('renders the shared header and router outlet while restoring the session', async () => {
        const pinia = createPinia()
        setActivePinia(pinia)

        const router = createAppRouter(createMemoryHistory())
        await router.push('/')

        let finishRestoration!: (result: CurrentUserResult) => void

        const pendingRestoration = new Promise<CurrentUserResult>((resolve) => {
            finishRestoration = resolve
        })

        vi.mocked(getCurrentUser).mockReturnValue(pendingRestoration)

        const wrapper = mount(App, {
            global: {
                plugins: [pinia, router],
                stubs: {
                    RouterView: {
                        template: '<div data-testid="router-view"></div>',
                    },
                },
            },
        })

        const header = wrapper.getComponent(SiteHeader)

        expect(header.get('a[href="/"]').text()).toBe('3Default')
        expect(wrapper.get('main').get('[data-testid="router-view"]').exists()).toBe(true)
        expect(useSessionStore().status).toBe('checking')
        expect(router.currentRoute.value.path).toBe('/')

        finishRestoration({ status: 'unauthenticated' })

        await vi.waitFor(() => {
            expect(useSessionStore().status).toBe('unauthenticated')
            expect(header.get('a[href="/login"]').text()).toBe('Sign in')
        })

        expect(getCurrentUser).toHaveBeenCalledTimes(1)

        wrapper.unmount()
    })
})
