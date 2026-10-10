import { defineStore } from 'pinia'
import { ref } from 'vue'

import {
    AuthenticationRequestError,
    getCurrentUser,
    loginUser,
    logoutUser,
    type AuthenticatedUser,
    type LoginCredentials,
} from '../api/auth'

export type SessionStatus = 'unknown' | 'checking' | 'authenticated' | 'unauthenticated' | 'error'

export const useSessionStore = defineStore('session', () => {
    const status = ref<SessionStatus>('unknown')
    const user = ref<AuthenticatedUser | null>(null)

    let generation = 0
    let pendingRestoration: Promise<void> | null = null
    let restorationController: AbortController | null = null

    let mutationTail: Promise<void> = Promise.resolve()
    let pendingMutations = 0

    function supersedeRestoration(): void {
        generation += 1

        restorationController?.abort()
        restorationController = null
        pendingRestoration = null
    }

    function queueMutation(operation: () => Promise<void>): Promise<void> {
        supersedeRestoration()
        pendingMutations += 1

        const request = mutationTail.then(async () => {
            try {
                await operation()
            } finally {
                pendingMutations -= 1
            }
        })

        // A failed operation must not block later operations.
        mutationTail = request.then(
            () => undefined,
            () => undefined,
        )

        return request
    }

    function restore(): Promise<void> {
        // Never check the session while login or logout can
        // still change the browser's session cookie.
        if (pendingMutations > 0) {
            return mutationTail.then(() => restore())
        }

        if (pendingRestoration !== null) {
            return pendingRestoration
        }

        if (status.value === 'authenticated' || status.value === 'unauthenticated') {
            return Promise.resolve()
        }

        const requestGeneration = ++generation
        const controller = new AbortController()

        restorationController = controller

        status.value = 'checking'
        user.value = null

        const request = (async () => {
            try {
                const result = await getCurrentUser(controller.signal)

                if (requestGeneration !== generation) {
                    return
                }

                if (result.status === 'authenticated') {
                    user.value = result.user
                    status.value = 'authenticated'
                } else {
                    user.value = null
                    status.value = 'unauthenticated'
                }
            } catch (error) {
                if (requestGeneration !== generation) {
                    return
                }

                user.value = null
                status.value = 'error'

                throw error
            } finally {
                if (requestGeneration === generation) {
                    restorationController = null
                    pendingRestoration = null
                }
            }
        })()

        pendingRestoration = request

        return request
    }

    function login(credentials: LoginCredentials): Promise<void> {
        return queueMutation(async () => {
            try {
                const authenticatedUser = await loginUser(credentials)

                // Mutations execute sequentially, so apply
                // successful results in their execution order.
                user.value = authenticatedUser
                status.value = 'authenticated'
            } catch (error) {
                // A failed attempt to switch accounts does
                // not revoke an existing authenticated session.
                if (status.value === 'authenticated' && user.value !== null) {
                    throw error
                }

                user.value = null

                if (error instanceof AuthenticationRequestError && error.status === 401) {
                    status.value = 'unauthenticated'
                } else {
                    status.value = 'error'
                }

                throw error
            }
        })
    }

    function logout(): Promise<void> {
        return queueMutation(async () => {
            try {
                await logoutUser()

                user.value = null
                status.value = 'unauthenticated'
            } catch (error) {
                // Do not claim that a failed logout revoked
                // a session that was previously authenticated.
                if (status.value !== 'authenticated' && status.value !== 'unauthenticated') {
                    user.value = null
                    status.value = 'error'
                }

                throw error
            }
        })
    }

    function invalidate(): void {
        supersedeRestoration()

        user.value = null
        status.value = 'unauthenticated'
    }

    return {
        status,
        user,
        restore,
        login,
        logout,
        invalidate,
    }
})
