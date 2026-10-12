import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
    createProject,
    listProjects,
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

function jsonResponse(body: unknown, status = 200): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: {
            'Content-Type': 'application/json',
        },
    })
}

describe('projects API client', () => {
    beforeEach(() => {
        fetchMock.mockReset()
        vi.stubGlobal('fetch', fetchMock)
    })

    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('lists projects using the existing cookie-based API', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([project]))

        await expect(listProjects()).resolves.toEqual([project])

        expect(fetchMock).toHaveBeenCalledWith('/api/projects', {
            method: 'GET',
            credentials: 'same-origin',
            headers: {
                Accept: 'application/json',
            },
            signal: undefined,
        })
    })

    it('accepts an empty project list', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([]))

        await expect(listProjects()).resolves.toEqual([])
    })

    it('preserves the project order returned by the backend', async () => {
        const olderProject = {
            ...project,
            id: '22222222-2222-4222-8222-222222222222',
            name: 'Older Project',
            createdAt: '2026-09-01T12:00:00Z',
            updatedAt: '2026-09-01T12:00:00Z',
        }

        fetchMock.mockResolvedValueOnce(jsonResponse([project, olderProject]))

        await expect(listProjects()).resolves.toEqual([project, olderProject])
    })

    it('accepts a null project description', async () => {
        const projectWithoutDescription = {
            ...project,
            description: null,
        }

        fetchMock.mockResolvedValueOnce(jsonResponse([projectWithoutDescription]))

        await expect(listProjects()).resolves.toEqual([projectWithoutDescription])
    })

    it('accepts an omitted optional description', async () => {
        const projectWithoutDescription = {
            id: project.id,
            name: project.name,
            visibility: project.visibility,
            createdAt: project.createdAt,
            updatedAt: project.updatedAt,
        }

        fetchMock.mockResolvedValueOnce(jsonResponse([projectWithoutDescription]))

        await expect(listProjects()).resolves.toEqual([projectWithoutDescription])
    })

    it.each([
        [401, 'Unauthorized'],
        [500, 'Internal Server Error'],
    ])('preserves HTTP %i project-list failures', async (status, statusText) => {
        fetchMock.mockResolvedValueOnce(new Response(null, { status, statusText }))

        const request = listProjects()

        await expect(request).rejects.toMatchObject({
            name: 'ProjectRequestError',
            status,
            statusText,
        })

        await expect(request).rejects.toBeInstanceOf(ProjectRequestError)
    })

    it('rejects an unexpected successful HTTP status', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse([project], 201))

        await expect(listProjects()).rejects.toBeInstanceOf(ProjectResponseError)
    })

    it('rejects invalid JSON in a successful response', async () => {
        fetchMock.mockResolvedValueOnce(
            new Response('not valid JSON', {
                status: 200,
                headers: {
                    'Content-Type': 'application/json',
                },
            }),
        )

        await expect(listProjects()).rejects.toBeInstanceOf(ProjectResponseError)
    })

    it('rejects a response that is not an array', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(project))

        await expect(listProjects()).rejects.toBeInstanceOf(ProjectResponseError)
    })

    it.each([
        ['missing project ID', [{ ...project, id: undefined }]],
        ['missing project name', [{ ...project, name: undefined }]],
        ['invalid visibility', [{ ...project, visibility: 'shared' }]],
        ['invalid description', [{ ...project, description: 42 }]],
        ['invalid creation timestamp', [{ ...project, createdAt: null }]],
    ])('rejects a project with %s', async (_reason, body) => {
        fetchMock.mockResolvedValueOnce(jsonResponse(body))

        await expect(listProjects()).rejects.toBeInstanceOf(ProjectResponseError)
    })

    it('passes request cancellation to fetch', async () => {
        const controller = new AbortController()

        fetchMock.mockResolvedValueOnce(jsonResponse([]))

        await listProjects(controller.signal)

        expect(fetchMock).toHaveBeenCalledWith(
            '/api/projects',
            expect.objectContaining({
                signal: controller.signal,
            }),
        )
    })

    it('propagates network failures without changing their meaning', async () => {
        const failure = new TypeError('Failed to fetch')

        fetchMock.mockRejectedValueOnce(failure)

        await expect(listProjects()).rejects.toBe(failure)
    })

    it('creates a project with a name and description', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(project, 201))

        await expect(
            createProject({
                name: 'Robotic Gripper',
                description: 'Mechanical gripper development',
            }),
        ).resolves.toEqual(project)

        expect(fetchMock).toHaveBeenCalledWith('/api/projects', {
            method: 'POST',
            credentials: 'same-origin',
            headers: {
                Accept: 'application/json',
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                name: 'Robotic Gripper',
                description: 'Mechanical gripper development',
            }),
            signal: undefined,
        })
    })

    it('omits the optional description when creating a project', async () => {
        const createdProject = {
            ...project,
            description: null,
        }

        fetchMock.mockResolvedValueOnce(jsonResponse(createdProject, 201))

        await expect(createProject({ name: 'Robotic Gripper' })).resolves.toEqual(createdProject)

        expect(fetchMock).toHaveBeenCalledWith(
            '/api/projects',
            expect.objectContaining({
                body: JSON.stringify({
                    name: 'Robotic Gripper',
                }),
            }),
        )
    })

    it.each([
        [400, 'Bad Request'],
        [401, 'Unauthorized'],
        [409, 'Conflict'],
        [500, 'Internal Server Error'],
    ])('preserves HTTP %i project-creation failures', async (status, statusText) => {
        fetchMock.mockResolvedValueOnce(new Response(null, { status, statusText }))

        await expect(createProject({ name: 'Robotic Gripper' })).rejects.toMatchObject({
            name: 'ProjectRequestError',
            status,
            statusText,
        })
    })

    it('rejects an unexpected successful creation status', async () => {
        fetchMock.mockResolvedValueOnce(jsonResponse(project, 200))

        await expect(createProject({ name: 'Robotic Gripper' })).rejects.toBeInstanceOf(
            ProjectResponseError,
        )
    })

    it('rejects invalid JSON in a successful creation response', async () => {
        fetchMock.mockResolvedValueOnce(
            new Response('not valid JSON', {
                status: 201,
                headers: {
                    'Content-Type': 'application/json',
                },
            }),
        )

        await expect(createProject({ name: 'Robotic Gripper' })).rejects.toBeInstanceOf(
            ProjectResponseError,
        )
    })

    it.each([
        ['an array', [project]],
        ['a missing ID', { ...project, id: undefined }],
        ['invalid visibility', { ...project, visibility: 'shared' }],
    ])('rejects a creation response containing %s', async (_reason, body) => {
        fetchMock.mockResolvedValueOnce(jsonResponse(body, 201))

        await expect(createProject({ name: 'Robotic Gripper' })).rejects.toBeInstanceOf(
            ProjectResponseError,
        )
    })

    it('passes project-creation cancellation to fetch', async () => {
        const controller = new AbortController()
        fetchMock.mockResolvedValueOnce(jsonResponse(project, 201))

        await createProject({ name: 'Robotic Gripper' }, controller.signal)

        expect(fetchMock).toHaveBeenCalledWith(
            '/api/projects',
            expect.objectContaining({
                signal: controller.signal,
            }),
        )
    })

    it('preserves project-creation network failures', async () => {
        const failure = new TypeError('Failed to fetch')
        fetchMock.mockRejectedValueOnce(failure)

        await expect(createProject({ name: 'Robotic Gripper' })).rejects.toBe(failure)
    })
})
