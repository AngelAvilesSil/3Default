import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
    createProjectRevision,
    getProjectRevision,
    listProjectBranchHistory,
    listProjectRevisionFiles,
    RevisionRequestError,
    RevisionResponseError,
    type CreateProjectRevisionInput,
} from '../api/revisions'

const fetchMock = vi.fn<typeof fetch>()

const projectId = '11111111-1111-4111-8111-111111111111'
const branchId = '22222222-2222-4222-8222-222222222222'
const revisionId = '33333333-3333-4333-8333-333333333333'
const fileId = '44444444-4444-4444-8444-444444444444'
const nextFileId = '55555555-5555-4555-8555-555555555555'

const revision = {
    id: revisionId,
    projectId,
    authorUserId: '66666666-6666-4666-8666-666666666666',
    message: 'Initial mounting bracket',
    parentRevisionId: null,
    mergeParentRevisionId: null,
    createdAt: '2026-10-19T12:00:00Z',
}

const projectFile = {
    id: fileId,
    projectId,
    uploadedByUserId: revision.authorUserId,
    contentSha256: 'a'.repeat(64),
    originalFilename: 'bracket.step',
    mediaType: null,
    createdAt: '2026-10-19T11:00:00Z',
}

const creation: CreateProjectRevisionInput = {
    message: 'Initial mounting bracket',
    expectedHeadRevisionId: null,
    projectFileIds: [fileId],
}

function jsonResponse(body: unknown, status = 200): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
    })
}

function requestOptions(method: 'GET' | 'POST', signal?: AbortSignal) {
    const bodyMatcher = expect.any(String)

    return {
        method,
        credentials: 'same-origin',
        headers: {
            Accept: 'application/json',
            ...(method === 'POST' ? { 'Content-Type': 'application/json' } : {}),
        },
        signal,
        ...(method === 'POST' ? { body: bodyMatcher } : {}),
    }
}

beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => {
    vi.unstubAllGlobals()
})

describe('revision creation API', () => {
    it('creates the first revision with an explicit null head and exact file snapshot', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(revision, 201))

        await expect(createProjectRevision(projectId, branchId, creation)).resolves.toEqual(
            revision,
        )

        expect(fetchMock).toHaveBeenCalledExactlyOnceWith(
            `/api/projects/${projectId}/branches/${branchId}/revisions`,
            requestOptions('POST'),
        )

        const options = fetchMock.mock.calls[0]![1]!

        expect(JSON.parse(String(options.body))).toEqual({
            message: creation.message,
            expectedHeadRevisionId: null,
            projectFileIds: [fileId],
        })
    })

    it('preserves an observed non-null branch head and multiple files', async () => {
        const input = {
            ...creation,
            expectedHeadRevisionId: revisionId,
            projectFileIds: [fileId, nextFileId],
            message: 'Updated mounting bracket',
        }

        fetchMock.mockResolvedValueOnce(
            jsonResponse(
                {
                    ...revision,
                    parentRevisionId: revisionId,
                    message: input.message,
                },
                201,
            ),
        )

        await createProjectRevision(projectId, branchId, input)

        expect(JSON.parse(String(fetchMock.mock.calls[0]![1]!.body))).toEqual(input)
    })

    it('preserves an intentionally empty file snapshot', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(revision, 201))

        await createProjectRevision(projectId, branchId, {
            ...creation,
            projectFileIds: [],
        })

        expect(JSON.parse(String(fetchMock.mock.calls[0]![1]!.body))).toEqual({
            message: creation.message,
            expectedHeadRevisionId: null,
            projectFileIds: [],
        })
    })

    it('preserves an explicitly supplied optional merge parent', async () => {
        const mergeParentRevisionId = '77777777-7777-4777-8777-777777777777'

        fetchMock.mockResolvedValueOnce(jsonResponse(revision, 201))

        await createProjectRevision(projectId, branchId, {
            ...creation,
            expectedHeadRevisionId: revisionId,
            mergeParentRevisionId,
        })

        expect(JSON.parse(String(fetchMock.mock.calls[0]![1]!.body))).toMatchObject({
            expectedHeadRevisionId: revisionId,
            mergeParentRevisionId,
        })
    })

    it('preserves an explicitly null optional merge parent', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(revision, 201))

        await createProjectRevision(projectId, branchId, {
            ...creation,
            mergeParentRevisionId: null,
        })

        expect(JSON.parse(String(fetchMock.mock.calls[0]![1]!.body))).toHaveProperty(
            'mergeParentRevisionId',
            null,
        )
    })

    it.each([400, 401, 403, 404, 409, 500])('preserves HTTP %i creation errors', async (status) => {
        fetchMock.mockResolvedValueOnce(
            new Response(null, {
                status,
                statusText: 'Request failed',
            }),
        )

        await expect(createProjectRevision(projectId, branchId, creation)).rejects.toMatchObject({
            name: 'RevisionRequestError',
            status,
            statusText: 'Request failed',
        })
    })

    it('uses the structured HTTP error class', async () => {
        fetchMock.mockResolvedValueOnce(new Response(null, { status: 409 }))

        await expect(createProjectRevision(projectId, branchId, creation)).rejects.toBeInstanceOf(
            RevisionRequestError,
        )
    })

    it('rejects a successful status other than 201', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(revision, 200))

        await expect(createProjectRevision(projectId, branchId, creation)).rejects.toBeInstanceOf(
            RevisionResponseError,
        )
    })

    it('rejects malformed JSON', async () => {
        fetchMock.mockResolvedValueOnce(new Response('invalid JSON', { status: 201 }))

        await expect(createProjectRevision(projectId, branchId, creation)).rejects.toBeInstanceOf(
            RevisionResponseError,
        )
    })

    it.each([
        ['missing revision ID', { ...revision, id: undefined }],
        ['blank revision ID', { ...revision, id: ' ' }],
        ['wrong project ID', { ...revision, projectId: 'different-project' }],
        ['missing author ID', { ...revision, authorUserId: undefined }],
        ['blank message', { ...revision, message: ' ' }],
        ['missing first parent', { ...revision, parentRevisionId: undefined }],
        ['invalid first parent', { ...revision, parentRevisionId: 42 }],
        ['missing merge parent', { ...revision, mergeParentRevisionId: undefined }],
        ['invalid merge parent', { ...revision, mergeParentRevisionId: false }],
        ['invalid creation time', { ...revision, createdAt: 'invalid-date' }],
    ])('rejects a revision response with %s', async (_reason, invalid) => {
        fetchMock.mockResolvedValueOnce(jsonResponse(invalid, 201))

        await expect(createProjectRevision(projectId, branchId, creation)).rejects.toBeInstanceOf(
            RevisionResponseError,
        )
    })
})

describe('revision detail API', () => {
    it('loads a revision with authenticated GET', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(revision))

        await expect(getProjectRevision(projectId, revisionId)).resolves.toEqual(revision)

        expect(fetchMock).toHaveBeenCalledExactlyOnceWith(
            `/api/projects/${projectId}/revisions/${revisionId}`,
            requestOptions('GET'),
        )
    })

    it('rejects the wrong revision identity', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse({ ...revision, id: 'different-revision' }))

        await expect(getProjectRevision(projectId, revisionId)).rejects.toBeInstanceOf(
            RevisionResponseError,
        )
    })

    it('rejects a different project', async () => {
        fetchMock.mockResolvedValueOnce(
            jsonResponse({ ...revision, projectId: 'different-project' }),
        )

        await expect(getProjectRevision(projectId, revisionId)).rejects.toBeInstanceOf(
            RevisionResponseError,
        )
    })

    it('preserves HTTP 404 from detail lookup', async () => {
        fetchMock.mockResolvedValueOnce(new Response(null, { status: 404 }))

        await expect(getProjectRevision(projectId, revisionId)).rejects.toMatchObject({
            status: 404,
        })
    })

    it('rejects an unexpected successful status', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(revision, 201))

        await expect(getProjectRevision(projectId, revisionId)).rejects.toBeInstanceOf(
            RevisionResponseError,
        )
    })
})

describe('branch revision history API', () => {
    it('loads revisions in the backend-provided order', async () => {
        const parent = {
            ...revision,
            id: '88888888-8888-4888-8888-888888888888',
            message: 'Original bracket',
        }

        fetchMock.mockResolvedValueOnce(jsonResponse([revision, parent]))

        await expect(listProjectBranchHistory(projectId, branchId)).resolves.toEqual([
            revision,
            parent,
        ])

        expect(fetchMock).toHaveBeenCalledExactlyOnceWith(
            `/api/projects/${projectId}/branches/${branchId}/history`,
            requestOptions('GET'),
        )
    })

    it('accepts an empty branch history', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([]))

        await expect(listProjectBranchHistory(projectId, branchId)).resolves.toEqual([])
    })

    it('rejects a non-array history', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(revision))

        await expect(listProjectBranchHistory(projectId, branchId)).rejects.toBeInstanceOf(
            RevisionResponseError,
        )
    })

    it('rejects a revision from another project in history', async () => {
        fetchMock.mockResolvedValueOnce(
            jsonResponse([revision, { ...revision, projectId: 'another-project' }]),
        )

        await expect(listProjectBranchHistory(projectId, branchId)).rejects.toBeInstanceOf(
            RevisionResponseError,
        )
    })

    it('preserves expired-session errors', async () => {
        fetchMock.mockResolvedValueOnce(new Response(null, { status: 401 }))

        await expect(listProjectBranchHistory(projectId, branchId)).rejects.toMatchObject({
            status: 401,
        })
    })

    it('rejects a successful response status other than 200', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([], 201))

        await expect(listProjectBranchHistory(projectId, branchId)).rejects.toBeInstanceOf(
            RevisionResponseError,
        )
    })
})

describe('exact revision file snapshot API', () => {
    it('retrieves the exact revision file snapshot using authenticated GET', async () => {
        const secondFile = {
            ...projectFile,
            id: nextFileId,
            originalFilename: 'housing.glb',
            contentSha256: 'b'.repeat(64),
        }

        fetchMock.mockResolvedValueOnce(jsonResponse([projectFile, secondFile]))

        await expect(listProjectRevisionFiles(projectId, revisionId)).resolves.toEqual([
            projectFile,
            secondFile,
        ])

        expect(fetchMock).toHaveBeenCalledExactlyOnceWith(
            `/api/projects/${projectId}/revisions/${revisionId}/files`,
            requestOptions('GET'),
        )
    })

    it('accepts an explicitly empty revision file snapshot', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([]))

        await expect(listProjectRevisionFiles(projectId, revisionId)).resolves.toEqual([])
    })

    it('rejects a non-array file snapshot', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(projectFile))

        await expect(listProjectRevisionFiles(projectId, revisionId)).rejects.toBeInstanceOf(
            RevisionResponseError,
        )
    })

    it.each([
        ['missing file ID', { ...projectFile, id: undefined }],
        ['wrong project ID', { ...projectFile, projectId: 'different-project' }],
        ['missing uploader', { ...projectFile, uploadedByUserId: undefined }],
        ['invalid content SHA-256', { ...projectFile, contentSha256: 'bad-hash' }],
        ['missing filename', { ...projectFile, originalFilename: '' }],
        ['missing media type', { ...projectFile, mediaType: undefined }],
        ['invalid timestamp', { ...projectFile, createdAt: 123 }],
    ])('rejects a snapshot file with %s', async (_reason, invalid) => {
        fetchMock.mockResolvedValueOnce(jsonResponse([invalid]))

        await expect(listProjectRevisionFiles(projectId, revisionId)).rejects.toBeInstanceOf(
            RevisionResponseError,
        )
    })

    it('preserves a nullable media type', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([projectFile]))

        await expect(listProjectRevisionFiles(projectId, revisionId)).resolves.toEqual([
            projectFile,
        ])
    })

    it('preserves HTTP 404 rather than treating it as an empty snapshot', async () => {
        fetchMock.mockResolvedValueOnce(new Response(null, { status: 404 }))

        await expect(listProjectRevisionFiles(projectId, revisionId)).rejects.toMatchObject({
            status: 404,
        })
    })

    it('rejects invalid JSON from a successful response', async () => {
        fetchMock.mockResolvedValueOnce(new Response('invalid JSON', { status: 200 }))

        await expect(listProjectRevisionFiles(projectId, revisionId)).rejects.toBeInstanceOf(
            RevisionResponseError,
        )
    })
})

describe('shared revision request behavior', () => {
    it('passes abort signals through all four operations', async () => {
        const controller = new AbortController()

        fetchMock
            .mockResolvedValueOnce(jsonResponse(revision, 201))
            .mockResolvedValueOnce(jsonResponse(revision))
            .mockResolvedValueOnce(jsonResponse([revision]))
            .mockResolvedValueOnce(jsonResponse([projectFile]))

        await createProjectRevision(projectId, branchId, creation, controller.signal)
        await getProjectRevision(projectId, revisionId, controller.signal)
        await listProjectBranchHistory(projectId, branchId, controller.signal)
        await listProjectRevisionFiles(projectId, revisionId, controller.signal)

        expect(fetchMock).toHaveBeenCalledTimes(4)

        for (const call of fetchMock.mock.calls) {
            expect(call[1]?.signal).toBe(controller.signal)
        }
    })

    it('URL-encodes identifiers in all four operations', async () => {
        const unusualProjectId = 'project/id'
        const unusualBranchId = 'branch/id'
        const unusualRevisionId = 'revision/id'
        const unusualFile = { ...projectFile, projectId: unusualProjectId }
        const unusualRevision = { ...revision, projectId: unusualProjectId, id: unusualRevisionId }

        fetchMock
            .mockResolvedValueOnce(jsonResponse(unusualRevision, 201))
            .mockResolvedValueOnce(jsonResponse(unusualRevision))
            .mockResolvedValueOnce(jsonResponse([unusualRevision]))
            .mockResolvedValueOnce(jsonResponse([unusualFile]))

        await createProjectRevision(unusualProjectId, unusualBranchId, creation)
        await getProjectRevision(unusualProjectId, unusualRevisionId)
        await listProjectBranchHistory(unusualProjectId, unusualBranchId)
        await listProjectRevisionFiles(unusualProjectId, unusualRevisionId)

        expect(fetchMock.mock.calls.map((call) => call[0])).toEqual([
            '/api/projects/project%2Fid/branches/branch%2Fid/revisions',
            '/api/projects/project%2Fid/revisions/revision%2Fid',
            '/api/projects/project%2Fid/branches/branch%2Fid/history',
            '/api/projects/project%2Fid/revisions/revision%2Fid/files',
        ])
    })

    it('propagates network failures without inventing an HTTP status', async () => {
        const error = new TypeError('Failed to fetch')

        fetchMock.mockRejectedValueOnce(error)

        await expect(getProjectRevision(projectId, revisionId)).rejects.toBe(error)
    })
})
