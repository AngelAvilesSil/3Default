import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
    ConversionJobRequestError,
    ConversionJobResponseError,
    createProjectFileConversionJob,
    getProjectConversionJob,
    listProjectFileConversionJobs,
} from '../api/conversionJobs'

const fetchMock = vi.fn<typeof fetch>()

const projectId = '11111111-1111-4111-8111-111111111111'
const fileId = '22222222-2222-4222-8222-222222222222'
const jobId = '33333333-3333-4333-8333-333333333333'

const pendingJob = {
    id: jobId,
    projectId,
    projectFileId: fileId,
    status: 'pending',
    attemptCount: 0,
    lastError: null,
    createdAt: '2026-10-18T12:00:00Z',
    startedAt: null,
    finishedAt: null,
    updatedAt: '2026-10-18T12:00:00Z',
}

const failedJob = {
    ...pendingJob,
    status: 'failed',
    attemptCount: 2,
    lastError: 'Unsupported source format',
    startedAt: '2026-10-18T12:01:00Z',
    finishedAt: '2026-10-18T12:02:00Z',
    updatedAt: '2026-10-18T12:02:00Z',
}

function jsonResponse(body: unknown, status = 200): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
    })
}

beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => {
    vi.unstubAllGlobals()
})

describe('conversion job API', () => {
    it('creates a job only through an explicit authenticated POST', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(pendingJob, 201))

        await expect(createProjectFileConversionJob(projectId, fileId)).resolves.toEqual(pendingJob)

        expect(fetchMock).toHaveBeenCalledExactlyOnceWith(
            `/api/projects/${projectId}/files/${fileId}/conversion-jobs`,
            {
                method: 'POST',
                credentials: 'same-origin',
                headers: { Accept: 'application/json' },
                signal: undefined,
            },
        )

        const options = fetchMock.mock.calls[0]![1]!

        expect(options.body).toBeUndefined()
        expect(options.headers).not.toHaveProperty('Content-Type')
    })

    it.each([400, 401, 403, 404, 409, 500])(
        'preserves HTTP %i creation failures',
        async (status) => {
            fetchMock.mockResolvedValueOnce(
                new Response(null, {
                    status,
                    statusText: 'Request failed',
                }),
            )

            await expect(createProjectFileConversionJob(projectId, fileId)).rejects.toMatchObject({
                name: 'ConversionJobRequestError',
                status,
                statusText: 'Request failed',
            })
        },
    )

    it('uses the structured request error class', async () => {
        fetchMock.mockResolvedValueOnce(new Response(null, { status: 409 }))

        await expect(createProjectFileConversionJob(projectId, fileId)).rejects.toBeInstanceOf(
            ConversionJobRequestError,
        )
    })

    it('rejects an unexpected successful creation status', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(pendingJob, 200))

        await expect(createProjectFileConversionJob(projectId, fileId)).rejects.toBeInstanceOf(
            ConversionJobResponseError,
        )
    })

    it('rejects invalid JSON after creation', async () => {
        fetchMock.mockResolvedValueOnce(new Response('not JSON', { status: 201 }))

        await expect(createProjectFileConversionJob(projectId, fileId)).rejects.toBeInstanceOf(
            ConversionJobResponseError,
        )
    })

    it.each([
        ['missing job ID', { ...pendingJob, id: '' }],
        ['wrong project', { ...pendingJob, projectId: 'different-project' }],
        ['wrong file', { ...pendingJob, projectFileId: 'different-file' }],
        ['invalid status', { ...pendingJob, status: 'unknown' }],
        ['negative attempts', { ...pendingJob, attemptCount: -1 }],
        ['fractional attempts', { ...pendingJob, attemptCount: 1.5 }],
        ['missing lastError', { ...pendingJob, lastError: undefined }],
        ['invalid startedAt', { ...pendingJob, startedAt: 123 }],
        ['missing updatedAt', { ...pendingJob, updatedAt: undefined }],
    ])('rejects conversion job responses with %s', async (_reason, body) => {
        fetchMock.mockResolvedValueOnce(jsonResponse(body, 201))

        await expect(createProjectFileConversionJob(projectId, fileId)).rejects.toBeInstanceOf(
            ConversionJobResponseError,
        )
    })

    it('accepts completed jobs with timestamps and error details', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(failedJob, 201))

        await expect(createProjectFileConversionJob(projectId, fileId)).resolves.toEqual(failedJob)
    })

    it('lists jobs for one project file in returned order', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([pendingJob, failedJob]))

        await expect(listProjectFileConversionJobs(projectId, fileId)).resolves.toEqual([
            pendingJob,
            failedJob,
        ])

        expect(fetchMock).toHaveBeenCalledExactlyOnceWith(
            `/api/projects/${projectId}/files/${fileId}/conversion-jobs`,
            {
                method: 'GET',
                credentials: 'same-origin',
                headers: { Accept: 'application/json' },
                signal: undefined,
            },
        )
    })

    it('accepts an empty conversion job history', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([]))

        await expect(listProjectFileConversionJobs(projectId, fileId)).resolves.toEqual([])
    })

    it('rejects invalid conversion job history', async () => {
        fetchMock.mockResolvedValueOnce(
            jsonResponse([pendingJob, { ...failedJob, projectFileId: 'another-file' }]),
        )

        await expect(listProjectFileConversionJobs(projectId, fileId)).rejects.toBeInstanceOf(
            ConversionJobResponseError,
        )
    })

    it('rejects a non-array job history', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(pendingJob))

        await expect(listProjectFileConversionJobs(projectId, fileId)).rejects.toBeInstanceOf(
            ConversionJobResponseError,
        )
    })

    it('preserves listing authentication errors', async () => {
        fetchMock.mockResolvedValueOnce(new Response(null, { status: 401 }))

        await expect(listProjectFileConversionJobs(projectId, fileId)).rejects.toMatchObject({
            status: 401,
        })
    })

    it('retrieves a conversion job by its project and job ID', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse({ ...pendingJob, status: 'running' }))

        await expect(getProjectConversionJob(projectId, jobId)).resolves.toMatchObject({
            id: jobId,
            projectId,
            projectFileId: fileId,
            status: 'running',
        })

        expect(fetchMock).toHaveBeenCalledExactlyOnceWith(
            `/api/projects/${projectId}/conversion-jobs/${jobId}`,
            {
                method: 'GET',
                credentials: 'same-origin',
                headers: { Accept: 'application/json' },
                signal: undefined,
            },
        )
    })

    it('rejects another job ID in a status response', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse({ ...pendingJob, id: 'different-job' }))

        await expect(getProjectConversionJob(projectId, jobId)).rejects.toBeInstanceOf(
            ConversionJobResponseError,
        )
    })

    it('preserves job lookup not-found responses', async () => {
        fetchMock.mockResolvedValueOnce(new Response(null, { status: 404 }))

        await expect(getProjectConversionJob(projectId, jobId)).rejects.toMatchObject({
            status: 404,
        })
    })

    it('passes the cancellation signal to all operations', async () => {
        const controller = new AbortController()

        fetchMock
            .mockResolvedValueOnce(jsonResponse(pendingJob, 201))
            .mockResolvedValueOnce(jsonResponse([pendingJob]))
            .mockResolvedValueOnce(jsonResponse(pendingJob))

        await createProjectFileConversionJob(projectId, fileId, controller.signal)

        await listProjectFileConversionJobs(projectId, fileId, controller.signal)

        await getProjectConversionJob(projectId, jobId, controller.signal)

        expect(fetchMock).toHaveBeenCalledTimes(3)

        for (const call of fetchMock.mock.calls) {
            expect(call[1]?.signal).toBe(controller.signal)
        }
    })

    it('URL-encodes path identifiers', async () => {
        const unusualProjectId = 'project/id'
        const unusualFileId = 'file/id'

        fetchMock.mockResolvedValueOnce(
            jsonResponse(
                {
                    ...pendingJob,
                    projectId: unusualProjectId,
                    projectFileId: unusualFileId,
                },
                201,
            ),
        )

        await createProjectFileConversionJob(unusualProjectId, unusualFileId)

        expect(fetchMock).toHaveBeenCalledWith(
            '/api/projects/project%2Fid/files/file%2Fid/conversion-jobs',
            expect.any(Object),
        )
    })

    it('preserves network errors without inventing HTTP status', async () => {
        const error = new TypeError('Failed to fetch')

        fetchMock.mockRejectedValueOnce(error)

        await expect(getProjectConversionJob(projectId, jobId)).rejects.toBe(error)
    })
})
