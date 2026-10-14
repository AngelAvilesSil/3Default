import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
    getProject,
    listProjectBranches,
    ProjectRequestError,
    ProjectResponseError,
} from '../api/projects'

const fetchMock = vi.fn<typeof fetch>()

const project = {
    id: '11111111-1111-4111-8111-111111111111',
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private',
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:01:00Z',
}

const mainBranch = {
    id: '22222222-2222-4222-8222-222222222222',
    projectId: project.id,
    name: 'main',
    headRevisionId: null,
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:00:00Z',
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

describe('project detail API', () => {
    it('retrieves one project using same-origin authentication', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(project))

        await expect(getProject(project.id)).resolves.toEqual(project)

        expect(fetchMock).toHaveBeenCalledWith(`/api/projects/${project.id}`, {
            method: 'GET',
            credentials: 'same-origin',
            headers: {
                Accept: 'application/json',
            },
            signal: undefined,
        })
    })

    it('accepts a null description', async () => {
        const projectWithoutDescription = {
            ...project,
            description: null,
        }

        fetchMock.mockResolvedValueOnce(jsonResponse(projectWithoutDescription))

        await expect(getProject(project.id)).resolves.toEqual(projectWithoutDescription)
    })

    it.each([400, 401, 404, 500])('preserves HTTP %i project-detail errors', async (status) => {
        fetchMock.mockResolvedValueOnce(
            new Response(null, {
                status,
                statusText: 'Request failed',
            }),
        )

        await expect(getProject(project.id)).rejects.toMatchObject({
            name: 'ProjectRequestError',
            status,
        })
    })

    it('rejects an unexpected successful status', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(project, 201))

        await expect(getProject(project.id)).rejects.toBeInstanceOf(ProjectResponseError)
    })

    it('rejects invalid JSON', async () => {
        fetchMock.mockResolvedValueOnce(new Response('invalid JSON', { status: 200 }))

        await expect(getProject(project.id)).rejects.toBeInstanceOf(ProjectResponseError)
    })

    it('rejects an invalid project response', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse({ ...project, name: null }))

        await expect(getProject(project.id)).rejects.toBeInstanceOf(ProjectResponseError)
    })

    it('passes request cancellation to fetch', async () => {
        const controller = new AbortController()

        fetchMock.mockResolvedValueOnce(jsonResponse(project))

        await getProject(project.id, controller.signal)

        expect(fetchMock).toHaveBeenCalledWith(
            `/api/projects/${project.id}`,
            expect.objectContaining({
                signal: controller.signal,
            }),
        )
    })

    it('propagates network errors', async () => {
        const error = new TypeError('Failed to fetch')
        fetchMock.mockRejectedValueOnce(error)

        await expect(getProject(project.id)).rejects.toBe(error)
    })
})

describe('project branch listing API', () => {
    it('retrieves branches using same-origin authentication', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([mainBranch]))

        await expect(listProjectBranches(project.id)).resolves.toEqual([mainBranch])

        expect(fetchMock).toHaveBeenCalledWith(`/api/projects/${project.id}/branches`, {
            method: 'GET',
            credentials: 'same-origin',
            headers: {
                Accept: 'application/json',
            },
            signal: undefined,
        })
    })

    it('accepts a project without branches', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([]))

        await expect(listProjectBranches(project.id)).resolves.toEqual([])
    })

    it('preserves branch order and nullable head revisions', async () => {
        const featureBranch = {
            ...mainBranch,
            id: '33333333-3333-4333-8333-333333333333',
            name: 'gripper-redesign',
            headRevisionId: '44444444-4444-4444-8444-444444444444',
        }

        fetchMock.mockResolvedValueOnce(jsonResponse([mainBranch, featureBranch]))

        await expect(listProjectBranches(project.id)).resolves.toEqual([mainBranch, featureBranch])
    })

    it.each([400, 401, 404, 500])('preserves HTTP %i branch-list errors', async (status) => {
        fetchMock.mockResolvedValueOnce(
            new Response(null, {
                status,
                statusText: 'Request failed',
            }),
        )

        await expect(listProjectBranches(project.id)).rejects.toBeInstanceOf(ProjectRequestError)
    })

    it('rejects unexpected successful statuses', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([], 201))

        await expect(listProjectBranches(project.id)).rejects.toBeInstanceOf(ProjectResponseError)
    })

    it('rejects malformed JSON', async () => {
        fetchMock.mockResolvedValueOnce(new Response('invalid JSON', { status: 200 }))

        await expect(listProjectBranches(project.id)).rejects.toBeInstanceOf(ProjectResponseError)
    })

    it('requires an array of branches', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(mainBranch))

        await expect(listProjectBranches(project.id)).rejects.toBeInstanceOf(ProjectResponseError)
    })

    it.each([
        ['missing branch ID', [{ ...mainBranch, id: undefined }]],
        ['missing project ID', [{ ...mainBranch, projectId: undefined }]],
        ['blank branch name', [{ ...mainBranch, name: ' ' }]],
        ['missing head revision', [{ ...mainBranch, headRevisionId: undefined }]],
        ['invalid head revision', [{ ...mainBranch, headRevisionId: 123 }]],
        ['missing creation time', [{ ...mainBranch, createdAt: undefined }]],
        ['missing update time', [{ ...mainBranch, updatedAt: undefined }]],
    ])('rejects branches with %s', async (_reason, body) => {
        fetchMock.mockResolvedValueOnce(jsonResponse(body))

        await expect(listProjectBranches(project.id)).rejects.toBeInstanceOf(ProjectResponseError)
    })

    it('passes cancellation signals to fetch', async () => {
        const controller = new AbortController()

        fetchMock.mockResolvedValueOnce(jsonResponse([]))

        await listProjectBranches(project.id, controller.signal)

        expect(fetchMock).toHaveBeenCalledWith(
            `/api/projects/${project.id}/branches`,
            expect.objectContaining({
                signal: controller.signal,
            }),
        )
    })

    it('propagates network failures', async () => {
        const error = new TypeError('Failed to fetch')
        fetchMock.mockRejectedValueOnce(error)

        await expect(listProjectBranches(project.id)).rejects.toBe(error)
    })
})
