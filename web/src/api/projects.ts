export type Project = {
    id: string
    name: string
    description?: string | null
    visibility: 'private' | 'public'
    createdAt: string
    updatedAt: string
}

export type CreateProjectInput = {
    name: string
    description?: string
}

export class ProjectRequestError extends Error {
    readonly status: number
    readonly statusText: string

    constructor(status: number, statusText: string) {
        super(`Project request failed with HTTP ${status}`)

        this.name = 'ProjectRequestError'
        this.status = status
        this.statusText = statusText
    }
}

export class ProjectResponseError extends Error {
    constructor(message: string) {
        super(message)

        this.name = 'ProjectResponseError'
    }
}

function isProject(value: unknown): value is Project {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) {
        return false
    }

    const candidate = value as Record<string, unknown>

    return (
        typeof candidate.id === 'string' &&
        candidate.id.length > 0 &&
        typeof candidate.name === 'string' &&
        candidate.name.trim().length > 0 &&
        (candidate.description === undefined ||
            candidate.description === null ||
            typeof candidate.description === 'string') &&
        (candidate.visibility === 'private' || candidate.visibility === 'public') &&
        typeof candidate.createdAt === 'string' &&
        candidate.createdAt.length > 0 &&
        typeof candidate.updatedAt === 'string' &&
        candidate.updatedAt.length > 0
    )
}

async function readProjects(response: Response): Promise<Project[]> {
    let body: unknown

    try {
        body = await response.json()
    } catch {
        throw new ProjectResponseError('Project response did not contain valid JSON')
    }

    if (!Array.isArray(body) || !body.every(isProject)) {
        throw new ProjectResponseError('Project response did not contain a valid project list')
    }

    return body
}

async function readProject(response: Response): Promise<Project> {
    let body: unknown

    try {
        body = await response.json()
    } catch {
        throw new ProjectResponseError('Project response did not contain valid JSON')
    }

    if (!isProject(body)) {
        throw new ProjectResponseError('Project response did not contain a valid project')
    }

    return body
}

export async function listProjects(signal?: AbortSignal): Promise<Project[]> {
    const response = await fetch('/api/projects', {
        method: 'GET',
        credentials: 'same-origin',
        headers: {
            Accept: 'application/json',
        },
        signal,
    })

    if (!response.ok) {
        throw new ProjectRequestError(response.status, response.statusText)
    }

    if (response.status !== 200) {
        throw new ProjectResponseError(`Unexpected project response status: ${response.status}`)
    }

    return readProjects(response)
}

export async function createProject(
    input: CreateProjectInput,
    signal?: AbortSignal,
): Promise<Project> {
    const response = await fetch('/api/projects', {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
            Accept: 'application/json',
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            name: input.name,
            description: input.description,
        }),
        signal,
    })

    if (!response.ok) {
        throw new ProjectRequestError(response.status, response.statusText)
    }

    if (response.status !== 201) {
        throw new ProjectResponseError(
            `Unexpected project creation response status: ${response.status}`,
        )
    }

    return readProject(response)
}
