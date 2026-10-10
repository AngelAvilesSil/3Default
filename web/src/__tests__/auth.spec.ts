import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
    AuthenticationRequestError,
    AuthenticationResponseError,
    getCurrentUser,
    loginUser,
    logoutUser,
} from '../api/auth'

const fetchMock = vi.fn<typeof fetch>()

const user = {
    id: '11111111-1111-4111-8111-111111111111',
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-09-11T12:00:00Z',
    updatedAt: '2026-09-11T12:01:00Z',
}

function jsonResponse(body: unknown, status = 200): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: {
            'Content-Type': 'application/json',
        },
    })
}

describe('authentication API client', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        vi.stubGlobal('fetch', fetchMock)
    })

    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('logs in using the existing cookie-based API', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(user))

        const result = await loginUser({
            email: 'person@example.com',
            password: 'submitted password',
        })

        expect(result).toEqual(user)

        expect(fetchMock).toHaveBeenCalledWith('/api/auth/login', {
            method: 'POST',
            credentials: 'same-origin',
            headers: {
                Accept: 'application/json',
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                email: 'person@example.com',
                password: 'submitted password',
            }),
            signal: undefined,
        })
    })

    it.each([
        [401, 'Unauthorized'],
        [429, 'Too Many Requests'],
        [500, 'Internal Server Error'],
    ])('preserves HTTP %i login failures', async (status, statusText) => {
        fetchMock.mockResolvedValueOnce(new Response(null, { status, statusText }))

        const request = loginUser({
            email: 'person@example.com',
            password: 'wrong password',
        })

        await expect(request).rejects.toMatchObject({
            name: 'AuthenticationRequestError',
            status,
            statusText,
        })
        await expect(request).rejects.toBeInstanceOf(AuthenticationRequestError)
    })

    it('restores an existing authenticated session', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(user))

        await expect(getCurrentUser()).resolves.toEqual({
            status: 'authenticated',
            user,
        })

        expect(fetchMock).toHaveBeenCalledWith('/api/auth/me', {
            method: 'GET',
            credentials: 'same-origin',
            headers: {
                Accept: 'application/json',
            },
            signal: undefined,
        })
    })

    it('treats current-user HTTP 401 as unauthenticated', async () => {
        fetchMock.mockResolvedValueOnce(
            new Response(null, {
                status: 401,
                statusText: 'Unauthorized',
            }),
        )

        await expect(getCurrentUser()).resolves.toEqual({
            status: 'unauthenticated',
        })
    })

    it('does not treat a current-user server failure as logout', async () => {
        fetchMock.mockResolvedValueOnce(
            new Response(null, {
                status: 500,
                statusText: 'Internal Server Error',
            }),
        )

        await expect(getCurrentUser()).rejects.toMatchObject({
            name: 'AuthenticationRequestError',
            status: 500,
        })
    })

    it('rejects malformed successful user responses', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse({ email: 'person@example.com' }))

        await expect(getCurrentUser()).rejects.toBeInstanceOf(AuthenticationResponseError)
    })

    it('logs out without expecting a response body', async () => {
        fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))

        await expect(logoutUser()).resolves.toBeUndefined()

        expect(fetchMock).toHaveBeenCalledWith('/api/auth/logout', {
            method: 'POST',
            credentials: 'same-origin',
            headers: {
                Accept: 'application/json',
            },
            signal: undefined,
        })
    })

    it('rejects unsuccessful logout requests', async () => {
        fetchMock.mockResolvedValueOnce(
            new Response(null, {
                status: 500,
                statusText: 'Internal Server Error',
            }),
        )

        await expect(logoutUser()).rejects.toMatchObject({
            name: 'AuthenticationRequestError',
            status: 500,
        })
    })

    it('passes request cancellation to fetch', async () => {
        const controller = new AbortController()

        fetchMock.mockResolvedValueOnce(new Response(null, { status: 401 }))

        await getCurrentUser(controller.signal)

        expect(fetchMock).toHaveBeenCalledWith(
            '/api/auth/me',
            expect.objectContaining({
                signal: controller.signal,
            }),
        )
    })

    it('propagates network failures without changing their meaning', async () => {
        const failure = new TypeError('Failed to fetch')
        fetchMock.mockRejectedValueOnce(failure)

        await expect(getCurrentUser()).rejects.toBe(failure)
    })
})
