import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
    uploadProjectFile,
    ProjectFileRequestError,
    ProjectFileResponseError,
} from '../api/projectFiles'

const fetchMock = vi.fn<typeof fetch>()

const projectId = '11111111-1111-4111-8111-111111111111'

const uploadedFile = {
    id: '22222222-2222-4222-8222-222222222222',
    projectId,
    uploadedByUserId: '33333333-3333-4333-8333-333333333333',
    contentSha256: 'a'.repeat(64),
    originalFilename: 'gripper.step',
    mediaType: 'model/step',
    createdAt: '2026-10-18T12:00:00Z',
}

function sourceFile(): File {
    return new File(['CAD source bytes'], 'gripper.step', {
        type: 'model/step',
    })
}

function jsonResponse(body: unknown, status = 201): Response {
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

describe('project file upload API', () => {
    it('uploads one source file as multipart form data', async () => {
        const file = sourceFile()

        fetchMock.mockResolvedValueOnce(jsonResponse(uploadedFile))

        await expect(uploadProjectFile(projectId, file)).resolves.toEqual(uploadedFile)

        expect(fetchMock).toHaveBeenCalledExactlyOnceWith(
            `/api/projects/${projectId}/files`,
            expect.objectContaining({
                method: 'POST',
                credentials: 'same-origin',
                headers: {
                    Accept: 'application/json',
                },
                signal: undefined,
            }),
        )

        const options = fetchMock.mock.calls[0]![1]!

        expect(options.body).toBeInstanceOf(FormData)

        const body = options.body as FormData

        expect([...body.keys()]).toEqual(['file'])
        expect(body.get('file')).toBe(file)

        // The browser must generate the multipart boundary.
        expect(options.headers).not.toHaveProperty('Content-Type')
    })

    it.each([400, 401, 403, 404, 409, 500])('preserves HTTP %i upload errors', async (status) => {
        fetchMock.mockResolvedValueOnce(
            new Response(null, {
                status,
                statusText: 'Request failed',
            }),
        )

        await expect(uploadProjectFile(projectId, sourceFile())).rejects.toMatchObject({
            name: 'ProjectFileRequestError',
            status,
            statusText: 'Request failed',
        })
    })

    it('uses the structured request error type', async () => {
        fetchMock.mockResolvedValueOnce(new Response(null, { status: 500 }))

        await expect(uploadProjectFile(projectId, sourceFile())).rejects.toBeInstanceOf(
            ProjectFileRequestError,
        )
    })

    it('rejects an unexpected successful status', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(uploadedFile, 200))

        await expect(uploadProjectFile(projectId, sourceFile())).rejects.toBeInstanceOf(
            ProjectFileResponseError,
        )
    })

    it('rejects invalid JSON after a successful upload', async () => {
        fetchMock.mockResolvedValueOnce(new Response('not valid JSON', { status: 201 }))

        await expect(uploadProjectFile(projectId, sourceFile())).rejects.toBeInstanceOf(
            ProjectFileResponseError,
        )
    })

    it.each([
        ['an array', [uploadedFile]],
        ['a missing file ID', { ...uploadedFile, id: undefined }],
        ['an invalid content hash', { ...uploadedFile, contentSha256: 'bad' }],
        ['a missing filename', { ...uploadedFile, originalFilename: '' }],
        ['an invalid media type', { ...uploadedFile, mediaType: 123 }],
    ])('rejects a response containing %s', async (_reason, body) => {
        fetchMock.mockResolvedValueOnce(jsonResponse(body))

        await expect(uploadProjectFile(projectId, sourceFile())).rejects.toBeInstanceOf(
            ProjectFileResponseError,
        )
    })

    it('rejects an uploaded file belonging to a different project', async () => {
        fetchMock.mockResolvedValueOnce(
            jsonResponse({
                ...uploadedFile,
                projectId: '44444444-4444-4444-8444-444444444444',
            }),
        )

        await expect(uploadProjectFile(projectId, sourceFile())).rejects.toBeInstanceOf(
            ProjectFileResponseError,
        )
    })

    it('URL-encodes the project ID', async () => {
        const encodedProjectId = 'project/id'

        fetchMock.mockResolvedValueOnce(
            jsonResponse({
                ...uploadedFile,
                projectId: encodedProjectId,
            }),
        )

        await uploadProjectFile(encodedProjectId, sourceFile())

        expect(fetchMock).toHaveBeenCalledWith(
            '/api/projects/project%2Fid/files',
            expect.any(Object),
        )
    })

    it('passes cancellation signals to fetch', async () => {
        const controller = new AbortController()

        fetchMock.mockResolvedValueOnce(jsonResponse(uploadedFile))

        await uploadProjectFile(projectId, sourceFile(), controller.signal)

        expect(fetchMock).toHaveBeenCalledWith(
            `/api/projects/${projectId}/files`,
            expect.objectContaining({
                signal: controller.signal,
            }),
        )
    })

    it('preserves network failures without inventing an HTTP status', async () => {
        const failure = new TypeError('Failed to fetch')

        fetchMock.mockRejectedValueOnce(failure)

        await expect(uploadProjectFile(projectId, sourceFile())).rejects.toBe(failure)
    })
})
