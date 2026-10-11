import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser, loginUser, type CurrentUserResult } from '../api/auth'
import { createAppRouter } from '../router'
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

describe('authentication navigation', () => {
    beforeEach(() => {
        vi.resetAllMocks()
        setActivePinia(createPinia())
    })

    it('keeps the root page public without forcing authentication', async () => {
        const router = createAppRouter(createMemoryHistory())

        await router.push('/')

        expect(router.currentRoute.value.path).toBe('/')
        expect(getCurrentUser).not.toHaveBeenCalled()
    })

    it('allows unauthenticated users to reach the login page', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })

        const router = createAppRouter(createMemoryHistory())

        await router.push('/login')

        expect(router.currentRoute.value.name).toBe('login')
        expect(getCurrentUser).toHaveBeenCalledTimes(1)
    })

    it('redirects an unauthenticated user from the app home', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })

        const router = createAppRouter(createMemoryHistory())

        await router.push('/app')

        expect(router.currentRoute.value.name).toBe('login')
        expect(router.currentRoute.value.query.redirect).toBe('/app')
        expect(useSessionStore().status).toBe('unauthenticated')
    })

    it('preserves the requested preview URL and query parameters', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })

        const router = createAppRouter(createMemoryHistory())

        await router.push('/projects/project-123/files/file-456/preview?camera=front')

        expect(router.currentRoute.value.name).toBe('login')
        expect(router.currentRoute.value.query.redirect).toBe(
            '/projects/project-123/files/file-456/preview?camera=front',
        )
    })

    it('allows a restored authenticated session into the preview', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user,
        })

        const router = createAppRouter(createMemoryHistory())

        await router.push('/projects/project-123/files/file-456/preview')

        expect(router.currentRoute.value.name).toBe('project-file-preview')
        expect(router.currentRoute.value.params.projectId).toBe('project-123')
        expect(router.currentRoute.value.params.projectFileId).toBe('file-456')

        expect(useSessionStore().user).toEqual(user)
    })

    it('does not repeat a resolved session check on later navigation', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user,
        })

        const router = createAppRouter(createMemoryHistory())

        await router.push('/app')
        await router.push('/projects/project-123/files/file-456/preview')

        expect(getCurrentUser).toHaveBeenCalledTimes(1)
        expect(router.currentRoute.value.name).toBe('project-file-preview')
    })

    it('waits for session restoration before deciding access', async () => {
        const pending = deferred<CurrentUserResult>()

        vi.mocked(getCurrentUser).mockReturnValue(pending.promise)

        const router = createAppRouter(createMemoryHistory())

        const navigation = router.push('/app')

        // Allow the asynchronous guard to start.
        await vi.waitFor(() => {
            expect(getCurrentUser).toHaveBeenCalledTimes(1)
        })

        expect(useSessionStore().status).toBe('checking')

        pending.resolve({
            status: 'authenticated',
            user,
        })

        await navigation

        expect(router.currentRoute.value.name).toBe('app-home')
        expect(useSessionStore().status).toBe('authenticated')
    })

    it('shows a separate recoverable route when verification fails', async () => {
        vi.mocked(getCurrentUser).mockRejectedValue(new TypeError('Failed to fetch'))

        const router = createAppRouter(createMemoryHistory())

        await router.push('/app')

        expect(router.currentRoute.value.name).toBe('session-unavailable')
        expect(router.currentRoute.value.query.redirect).toBe('/app')
        expect(useSessionStore().status).toBe('error')
        expect(useSessionStore().user).toBeNull()
    })

    it('returns an authenticated visitor to a safe protected destination', async () => {
        vi.mocked(loginUser).mockResolvedValue(user)

        const session = useSessionStore()

        await session.login({
            email: 'person@example.com',
            password: 'my password',
        })

        const router = createAppRouter(createMemoryHistory())

        await router.push('/login?redirect=%2Fprojects%2Fp%2Ffiles%2Ff%2Fpreview')

        expect(router.currentRoute.value.name).toBe('project-file-preview')
        expect(router.currentRoute.value.params.projectId).toBe('p')
    })

    it('rejects external return destinations', async () => {
        vi.mocked(loginUser).mockResolvedValue(user)

        await useSessionStore().login({
            email: 'person@example.com',
            password: 'my password',
        })

        const router = createAppRouter(createMemoryHistory())

        await router.push('/login?redirect=%2F%2Fevil.example')

        expect(router.currentRoute.value.name).toBe('app-home')
        expect(router.currentRoute.value.path).toBe('/app')
    })
})
