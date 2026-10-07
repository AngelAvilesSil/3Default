import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
    ProjectFilePreviewMediaTypeError,
    ProjectFilePreviewRequestError,
    fetchProjectFilePreview,
} from '../api/projectFilePreview'

const fetchMock = vi.fn<typeof fetch>()

describe('fetchProjectFilePreview', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        vi.stubGlobal('fetch', fetchMock)
    })

    it('returns GLB bytes for a successful preview response', async () => {
        const expected = new Uint8Array([0x67, 0x6c, 0x54, 0x46]).buffer

        fetchMock.mockResolvedValueOnce(
            new Response(expected, {
                status: 200,
                headers: {
                    'Content-Type': 'model/gltf-binary',
                },
            }),
        )

        const result = await fetchProjectFilePreview('project-id', 'project-file-id')

        expect(fetchMock).toHaveBeenCalledWith(
            '/api/projects/project-id/files/project-file-id/preview',
            {
                method: 'GET',
                credentials: 'same-origin',
                headers: {
                    Accept: 'model/gltf-binary',
                },
                signal: undefined,
            },
        )

        expect(result.status).toBe('available')

        if (result.status !== 'available') {
            throw new Error('expected available preview')
        }

        expect(new Uint8Array(result.data)).toEqual(new Uint8Array(expected))
    })

    it('returns not-found when no successful preview exists', async () => {
        fetchMock.mockResolvedValueOnce(
            new Response(null, {
                status: 404,
                statusText: 'Not Found',
            }),
        )

        await expect(fetchProjectFilePreview('project-id', 'project-file-id')).resolves.toEqual({
            status: 'not-found',
        })
    })

    it('throws a structured request error for non-404 HTTP failures', async () => {
        fetchMock.mockResolvedValueOnce(
            new Response(null, {
                status: 500,
                statusText: 'Internal Server Error',
            }),
        )

        const request = fetchProjectFilePreview('project-id', 'project-file-id')

        await expect(request).rejects.toMatchObject({
            name: 'ProjectFilePreviewRequestError',
            status: 500,
            statusText: 'Internal Server Error',
        })

        await expect(request).rejects.toBeInstanceOf(ProjectFilePreviewRequestError)
    })

    it('rejects successful responses with an unexpected media type', async () => {
        fetchMock.mockResolvedValueOnce(
            new Response(new ArrayBuffer(8), {
                status: 200,
                headers: {
                    'Content-Type': 'application/octet-stream',
                },
            }),
        )

        const request = fetchProjectFilePreview('project-id', 'project-file-id')

        await expect(request).rejects.toMatchObject({
            name: 'ProjectFilePreviewMediaTypeError',
            mediaType: 'application/octet-stream',
        })

        await expect(request).rejects.toBeInstanceOf(ProjectFilePreviewMediaTypeError)
    })

    it('accepts media-type parameters on a GLB response', async () => {
        fetchMock.mockResolvedValueOnce(
            new Response(new ArrayBuffer(8), {
                status: 200,
                headers: {
                    'Content-Type': 'model/gltf-binary; charset=binary',
                },
            }),
        )

        await expect(
            fetchProjectFilePreview('project-id', 'project-file-id'),
        ).resolves.toMatchObject({
            status: 'available',
        })
    })

    it('passes an AbortSignal to fetch', async () => {
        const controller = new AbortController()

        fetchMock.mockResolvedValueOnce(
            new Response(null, {
                status: 404,
            }),
        )

        await fetchProjectFilePreview('project-id', 'project-file-id', controller.signal)

        expect(fetchMock).toHaveBeenCalledWith(
            '/api/projects/project-id/files/project-file-id/preview',
            expect.objectContaining({
                signal: controller.signal,
            }),
        )
    })

    it('URL-encodes path parameters', async () => {
        fetchMock.mockResolvedValueOnce(
            new Response(null, {
                status: 404,
            }),
        )

        await fetchProjectFilePreview('project/id', 'file id')

        expect(fetchMock).toHaveBeenCalledWith(
            '/api/projects/project%2Fid/files/file%20id/preview',
            expect.any(Object),
        )
    })
})
