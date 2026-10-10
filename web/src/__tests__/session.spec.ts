import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
    AuthenticationRequestError,
    getCurrentUser,
    loginUser,
    logoutUser,
    type CurrentUserResult,
} from '../api/auth'
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

const credentials = {
    email: 'person@example.com',
    password: 'submitted password',
}

function deferred<T>() {
    let resolve!: (value: T) => void
    let reject!: (reason?: unknown) => void

    const promise = new Promise<T>((resolvePromise, rejectPromise) => {
        resolve = resolvePromise
        reject = rejectPromise
    })

    return { promise, resolve, reject }
}

describe('session store', () => {
    beforeEach(() => {
        vi.resetAllMocks()
        setActivePinia(createPinia())
    })

    it('starts without assuming authentication', () => {
        const store = useSessionStore()

        expect(store.status).toBe('unknown')
        expect(store.user).toBeNull()
        expect(getCurrentUser).not.toHaveBeenCalled()
    })

    it('restores an authenticated session', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user,
        })

        const store = useSessionStore()

        await store.restore()

        expect(store.status).toBe('authenticated')
        expect(store.user).toEqual(user)
        expect(getCurrentUser).toHaveBeenCalledTimes(1)
    })

    it('recognizes an absent or expired session', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })

        const store = useSessionStore()

        await store.restore()

        expect(store.status).toBe('unauthenticated')
        expect(store.user).toBeNull()
    })

    it('distinguishes server failure from unauthentication', async () => {
        const failure = new Error('backend unavailable')

        vi.mocked(getCurrentUser).mockRejectedValue(failure)

        const store = useSessionStore()

        await expect(store.restore()).rejects.toBe(failure)

        expect(store.status).toBe('error')
        expect(store.user).toBeNull()
    })

    it('allows retrying session restoration after an error', async () => {
        vi.mocked(getCurrentUser)
            .mockRejectedValueOnce(new Error('temporary failure'))
            .mockResolvedValueOnce({
                status: 'authenticated',
                user,
            })

        const store = useSessionStore()

        await expect(store.restore()).rejects.toThrow()

        await store.restore()

        expect(store.status).toBe('authenticated')
        expect(store.user).toEqual(user)
        expect(getCurrentUser).toHaveBeenCalledTimes(2)
    })

    it('deduplicates simultaneous restoration requests', async () => {
        const pending = deferred<CurrentUserResult>()

        vi.mocked(getCurrentUser).mockReturnValue(pending.promise)

        const store = useSessionStore()

        const first = store.restore()
        const second = store.restore()

        expect(store.status).toBe('checking')
        expect(getCurrentUser).toHaveBeenCalledTimes(1)

        pending.resolve({
            status: 'authenticated',
            user,
        })

        await Promise.all([first, second])

        expect(store.status).toBe('authenticated')
        expect(store.user).toEqual(user)
    })

    it('does not recheck an already resolved session', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user,
        })

        const store = useSessionStore()

        await store.restore()
        await store.restore()

        expect(getCurrentUser).toHaveBeenCalledTimes(1)
    })

    it('updates the authenticated user after successful login', async () => {
        vi.mocked(loginUser).mockResolvedValue(user)

        const store = useSessionStore()

        await store.login(credentials)

        expect(loginUser).toHaveBeenCalledWith(credentials)
        expect(store.status).toBe('authenticated')
        expect(store.user).toEqual(user)
    })

    it('does not authenticate after failed login', async () => {
        const failure = new AuthenticationRequestError(401, 'Unauthorized')

        vi.mocked(loginUser).mockRejectedValue(failure)

        const store = useSessionStore()

        await expect(store.login(credentials)).rejects.toBe(failure)

        expect(store.status).toBe('unauthenticated')
        expect(store.user).toBeNull()
    })

    it('ignores stale restoration after successful login', async () => {
        const pending = deferred<CurrentUserResult>()

        vi.mocked(getCurrentUser).mockReturnValue(pending.promise)
        vi.mocked(loginUser).mockResolvedValue(user)

        const store = useSessionStore()

        const restoration = store.restore()

        const signal = vi.mocked(getCurrentUser).mock.calls[0]?.[0]

        await store.login(credentials)

        expect(signal?.aborted).toBe(true)
        expect(store.status).toBe('authenticated')

        pending.resolve({
            status: 'unauthenticated',
        })

        await restoration

        expect(store.status).toBe('authenticated')
        expect(store.user).toEqual(user)
    })

    it('logs out only after successful server revocation', async () => {
        vi.mocked(loginUser).mockResolvedValue(user)
        vi.mocked(logoutUser).mockResolvedValue(undefined)

        const store = useSessionStore()

        await store.login(credentials)
        await store.logout()

        expect(logoutUser).toHaveBeenCalledTimes(1)
        expect(store.status).toBe('unauthenticated')
        expect(store.user).toBeNull()
    })

    it('preserves authenticated state if logout fails', async () => {
        const failure = new Error('revocation failed')

        vi.mocked(loginUser).mockResolvedValue(user)
        vi.mocked(logoutUser).mockRejectedValue(failure)

        const store = useSessionStore()

        await store.login(credentials)

        await expect(store.logout()).rejects.toBe(failure)

        expect(store.status).toBe('authenticated')
        expect(store.user).toEqual(user)
    })

    it('ignores stale restoration failure after logout', async () => {
        const pending = deferred<CurrentUserResult>()

        vi.mocked(getCurrentUser).mockReturnValue(pending.promise)
        vi.mocked(logoutUser).mockResolvedValue(undefined)

        const store = useSessionStore()

        const restoration = store.restore()

        await store.logout()

        pending.reject(new Error('stale response'))

        await expect(restoration).resolves.toBeUndefined()

        expect(store.status).toBe('unauthenticated')
        expect(store.user).toBeNull()
    })

    it('invalidates authentication after an explicit session rejection', async () => {
        vi.mocked(loginUser).mockResolvedValue(user)

        const store = useSessionStore()

        await store.login(credentials)

        store.invalidate()

        expect(store.status).toBe('unauthenticated')
        expect(store.user).toBeNull()
        expect(logoutUser).not.toHaveBeenCalled()
    })

    it('prevents pending restoration from undoing invalidation', async () => {
        const pending = deferred<CurrentUserResult>()

        vi.mocked(getCurrentUser).mockReturnValue(pending.promise)

        const store = useSessionStore()

        const restoration = store.restore()

        store.invalidate()

        pending.resolve({
            status: 'authenticated',
            user,
        })

        await restoration

        expect(store.status).toBe('unauthenticated')
        expect(store.user).toBeNull()
    })

    it('queues logout behind a pending login', async () => {
        const pendingLogin = deferred<typeof user>()

        vi.mocked(loginUser).mockReturnValue(pendingLogin.promise)
        vi.mocked(logoutUser).mockResolvedValue(undefined)

        const store = useSessionStore()

        const loginRequest = store.login(credentials)
        const logoutRequest = store.logout()

        const earlyLogoutCalls = vi.mocked(logoutUser).mock.calls.length

        pendingLogin.resolve(user)

        await Promise.all([loginRequest, logoutRequest])

        expect(earlyLogoutCalls).toBe(0)
        expect(logoutUser).toHaveBeenCalledTimes(1)
        expect(store.status).toBe('unauthenticated')
        expect(store.user).toBeNull()
    })

    it('queues login behind a pending logout', async () => {
        const pendingLogout = deferred<void>()

        vi.mocked(loginUser).mockResolvedValue(user)

        const store = useSessionStore()

        await store.login(credentials)

        vi.mocked(logoutUser).mockReturnValue(pendingLogout.promise)

        const logoutRequest = store.logout()
        const loginRequest = store.login(credentials)

        const loginCallsBeforeLogoutFinished = vi.mocked(loginUser).mock.calls.length

        pendingLogout.resolve(undefined)

        await Promise.all([logoutRequest, loginRequest])

        expect(loginCallsBeforeLogoutFinished).toBe(1)
        expect(loginUser).toHaveBeenCalledTimes(2)
        expect(store.status).toBe('authenticated')
        expect(store.user).toEqual(user)
    })

    it('does not restore while login is pending', async () => {
        const pendingLogin = deferred<typeof user>()

        vi.mocked(loginUser).mockReturnValue(pendingLogin.promise)
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })

        const store = useSessionStore()

        const loginRequest = store.login(credentials)
        const restoration = store.restore()

        const earlySessionChecks = vi.mocked(getCurrentUser).mock.calls.length

        pendingLogin.resolve(user)

        await Promise.all([loginRequest, restoration])

        expect(earlySessionChecks).toBe(0)
        expect(getCurrentUser).not.toHaveBeenCalled()
        expect(store.status).toBe('authenticated')
        expect(store.user).toEqual(user)
    })

    it('distinguishes login network failures from invalid credentials', async () => {
        const failure = new TypeError('Failed to fetch')

        vi.mocked(loginUser).mockRejectedValue(failure)

        const store = useSessionStore()

        await expect(store.login(credentials)).rejects.toBe(failure)

        expect(store.status).toBe('error')
        expect(store.user).toBeNull()
    })

    it('executes a queued login after logout fails', async () => {
        const nextUser = {
            ...user,
            id: '22222222-2222-4222-8222-222222222222',
            email: 'other@example.com',
            displayName: 'Other',
        }

        const nextCredentials = {
            email: 'other@example.com',
            password: 'other password',
        }

        vi.mocked(loginUser).mockResolvedValueOnce(user).mockResolvedValueOnce(nextUser)

        const store = useSessionStore()

        await store.login(credentials)

        const pendingLogout = deferred<void>()
        const failure = new Error('logout unavailable')

        vi.mocked(logoutUser).mockReturnValue(pendingLogout.promise)

        const logoutRequest = store.logout()
        const loginRequest = store.login(nextCredentials)

        await vi.waitFor(() => {
            expect(logoutUser).toHaveBeenCalledTimes(1)
        })

        expect(loginUser).toHaveBeenCalledTimes(1)

        pendingLogout.reject(failure)

        await expect(logoutRequest).rejects.toBe(failure)
        await loginRequest

        expect(loginUser).toHaveBeenCalledTimes(2)
        expect(loginUser).toHaveBeenLastCalledWith(nextCredentials)
        expect(store.status).toBe('authenticated')
        expect(store.user).toEqual(nextUser)
    })

    it('applies consecutive successful logins in execution order', async () => {
        const pendingFirstLogin = deferred<typeof user>()

        const nextUser = {
            ...user,
            id: '33333333-3333-4333-8333-333333333333',
            email: 'second@example.com',
            displayName: 'Second',
        }

        const nextCredentials = {
            email: 'second@example.com',
            password: 'second password',
        }

        vi.mocked(loginUser)
            .mockReturnValueOnce(pendingFirstLogin.promise)
            .mockResolvedValueOnce(nextUser)

        const store = useSessionStore()

        const firstLogin = store.login(credentials)
        const secondLogin = store.login(nextCredentials)

        await vi.waitFor(() => {
            expect(loginUser).toHaveBeenCalledTimes(1)
        })

        pendingFirstLogin.resolve(user)

        await Promise.all([firstLogin, secondLogin])

        expect(loginUser).toHaveBeenCalledTimes(2)
        expect(loginUser).toHaveBeenNthCalledWith(1, credentials)
        expect(loginUser).toHaveBeenNthCalledWith(2, nextCredentials)

        expect(store.status).toBe('authenticated')
        expect(store.user).toEqual(nextUser)
    })
})
