import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import { type ProjectFile } from '../api/projectFiles'
import { getProject, listProjectBranches, ProjectRequestError, type Project } from '../api/projects'
import {
    getProjectRevision,
    listProjectBranchHistory,
    listProjectRevisionFiles,
    RevisionRequestError,
    type ProjectRevision,
} from '../api/revisions'
import { createAppRouter } from '../router'
import BranchHistoryView from '../views/BranchHistoryView.vue'
import RevisionDetailView from '../views/RevisionDetailView.vue'

vi.mock('../api/auth', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/auth')>()
    return {
        ...original,
        getCurrentUser: vi.fn<typeof original.getCurrentUser>(),
    }
})

vi.mock('../api/projects', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/projects')>()
    return {
        ...original,
        getProject: vi.fn<typeof original.getProject>(),
        listProjectBranches: vi.fn<typeof original.listProjectBranches>(),
    }
})

vi.mock('../api/revisions', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/revisions')>()
    return {
        ...original,
        getProjectRevision: vi.fn<typeof original.getProjectRevision>(),
        listProjectBranchHistory: vi.fn<typeof original.listProjectBranchHistory>(),
        listProjectRevisionFiles: vi.fn<typeof original.listProjectRevisionFiles>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const branchId = '22222222-2222-4222-8222-222222222222'
const revisionId = '33333333-3333-4333-8333-333333333333'
const parentId = '44444444-4444-4444-8444-444444444444'
const mergeParentId = '55555555-5555-4555-8555-555555555555'

const project: Project = {
    id: projectId,
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private',
    createdAt: '2026-10-19T12:00:00Z',
    updatedAt: '2026-10-19T12:00:00Z',
}

const revision: ProjectRevision = {
    id: revisionId,
    projectId,
    authorUserId: '66666666-6666-4666-8666-666666666666',
    message: 'Reinforce gripper housing',
    parentRevisionId: parentId,
    mergeParentRevisionId: null,
    createdAt: '2026-10-19T13:00:00Z',
}

const sourceFile: ProjectFile = {
    id: '77777777-7777-4777-8777-777777777777',
    projectId,
    uploadedByUserId: revision.authorUserId,
    contentSha256: 'a'.repeat(64),
    originalFilename: 'gripper-reinforced.step',
    mediaType: null,
    createdAt: '2026-10-19T12:45:00Z',
}

const anotherFile: ProjectFile = {
    ...sourceFile,
    id: '88888888-8888-4888-8888-888888888888',
    originalFilename: 'mounting-plate.step',
    contentSha256: 'b'.repeat(64),
}

const user = {
    id: revision.authorUserId,
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-10-19T10:00:00Z',
    updatedAt: '2026-10-19T10:00:00Z',
}

let pinia: Pinia

beforeEach(() => {
    vi.resetAllMocks()

    pinia = createPinia()
    setActivePinia(pinia)

    vi.mocked(getCurrentUser).mockResolvedValue({
        status: 'authenticated',
        user,
    })

    vi.mocked(getProject).mockResolvedValue(project)
    vi.mocked(getProjectRevision).mockResolvedValue(revision)
    vi.mocked(listProjectRevisionFiles).mockResolvedValue([sourceFile])
    vi.mocked(listProjectBranchHistory).mockResolvedValue([revision])
})

async function mountDetail(
    fromBranchId: string | null = branchId,
    selectedRevisionId = revisionId,
) {
    const router = createAppRouter(createMemoryHistory())

    await router.push({
        name: 'project-revision-detail',
        params: { projectId, revisionId: selectedRevisionId },
        query: fromBranchId === null ? {} : { fromBranchId },
    })
    await router.isReady()

    const wrapper = mount(RevisionDetailView, {
        props: {
            projectId,
            revisionId: selectedRevisionId,
        },
        global: {
            plugins: [pinia, router],
        },
    })

    return { wrapper, router }
}

describe('revision detail navigation', () => {
    it('registers a protected project-scoped lazy-loaded route', () => {
        const router = createAppRouter(createMemoryHistory())
        const route = router.getRoutes().find((item) => item.name === 'project-revision-detail')

        expect(route).toBeDefined()
        expect(route?.path).toBe('/projects/:projectId/revisions/:revisionId')
        expect(route?.meta.requiresAuth).toBe(true)
        expect(route?.props.default).toBe(true)
        expect(typeof route?.components?.default).toBe('function')
    })

    it('requires login for a direct revision URL', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })

        const router = createAppRouter(createMemoryHistory())
        const path = `/projects/${projectId}/revisions/${revisionId}`

        await router.push(path)

        expect(router.currentRoute.value.name).toBe('login')
        expect(router.currentRoute.value.query.redirect).toBe(path)
    })

    it('links a history entry to its exact project revision', async () => {
        const router = createAppRouter(createMemoryHistory())

        await router.push(`/projects/${projectId}/branches/${branchId}/history`)
        await router.isReady()

        const branch = {
            id: branchId,
            projectId,
            name: 'main',
            headRevisionId: revisionId,
            createdAt: '2026-10-19T12:00:00Z',
            updatedAt: '2026-10-19T13:00:00Z',
        }

        // The branch-history view resolves the selected branch before
        // exposing links to historical revisions.
        vi.mocked(listProjectBranches).mockResolvedValueOnce([branch])

        const wrapper = mount(BranchHistoryView, {
            props: { projectId, branchId },
            global: { plugins: [pinia, router] },
        })

        await vi.waitFor(() => {
            expect(wrapper.findAll('a.branch-history__inspect')).toHaveLength(1)
        })

        const link = wrapper.get('a.branch-history__inspect')
        expect(link.text()).toContain('Inspect revision')

        await link.trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('project-revision-detail')
        })

        expect(router.currentRoute.value.params).toMatchObject({
            projectId,
            revisionId,
        })
        expect(router.currentRoute.value.query.fromBranchId).toBe(branchId)

        expect(listProjectBranches).toHaveBeenCalledWith(projectId, expect.any(AbortSignal))

        wrapper.unmount()
    })

    it('returns to the source branch history when the hint is present', async () => {
        const { wrapper } = await mountDetail()

        const link = wrapper.get('a.revision-detail__back')

        expect(link.text()).toContain('Back to branch history')
        expect(link.attributes('href')).toBe(`/projects/${projectId}/branches/${branchId}/history`)

        wrapper.unmount()
    })

    it('returns to the project when opened without a branch hint', async () => {
        const { wrapper } = await mountDetail(null)

        const link = wrapper.get('a.revision-detail__back')
        expect(link.text()).toContain('Back to project')
        expect(link.attributes('href')).toBe(`/projects/${projectId}`)

        wrapper.unmount()
    })
})

describe('read-only revision and file snapshot', () => {
    it('loads the exact revision and displays its selected file records', async () => {
        vi.mocked(listProjectRevisionFiles).mockResolvedValue([sourceFile, anotherFile])

        const { wrapper } = await mountDetail()

        await vi.waitFor(() => {
            expect(wrapper.findAll('.revision-detail__file')).toHaveLength(2)
        })

        expect(getProject).toHaveBeenCalledWith(projectId, expect.any(AbortSignal))
        expect(getProjectRevision).toHaveBeenCalledWith(
            projectId,
            revisionId,
            expect.any(AbortSignal),
        )
        expect(listProjectRevisionFiles).toHaveBeenCalledWith(
            projectId,
            revisionId,
            expect.any(AbortSignal),
        )

        expect(wrapper.text()).toContain(project.name)
        expect(wrapper.text()).toContain(revision.message)
        const shortId = wrapper.get('.revision-detail__metadata .revision-id__short')

        expect(shortId.text()).toBe(revision.id.slice(0, 8))
        expect(shortId.attributes('title')).toBe(revision.id)
        expect(wrapper.text()).not.toContain(revision.id)
        expect(
            wrapper.get('button[aria-label="Copy full revision ID 33333333"]').attributes('type'),
        ).toBe('button')

        const createBranchLink = wrapper.get('a.revision-detail__create-branch')
        expect(createBranchLink.attributes('href')).toContain(
            `/projects/${projectId}/revisions/${revisionId}/branches/new`,
        )
        expect(createBranchLink.attributes('href')).toContain(`fromBranchId=${branchId}`)

        expect(wrapper.get('.revision-detail__author-name').text()).toBe('Unknown author')

        const authorId = wrapper.get('.user-id__short')
        expect(authorId.text()).toBe(revision.authorUserId.slice(0, 8))
        expect(authorId.attributes('title')).toBe(revision.authorUserId)
        expect(wrapper.text()).not.toContain(revision.authorUserId)
        expect(
            wrapper.get('button[aria-label="Copy full user ID 66666666"]').attributes('type'),
        ).toBe('button')
        expect(wrapper.text()).toContain('Files in this revision (2)')
        expect(wrapper.text()).toContain(sourceFile.originalFilename)
        expect(wrapper.text()).toContain(anotherFile.originalFilename)
        expect(wrapper.text()).toContain(sourceFile.id)
        expect(wrapper.text()).toContain(sourceFile.contentSha256)
        expect(wrapper.text()).toContain('not every file uploaded to the project')
        expect(wrapper.text()).toContain('does not calculate CAD geometry differences')

        wrapper.unmount()
    })

    it('displays the recorded author instead of the signed-in user', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user: {
                ...user,
                id: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
                displayName: 'Signed In Viewer',
            },
        })

        vi.mocked(getProjectRevision).mockResolvedValue({
            ...revision,
            authorDisplayName: 'Engineering Author',
        })

        const { wrapper } = await mountDetail()

        await vi.waitFor(() => {
            expect(wrapper.get('.revision-detail__author-name').text()).toBe('Engineering Author')
        })

        expect(wrapper.text()).not.toContain('Signed In Viewer')
        expect(wrapper.get('.user-id__short').attributes('title')).toBe(revision.authorUserId)
        expect(wrapper.text()).not.toContain(revision.authorUserId)

        wrapper.unmount()
    })

    it('does not invent unselected project files', async () => {
        const { wrapper } = await mountDetail()

        await vi.waitFor(() => {
            expect(wrapper.findAll('.revision-detail__file')).toHaveLength(1)
        })

        expect(wrapper.text()).toContain(sourceFile.originalFilename)
        expect(wrapper.text()).not.toContain(anotherFile.originalFilename)

        wrapper.unmount()
    })

    it('handles an intentionally empty revision snapshot', async () => {
        vi.mocked(listProjectRevisionFiles).mockResolvedValue([])

        const { wrapper } = await mountDetail()

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('This revision contains no source files.')
        })

        expect(wrapper.text()).toContain('Files in this revision (0)')
        expect(wrapper.findAll('.revision-detail__file')).toHaveLength(0)

        wrapper.unmount()
    })

    it('displays exact primary and merge-parent references', async () => {
        vi.mocked(getProjectRevision).mockResolvedValue({
            ...revision,
            mergeParentRevisionId: mergeParentId,
        })

        const { wrapper } = await mountDetail()

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('Merge parent')
        })

        const revisionIds = wrapper.findAll('.revision-detail__section .revision-id__short')

        expect(revisionIds.map((node) => node.text())).toEqual([
            revisionId.slice(0, 8),
            parentId.slice(0, 8),
            mergeParentId.slice(0, 8),
        ])
        expect(revisionIds.map((node) => node.attributes('title'))).toEqual([
            revisionId,
            parentId,
            mergeParentId,
        ])

        expect(wrapper.text()).not.toContain(parentId)
        expect(wrapper.text()).not.toContain(mergeParentId)
        expect(
            wrapper
                .get('button[aria-label="Copy full merge parent revision ID 55555555"]')
                .attributes('type'),
        ).toBe('button')

        expect(wrapper.get('time').attributes('datetime')).toBe(revision.createdAt)

        wrapper.unmount()
    })

    it('represents a root revision without inventing a parent', async () => {
        vi.mocked(getProjectRevision).mockResolvedValue({
            ...revision,
            parentRevisionId: null,
        })

        const { wrapper } = await mountDetail()

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('None (root revision)')
        })

        expect(wrapper.text()).not.toContain('Merge parent')
        expect(wrapper.findAll('.revision-id')).toHaveLength(1)

        wrapper.unmount()
    })

    it('shows loading while project resolution is pending', async () => {
        vi.mocked(getProject).mockReturnValue(new Promise<Project>(() => {}))

        const { wrapper } = await mountDetail()

        expect(wrapper.get('[role="status"]').text()).toContain('Loading revision details')
        expect(getProjectRevision).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('does not expose an inaccessible project', async () => {
        vi.mocked(getProject).mockRejectedValue(new ProjectRequestError(404, 'Not Found'))

        const { wrapper } = await mountDetail()

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain('not accessible')
        })

        expect(getProjectRevision).not.toHaveBeenCalled()
        expect(listProjectRevisionFiles).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('does not expose an inaccessible revision', async () => {
        vi.mocked(getProjectRevision).mockRejectedValue(new RevisionRequestError(404, 'Not Found'))

        const { wrapper } = await mountDetail()

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain('not accessible')
        })

        expect(listProjectRevisionFiles).not.toHaveBeenCalled()
        wrapper.unmount()
    })

    it('rejects a mismatched revision identity', async () => {
        vi.mocked(getProjectRevision).mockResolvedValue({
            ...revision,
            id: 'different-revision',
        })

        const { wrapper } = await mountDetail()

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain(
                'Unable to load revision details',
            )
        })

        expect(wrapper.text()).not.toContain(revision.message)
        expect(listProjectRevisionFiles).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('rejects file records from a different project', async () => {
        vi.mocked(listProjectRevisionFiles).mockResolvedValue([
            { ...sourceFile, projectId: 'another-project' },
        ])

        const { wrapper } = await mountDetail()

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain(
                'Unable to load revision details',
            )
        })

        expect(wrapper.findAll('.revision-detail__file')).toHaveLength(0)

        wrapper.unmount()
    })

    it('redirects to login if snapshot retrieval loses authentication', async () => {
        vi.mocked(listProjectRevisionFiles).mockRejectedValue(
            new RevisionRequestError(401, 'Unauthorized'),
        )

        const { wrapper, router } = await mountDetail()

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('login')
        })

        expect(router.currentRoute.value.query.redirect).toContain(
            `/projects/${projectId}/revisions/${revisionId}`,
        )

        wrapper.unmount()
    })

    it('allows retry after a temporary snapshot API failure', async () => {
        vi.mocked(listProjectRevisionFiles)
            .mockRejectedValueOnce(new RevisionRequestError(503, 'Unavailable'))
            .mockResolvedValueOnce([sourceFile])

        const { wrapper } = await mountDetail()

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain(
                'Unable to load revision details',
            )
        })

        await wrapper.get('button').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.findAll('.revision-detail__file')).toHaveLength(1)
        })

        expect(listProjectRevisionFiles).toHaveBeenCalledTimes(2)
        wrapper.unmount()
    })

    it('ignores an obsolete revision response after parameters change', async () => {
        let resolveOld!: (value: ProjectRevision) => void
        let oldSignal: AbortSignal | undefined

        vi.mocked(getProjectRevision).mockImplementationOnce((_projectId, _revisionId, signal) => {
            oldSignal = signal
            return new Promise<ProjectRevision>((resolve) => {
                resolveOld = resolve
            })
        })

        const { wrapper } = await mountDetail()

        await vi.waitFor(() => {
            expect(getProjectRevision).toHaveBeenCalledTimes(1)
        })

        const newerRevisionId = '99999999-9999-4999-8999-999999999999'

        const newerRevision: ProjectRevision = {
            ...revision,
            id: newerRevisionId,
            message: 'Update mounting plate',
        }

        vi.mocked(getProjectRevision).mockResolvedValueOnce(newerRevision)
        vi.mocked(listProjectRevisionFiles).mockResolvedValueOnce([anotherFile])

        await wrapper.setProps({ revisionId: newerRevisionId })

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('Update mounting plate')
            expect(wrapper.text()).toContain(anotherFile.originalFilename)
        })

        expect(oldSignal?.aborted).toBe(true)

        resolveOld(revision)
        await flushPromises()

        expect(wrapper.text()).toContain('Update mounting plate')
        expect(wrapper.text()).not.toContain(revision.message)
        expect(wrapper.text()).not.toContain(sourceFile.originalFilename)

        wrapper.unmount()
    })

    it('aborts a pending project request when the view unmounts', async () => {
        let observedSignal: AbortSignal | undefined

        vi.mocked(getProject).mockImplementation((_id, signal) => {
            observedSignal = signal
            return new Promise<Project>(() => {})
        })

        const { wrapper } = await mountDetail()

        await vi.waitFor(() => {
            expect(observedSignal).toBeDefined()
        })

        wrapper.unmount()

        expect(observedSignal?.aborted).toBe(true)
    })
})
