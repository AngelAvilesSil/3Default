import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
    listProjectFiles,
    ProjectFileRequestError,
    ProjectFileResponseError,
} from '../api/projectFiles'

const fetchMock = vi.fn<typeof fetch>()

const projectId = '11111111-1111-4111-8111-111111111111'

const file = {
    id: '22222222-2222-4222-8222-222222222222',
    projectId,
    uploadedByUserId: '33333333-3333-4333-8333-333333333333',
    contentSha256: 'a'.repeat(64),
    originalFilename: 'gripper.step',
    mediaType: 'model/step',
    createdAt: '2026-10-15T12:00:00Z',
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

describe('project file listing API', () => {
    it('retrieves project file metadata using same-origin authentication', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([file]))

        await expect(listProjectFiles(projectId)).resolves.toEqual([file])

        expect(fetchMock).toHaveBeenCalledWith(`/api/projects/${projectId}/files`, {
            method: 'GET',
            credentials: 'same-origin',
            headers: {
                Accept: 'application/json',
            },
            signal: undefined,
        })
    })

    it('accepts an empty project file list', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([]))

        await expect(listProjectFiles(projectId)).resolves.toEqual([])
    })

    it('accepts a nullable media type', async () => {
        const fileWithoutMediaType = {
            ...file,
            mediaType: null,
        }

        fetchMock.mockResolvedValueOnce(jsonResponse([fileWithoutMediaType]))

        await expect(listProjectFiles(projectId)).resolves.toEqual([fileWithoutMediaType])
    })

    it('preserves file order and metadata', async () => {
        const secondFile = {
            ...file,
            id: '44444444-4444-4444-8444-444444444444',
            contentSha256: 'b'.repeat(64),
            originalFilename: 'housing.glb',
            mediaType: 'model/gltf-binary',
        }

        fetchMock.mockResolvedValueOnce(jsonResponse([file, secondFile]))

        await expect(listProjectFiles(projectId)).resolves.toEqual([file, secondFile])
    })

    it.each([400, 401, 404, 500])('preserves HTTP %i request errors', async (status) => {
        fetchMock.mockResolvedValueOnce(
            new Response(null, {
                status,
                statusText: 'Request failed',
            }),
        )

        await expect(listProjectFiles(projectId)).rejects.toMatchObject({
            name: 'ProjectFileRequestError',
            status,
            statusText: 'Request failed',
        })
    })

    it('uses the structured request error type', async () => {
        fetchMock.mockResolvedValueOnce(new Response(null, { status: 500 }))

        await expect(listProjectFiles(projectId)).rejects.toBeInstanceOf(ProjectFileRequestError)
    })

    it('rejects an unexpected successful status', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([], 201))

        await expect(listProjectFiles(projectId)).rejects.toBeInstanceOf(ProjectFileResponseError)
    })

    it('rejects invalid JSON', async () => {
        fetchMock.mockResolvedValueOnce(new Response('invalid JSON', { status: 200 }))

        await expect(listProjectFiles(projectId)).rejects.toBeInstanceOf(ProjectFileResponseError)
    })

    it('requires an array response', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(file))

        await expect(listProjectFiles(projectId)).rejects.toBeInstanceOf(ProjectFileResponseError)
    })

    it.each([
        ['missing file ID', { ...file, id: undefined }],
        ['missing project ID', { ...file, projectId: undefined }],
        ['missing uploader ID', { ...file, uploadedByUserId: undefined }],
        ['invalid content hash', { ...file, contentSha256: 'invalid' }],
        ['blank original filename', { ...file, originalFilename: ' ' }],
        ['missing media type', { ...file, mediaType: undefined }],
        ['invalid media type', { ...file, mediaType: 123 }],
        ['missing creation time', { ...file, createdAt: undefined }],
    ])('rejects a file with %s', async (_reason, invalidFile) => {
        fetchMock.mockResolvedValueOnce(jsonResponse([invalidFile]))

        await expect(listProjectFiles(projectId)).rejects.toBeInstanceOf(ProjectFileResponseError)
    })

    it('rejects files belonging to a different project', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([{ ...file, projectId: 'another-project' }]))

        await expect(listProjectFiles(projectId)).rejects.toBeInstanceOf(ProjectFileResponseError)
    })

    it('passes cancellation signals to fetch', async () => {
        const controller = new AbortController()

        fetchMock.mockResolvedValueOnce(jsonResponse([]))

        await listProjectFiles(projectId, controller.signal)

        expect(fetchMock).toHaveBeenCalledWith(
            `/api/projects/${projectId}/files`,
            expect.objectContaining({
                signal: controller.signal,
            }),
        )
    })

    it('URL-encodes the project ID', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([]))

        await listProjectFiles('project/id')

        expect(fetchMock).toHaveBeenCalledWith(
            '/api/projects/project%2Fid/files',
            expect.any(Object),
        )
    })

    it('propagates network failures', async () => {
        const error = new TypeError('Failed to fetch')
        fetchMock.mockRejectedValueOnce(error)

        await expect(listProjectFiles(projectId)).rejects.toBe(error)
    })
})
