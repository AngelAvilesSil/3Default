export type ConversionJobStatus = 'pending' | 'running' | 'succeeded' | 'failed'

export type ConversionJob = {
    id: string
    projectId: string
    projectFileId: string
    status: ConversionJobStatus
    attemptCount: number
    lastError: string | null
    createdAt: string
    startedAt: string | null
    finishedAt: string | null
    updatedAt: string
}

export class ConversionJobRequestError extends Error {
    readonly status: number
    readonly statusText: string

    constructor(status: number, statusText: string) {
        super(`Conversion job request failed with HTTP ${status}`)
        this.name = 'ConversionJobRequestError'
        this.status = status
        this.statusText = statusText
    }
}

export class ConversionJobResponseError extends Error {
    constructor(message: string) {
        super(message)
        this.name = 'ConversionJobResponseError'
    }
}

function isNonEmptyString(value: unknown): value is string {
    return typeof value === 'string' && value.trim().length > 0
}

function isTimestamp(value: unknown): value is string {
    return isNonEmptyString(value) && !Number.isNaN(Date.parse(value))
}

function isOptionalTimestamp(value: unknown): value is string | null {
    return value === null || isTimestamp(value)
}

function isConversionJob(value: unknown): value is ConversionJob {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) {
        return false
    }

    const job = value as Record<string, unknown>

    const validStatus =
        job.status === 'pending' ||
        job.status === 'running' ||
        job.status === 'succeeded' ||
        job.status === 'failed'

    return (
        isNonEmptyString(job.id) &&
        isNonEmptyString(job.projectId) &&
        isNonEmptyString(job.projectFileId) &&
        validStatus &&
        typeof job.attemptCount === 'number' &&
        Number.isInteger(job.attemptCount) &&
        job.attemptCount >= 0 &&
        (job.lastError === null || typeof job.lastError === 'string') &&
        isTimestamp(job.createdAt) &&
        isOptionalTimestamp(job.startedAt) &&
        isOptionalTimestamp(job.finishedAt) &&
        isTimestamp(job.updatedAt)
    )
}

function requireConversionJob(
    value: unknown,
    projectId: string,
    projectFileId?: string,
    jobId?: string,
): ConversionJob {
    if (
        !isConversionJob(value) ||
        value.projectId !== projectId ||
        (projectFileId !== undefined && value.projectFileId !== projectFileId) ||
        (jobId !== undefined && value.id !== jobId)
    ) {
        throw new ConversionJobResponseError(
            'Conversion job response did not contain the expected job',
        )
    }

    return value
}

async function requestConversionJobJson(
    url: string,
    method: 'GET' | 'POST',
    expectedStatus: number,
    signal?: AbortSignal,
): Promise<unknown> {
    const response = await fetch(url, {
        method,
        credentials: 'same-origin',
        headers: {
            Accept: 'application/json',
        },
        signal,
    })

    if (!response.ok) {
        throw new ConversionJobRequestError(response.status, response.statusText)
    }

    if (response.status !== expectedStatus) {
        throw new ConversionJobResponseError(
            `Unexpected conversion job response status: ${response.status}`,
        )
    }

    try {
        return await response.json()
    } catch {
        throw new ConversionJobResponseError('Conversion job response did not contain valid JSON')
    }
}
export async function createProjectFileConversionJob(
    projectId: string,
    projectFileId: string,
    signal?: AbortSignal,
): Promise<ConversionJob> {
    const url =
        `/api/projects/${encodeURIComponent(projectId)}` +
        `/files/${encodeURIComponent(projectFileId)}/conversion-jobs`

    const body = await requestConversionJobJson(url, 'POST', 201, signal)

    return requireConversionJob(body, projectId, projectFileId)
}

export async function listProjectFileConversionJobs(
    projectId: string,
    projectFileId: string,
    signal?: AbortSignal,
): Promise<ConversionJob[]> {
    const url =
        `/api/projects/${encodeURIComponent(projectId)}` +
        `/files/${encodeURIComponent(projectFileId)}/conversion-jobs`

    const body = await requestConversionJobJson(url, 'GET', 200, signal)

    if (!Array.isArray(body)) {
        throw new ConversionJobResponseError('Conversion job history was not an array')
    }

    return body.map((value: unknown) => requireConversionJob(value, projectId, projectFileId))
}

export async function getProjectConversionJob(
    projectId: string,
    jobId: string,
    signal?: AbortSignal,
): Promise<ConversionJob> {
    const url =
        `/api/projects/${encodeURIComponent(projectId)}` +
        `/conversion-jobs/${encodeURIComponent(jobId)}`

    const body = await requestConversionJobJson(url, 'GET', 200, signal)

    return requireConversionJob(body, projectId, undefined, jobId)
}
