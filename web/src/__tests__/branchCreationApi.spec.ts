import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
    createProjectBranch,
    ProjectRequestError,
    ProjectResponseError,
    type CreateProjectBranchInput,
} from '../api/projects'

const fetchMock = vi.fn<typeof fetch>()

const projectId = '11111111-1111-4111-8111-111111111111'
const sourceRevisionId = '22222222-2222-4222-8222-222222222222'

const branch = {
    id: '33333333-3333-4333-8333-333333333333',
    projectId,
    name: 'design-experiment',
    headRevisionId: sourceRevisionId,
    createdAt: '2026-10-23T12:00:00Z',
    updatedAt: '2026-10-23T12:00:00Z',
}

const input: CreateProjectBranchInput = {
    name: 'design-experiment',
    headRevisionId: sourceRevisionId,
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

describe('project branch creation API', () => {
    it('creates a branch at an exact existing revision', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(branch))

        await expect(createProjectBranch(projectId, input)).resolves.toEqual(branch)

        expect(fetchMock).toHaveBeenCalledExactlyOnceWith(`/api/projects/${projectId}/branches`, {
            method: 'POST',
            credentials: 'same-origin',
            headers: {
                Accept: 'application/json',
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                name: 'design-experiment',
                headRevisionId: sourceRevisionId,
            }),
            signal: undefined,
        })
    })

    it('sends an explicit null when creating an empty branch', async () => {
        const emptyBranch = { ...branch, headRevisionId: null }

        fetchMock.mockResolvedValueOnce(jsonResponse(emptyBranch))

        await expect(
            createProjectBranch(projectId, {
                name: 'design-experiment',
                headRevisionId: null,
            }),
        ).resolves.toEqual(emptyBranch)

        const requestBody = JSON.parse(fetchMock.mock.calls[0]![1]!.body as string) as Record<
            string,
            unknown
        >

        expect(requestBody).toEqual({
            name: 'design-experiment',
            headRevisionId: null,
        })
    })

    it('preserves the exact supplied branch name in the request', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(branch))

        await createProjectBranch(projectId, {
            ...input,
            name: ' design-experiment ',
        })

        const requestBody = JSON.parse(fetchMock.mock.calls[0]![1]!.body as string) as Record<
            string,
            unknown
        >

        expect(requestBody.name).toBe(' design-experiment ')
    })

    it('URL-encodes the project identifier', async () => {
        const unusualProjectId = 'project/id'

        fetchMock.mockResolvedValueOnce(jsonResponse({ ...branch, projectId: unusualProjectId }))

        await createProjectBranch(unusualProjectId, input)

        expect(fetchMock.mock.calls[0]![0]).toBe('/api/projects/project%2Fid/branches')
    })

    it('passes request cancellation to fetch', async () => {
        const controller = new AbortController()

        fetchMock.mockResolvedValueOnce(jsonResponse(branch))

        await createProjectBranch(projectId, input, controller.signal)

        expect(fetchMock.mock.calls[0]![1]?.signal).toBe(controller.signal)
    })

    it.each([
        [400, 'Bad Request'],
        [401, 'Unauthorized'],
        [403, 'Forbidden'],
        [404, 'Not Found'],
        [409, 'Conflict'],
        [500, 'Internal Server Error'],
    ])('preserves HTTP %i branch-creation failures', async (status, statusText) => {
        fetchMock.mockResolvedValueOnce(new Response(null, { status, statusText }))

        const request = createProjectBranch(projectId, input)

        await expect(request).rejects.toMatchObject({
            name: 'ProjectRequestError',
            status,
            statusText,
        })

        await expect(request).rejects.toBeInstanceOf(ProjectRequestError)
    })

    it('rejects an unexpected successful HTTP status', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(branch, 200))

        await expect(createProjectBranch(projectId, input)).rejects.toBeInstanceOf(
            ProjectResponseError,
        )
    })

    it('rejects invalid JSON in a successful response', async () => {
        fetchMock.mockResolvedValueOnce(new Response('invalid JSON', { status: 201 }))

        await expect(createProjectBranch(projectId, input)).rejects.toBeInstanceOf(
            ProjectResponseError,
        )
    })

    it.each([
        ['an array', [branch]],
        ['a missing branch ID', { ...branch, id: undefined }],
        ['an empty branch name', { ...branch, name: '' }],
        ['a missing project ID', { ...branch, projectId: undefined }],
        ['a missing head field', { ...branch, headRevisionId: undefined }],
        ['an invalid head field', { ...branch, headRevisionId: 12 }],
        ['an invalid createdAt', { ...branch, createdAt: null }],
        ['an invalid updatedAt', { ...branch, updatedAt: null }],
        ['a different project', { ...branch, projectId: 'different-project' }],
        ['a different starting revision', { ...branch, headRevisionId: 'other-revision' }],
        ['an unexpected empty head', { ...branch, headRevisionId: null }],
    ])('rejects a successful response containing %s', async (_reason, invalidBody) => {
        fetchMock.mockResolvedValueOnce(jsonResponse(invalidBody))

        await expect(createProjectBranch(projectId, input)).rejects.toBeInstanceOf(
            ProjectResponseError,
        )
    })

    it('rejects an unexpected revision when an empty branch was requested', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(branch))

        await expect(
            createProjectBranch(projectId, {
                name: 'design-experiment',
                headRevisionId: null,
            }),
        ).rejects.toBeInstanceOf(ProjectResponseError)
    })

    it('propagates network failures without inventing an HTTP status', async () => {
        const failure = new TypeError('Failed to fetch')

        fetchMock.mockRejectedValueOnce(failure)

        await expect(createProjectBranch(projectId, input)).rejects.toBe(failure)
    })
})
