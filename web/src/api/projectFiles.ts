export type ProjectFile = {
    id: string
    projectId: string
    uploadedByUserId: string
    contentSha256: string
    originalFilename: string
    mediaType: string | null
    createdAt: string
}

export class ProjectFileRequestError extends Error {
    readonly status: number
    readonly statusText: string

    constructor(status: number, statusText: string) {
        super(`Project file request failed with HTTP ${status}`)

        this.name = 'ProjectFileRequestError'
        this.status = status
        this.statusText = statusText
    }
}

export class ProjectFileResponseError extends Error {
    constructor(message: string) {
        super(message)

        this.name = 'ProjectFileResponseError'
    }
}

function isProjectFile(value: unknown): value is ProjectFile {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) {
        return false
    }

    const candidate = value as Record<string, unknown>

    return (
        typeof candidate.id === 'string' &&
        candidate.id.trim().length > 0 &&
        typeof candidate.projectId === 'string' &&
        candidate.projectId.trim().length > 0 &&
        typeof candidate.uploadedByUserId === 'string' &&
        candidate.uploadedByUserId.trim().length > 0 &&
        typeof candidate.contentSha256 === 'string' &&
        /^[0-9a-f]{64}$/.test(candidate.contentSha256) &&
        typeof candidate.originalFilename === 'string' &&
        candidate.originalFilename.trim().length > 0 &&
        (candidate.mediaType === null || typeof candidate.mediaType === 'string') &&
        typeof candidate.createdAt === 'string' &&
        candidate.createdAt.trim().length > 0
    )
}

async function readProjectFiles(response: Response, projectId: string): Promise<ProjectFile[]> {
    let body: unknown

    try {
        body = await response.json()
    } catch {
        throw new ProjectFileResponseError('Project file response did not contain valid JSON')
    }

    if (
        !Array.isArray(body) ||
        !body.every((file) => isProjectFile(file) && file.projectId === projectId)
    ) {
        throw new ProjectFileResponseError(
            'Project file response did not contain a valid project file list',
        )
    }

    return body
}

export async function listProjectFiles(
    projectId: string,
    signal?: AbortSignal,
): Promise<ProjectFile[]> {
    const response = await fetch(`/api/projects/${encodeURIComponent(projectId)}/files`, {
        method: 'GET',
        credentials: 'same-origin',
        headers: {
            Accept: 'application/json',
        },
        signal,
    })

    if (!response.ok) {
        throw new ProjectFileRequestError(response.status, response.statusText)
    }

    if (response.status !== 200) {
        throw new ProjectFileResponseError(
            `Unexpected project file response status: ${response.status}`,
        )
    }

    return readProjectFiles(response, projectId)
}

export async function uploadProjectFile(
    projectId: string,
    file: File,
    signal?: AbortSignal,
): Promise<ProjectFile> {
    const formData = new FormData()
    formData.append('file', file)

    const response = await fetch(`/api/projects/${encodeURIComponent(projectId)}/files`, {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
            Accept: 'application/json',
        },
        body: formData,
        signal,
    })

    if (!response.ok) {
        throw new ProjectFileRequestError(response.status, response.statusText)
    }

    if (response.status !== 201) {
        throw new ProjectFileResponseError(
            `Unexpected project file upload response status: ${response.status}`,
        )
    }

    let body: unknown

    try {
        body = await response.json()
    } catch {
        throw new ProjectFileResponseError(
            'Project file upload response did not contain valid JSON',
        )
    }

    if (!isProjectFile(body) || body.projectId !== projectId) {
        throw new ProjectFileResponseError(
            'Project file upload response did not contain a valid project file',
        )
    }

    return body
}
