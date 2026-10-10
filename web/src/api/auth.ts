export type AuthenticatedUser = {
    id: string
    email: string
    displayName: string
    createdAt: string
    updatedAt: string
}

export type LoginCredentials = {
    email: string
    password: string
}

export type CurrentUserResult =
    | {
          status: 'authenticated'
          user: AuthenticatedUser
      }
    | {
          status: 'unauthenticated'
      }

export class AuthenticationRequestError extends Error {
    readonly status: number
    readonly statusText: string

    constructor(status: number, statusText: string) {
        super(`Authentication request failed with HTTP ${status}`)

        this.name = 'AuthenticationRequestError'
        this.status = status
        this.statusText = statusText
    }
}

export class AuthenticationResponseError extends Error {
    constructor(message: string) {
        super(message)

        this.name = 'AuthenticationResponseError'
    }
}

function isAuthenticatedUser(value: unknown): value is AuthenticatedUser {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) {
        return false
    }

    const candidate = value as Record<string, unknown>

    return (
        typeof candidate.id === 'string' &&
        candidate.id.length > 0 &&
        typeof candidate.email === 'string' &&
        candidate.email.length > 0 &&
        typeof candidate.displayName === 'string' &&
        typeof candidate.createdAt === 'string' &&
        candidate.createdAt.length > 0 &&
        typeof candidate.updatedAt === 'string' &&
        candidate.updatedAt.length > 0
    )
}

async function readAuthenticatedUser(response: Response): Promise<AuthenticatedUser> {
    let body: unknown

    try {
        body = await response.json()
    } catch {
        throw new AuthenticationResponseError('Authentication response did not contain valid JSON')
    }

    if (!isAuthenticatedUser(body)) {
        throw new AuthenticationResponseError(
            'Authentication response did not contain a valid user',
        )
    }

    return body
}

function requireSuccessfulStatus(response: Response, expectedStatus: number): void {
    if (!response.ok) {
        throw new AuthenticationRequestError(response.status, response.statusText)
    }

    if (response.status !== expectedStatus) {
        throw new AuthenticationResponseError(
            `Unexpected authentication response status: ${response.status}`,
        )
    }
}

export async function loginUser(
    credentials: LoginCredentials,
    signal?: AbortSignal,
): Promise<AuthenticatedUser> {
    const response = await fetch('/api/auth/login', {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
            Accept: 'application/json',
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            email: credentials.email,
            password: credentials.password,
        }),
        signal,
    })

    requireSuccessfulStatus(response, 200)

    return readAuthenticatedUser(response)
}

export async function getCurrentUser(signal?: AbortSignal): Promise<CurrentUserResult> {
    const response = await fetch('/api/auth/me', {
        method: 'GET',
        credentials: 'same-origin',
        headers: {
            Accept: 'application/json',
        },
        signal,
    })

    if (response.status === 401) {
        return {
            status: 'unauthenticated',
        }
    }

    requireSuccessfulStatus(response, 200)

    return {
        status: 'authenticated',
        user: await readAuthenticatedUser(response),
    }
}

export async function logoutUser(signal?: AbortSignal): Promise<void> {
    const response = await fetch('/api/auth/logout', {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
            Accept: 'application/json',
        },
        signal,
    })

    requireSuccessfulStatus(response, 204)
}
