import { isProjectFile, type ProjectFile } from './projectFiles'

export type ProjectRevision = {
    id: string
    projectId: string
    authorUserId: string
    message: string
    parentRevisionId: string | null
    mergeParentRevisionId: string | null
    createdAt: string
}

export type CreateProjectRevisionInput = {
    message: string
    expectedHeadRevisionId: string | null
    projectFileIds: string[]
    mergeParentRevisionId?: string | null
}

export class RevisionRequestError extends Error {
    readonly status: number
    readonly statusText: string

    constructor(status: number, statusText: string) {
        super(`Revision request failed with HTTP ${status}`)
        this.name = 'RevisionRequestError'
        this.status = status
        this.statusText = statusText
    }
}

export class RevisionResponseError extends Error {
    constructor(message: string) {
        super(message)
        this.name = 'RevisionResponseError'
    }
}

function isNonEmptyString(value: unknown): value is string {
    return typeof value === 'string' && value.trim().length > 0
}

function isTimestamp(value: unknown): value is string {
    return isNonEmptyString(value) && !Number.isNaN(Date.parse(value))
}

function isNullableRevisionId(value: unknown): value is string | null {
    return value === null || isNonEmptyString(value)
}

function isProjectRevision(value: unknown): value is ProjectRevision {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) {
        return false
    }

    const revision = value as Record<string, unknown>

    return (
        isNonEmptyString(revision.id) &&
        isNonEmptyString(revision.projectId) &&
        isNonEmptyString(revision.authorUserId) &&
        isNonEmptyString(revision.message) &&
        isNullableRevisionId(revision.parentRevisionId) &&
        isNullableRevisionId(revision.mergeParentRevisionId) &&
        isTimestamp(revision.createdAt)
    )
}

function requireProjectRevision(
    value: unknown,
    projectId: string,
    revisionId?: string,
): ProjectRevision {
    if (
        !isProjectRevision(value) ||
        value.projectId !== projectId ||
        (revisionId !== undefined && value.id !== revisionId)
    ) {
        throw new RevisionResponseError('Revision response did not contain the expected revision')
    }

    return value
}

async function requestRevisionJson(
    url: string,
    method: 'GET' | 'POST',
    expectedStatus: number,
    signal?: AbortSignal,
    input?: CreateProjectRevisionInput,
): Promise<unknown> {
    const options: RequestInit = {
        method,
        credentials: 'same-origin',
        headers: {
            Accept: 'application/json',
            ...(method === 'POST' ? { 'Content-Type': 'application/json' } : {}),
        },
        signal,
    }

    if (method === 'POST') {
        if (input === undefined) {
            throw new Error('Revision creation input is required')
        }

        options.body = JSON.stringify({
            message: input.message,
            expectedHeadRevisionId: input.expectedHeadRevisionId,
            projectFileIds: input.projectFileIds,
            ...(input.mergeParentRevisionId !== undefined
                ? { mergeParentRevisionId: input.mergeParentRevisionId }
                : {}),
        })
    }

    const response = await fetch(url, options)

    if (!response.ok) {
        throw new RevisionRequestError(response.status, response.statusText)
    }

    if (response.status !== expectedStatus) {
        throw new RevisionResponseError(`Unexpected revision response status: ${response.status}`)
    }

    try {
        return await response.json()
    } catch {
        throw new RevisionResponseError('Revision response did not contain valid JSON')
    }
}

export async function createProjectRevision(
    projectId: string,
    branchId: string,
    input: CreateProjectRevisionInput,
    signal?: AbortSignal,
): Promise<ProjectRevision> {
    const url =
        `/api/projects/${encodeURIComponent(projectId)}` +
        `/branches/${encodeURIComponent(branchId)}/revisions`

    const body = await requestRevisionJson(url, 'POST', 201, signal, input)

    return requireProjectRevision(body, projectId)
}

export async function getProjectRevision(
    projectId: string,
    revisionId: string,
    signal?: AbortSignal,
): Promise<ProjectRevision> {
    const url =
        `/api/projects/${encodeURIComponent(projectId)}` +
        `/revisions/${encodeURIComponent(revisionId)}`

    const body = await requestRevisionJson(url, 'GET', 200, signal)

    return requireProjectRevision(body, projectId, revisionId)
}

export async function listProjectBranchHistory(
    projectId: string,
    branchId: string,
    signal?: AbortSignal,
): Promise<ProjectRevision[]> {
    const url =
        `/api/projects/${encodeURIComponent(projectId)}` +
        `/branches/${encodeURIComponent(branchId)}/history`

    const body = await requestRevisionJson(url, 'GET', 200, signal)

    if (!Array.isArray(body)) {
        throw new RevisionResponseError('Branch revision history was not an array')
    }

    return body.map((value: unknown) => requireProjectRevision(value, projectId))
}

export async function listProjectRevisionFiles(
    projectId: string,
    revisionId: string,
    signal?: AbortSignal,
): Promise<ProjectFile[]> {
    const url =
        `/api/projects/${encodeURIComponent(projectId)}` +
        `/revisions/${encodeURIComponent(revisionId)}/files`

    const body = await requestRevisionJson(url, 'GET', 200, signal)

    if (!Array.isArray(body)) {
        throw new RevisionResponseError('Revision file snapshot was not an array')
    }

    return body.map((value: unknown): ProjectFile => {
        if (!isProjectFile(value) || value.projectId !== projectId) {
            throw new RevisionResponseError(
                'Revision file snapshot contained an invalid project file',
            )
        }

        return value
    })
}
