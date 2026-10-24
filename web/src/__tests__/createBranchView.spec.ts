import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import {
    createProjectBranch,
    getProject,
    ProjectRequestError,
    ProjectResponseError,
    type Project,
    type ProjectBranch,
} from '../api/projects'
import {
    getProjectRevision,
    listProjectRevisionFiles,
    RevisionRequestError,
    type ProjectRevision,
} from '../api/revisions'
import { createAppRouter } from '../router'
import CreateBranchView from '../views/CreateBranchView.vue'
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
        createProjectBranch: vi.fn<typeof original.createProjectBranch>(),
    }
})

vi.mock('../api/revisions', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/revisions')>()
    return {
        ...original,
        getProjectRevision: vi.fn<typeof original.getProjectRevision>(),
        listProjectRevisionFiles: vi.fn<typeof original.listProjectRevisionFiles>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const sourceBranchId = '22222222-2222-4222-8222-222222222222'
const revisionId = '33333333-3333-4333-8333-333333333333'
const newBranchId = '44444444-4444-4444-8444-444444444444'

const project: Project = {
    id: projectId,
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private',
    createdAt: '2026-10-22T12:00:00Z',
    updatedAt: '2026-10-22T12:00:00Z',
}

const revision: ProjectRevision = {
    id: revisionId,
    projectId,
    authorUserId: '55555555-5555-4555-8555-555555555555',
    message: 'Reinforce gripper housing',
    parentRevisionId: null,
    mergeParentRevisionId: null,
    createdAt: '2026-10-22T13:00:00Z',
}

const createdBranch: ProjectBranch = {
    id: newBranchId,
    projectId,
    name: 'lightweight-design',
    headRevisionId: revisionId,
    createdAt: '2026-10-23T12:00:00Z',
    updatedAt: '2026-10-23T12:00:00Z',
}

const user = {
    id: revision.authorUserId,
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-10-22T10:00:00Z',
    updatedAt: '2026-10-22T10:00:00Z',
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
    vi.mocked(listProjectRevisionFiles).mockResolvedValue([])
})

async function renderPage(
    selectedRevisionId = revisionId,
    withSourceBranch = true,
    waitForReady = true,
) {
    const router = createAppRouter(createMemoryHistory())

    await router.push({
        name: 'create-project-branch',
        params: { projectId, revisionId: selectedRevisionId },
        query: withSourceBranch ? { fromBranchId: sourceBranchId } : {},
    })

    await router.isReady()

    const wrapper = mount(CreateBranchView, {
        props: { projectId, revisionId: selectedRevisionId },
        global: { plugins: [pinia, router] },
    })

    if (waitForReady) {
        await vi.waitFor(() => wrapper.get('#branch-name'))
    }

    return { wrapper, router }
}

describe('Create Branch navigation', () => {
    it('registers a protected, lazy-loaded, project-scoped route', () => {
        const router = createAppRouter(createMemoryHistory())
        const target = router
            .getRoutes()
            .find((candidate) => candidate.name === 'create-project-branch')

        expect(target).toBeDefined()
        expect(target?.path).toBe('/projects/:projectId/revisions/:revisionId/branches/new')
        expect(target?.meta.requiresAuth).toBe(true)
        expect(target?.props.default).toBe(true)
        expect(typeof target?.components?.default).toBe('function')
    })

    it('requires login when opening the URL directly without a session', async () => {
        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'unauthenticated',
        })

        const router = createAppRouter(createMemoryHistory())
        const path = `/projects/${projectId}/revisions/${revisionId}/branches/new`

        await router.push(path)

        expect(router.currentRoute.value.name).toBe('login')
        expect(router.currentRoute.value.query.redirect).toBe(path)
    })

    it('links from exact Revision Detail and preserves return context', async () => {
        const router = createAppRouter(createMemoryHistory())

        await router.push({
            name: 'project-revision-detail',
            params: { projectId, revisionId },
            query: { fromBranchId: sourceBranchId },
        })
        await router.isReady()

        const wrapper = mount(RevisionDetailView, {
            props: { projectId, revisionId },
            global: { plugins: [pinia, router] },
        })

        await vi.waitFor(() => {
            expect(wrapper.find('a.revision-detail__create-branch').exists()).toBe(true)
        })

        const link = wrapper.get('a.revision-detail__create-branch')

        expect(link.text()).toContain('Create branch from this revision')
        expect(link.attributes('href')).toContain(
            `/projects/${projectId}/revisions/${revisionId}/branches/new`,
        )

        await link.trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('create-project-branch')
        })

        expect(router.currentRoute.value.params).toMatchObject({
            projectId,
            revisionId,
        })
        expect(router.currentRoute.value.query.fromBranchId).toBe(sourceBranchId)

        wrapper.unmount()
    })

    it('returns to the exact revision when Cancel is selected', async () => {
        const { wrapper, router } = await renderPage()

        await wrapper.get('button[type="button"]').trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('project-revision-detail')
        })

        expect(router.currentRoute.value.params.revisionId).toBe(revisionId)
        expect(router.currentRoute.value.query.fromBranchId).toBe(sourceBranchId)
        expect(createProjectBranch).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('supports a direct link without inventing a source branch', async () => {
        const { wrapper } = await renderPage(revisionId, false)

        expect(wrapper.get('a.create-branch__back').attributes('href')).toBe(
            `/projects/${projectId}/revisions/${revisionId}`,
        )

        wrapper.unmount()
    })
})

describe('Create Branch context', () => {
    it('displays a read-only exact starting revision and clear storage semantics', async () => {
        const { wrapper } = await renderPage()

        expect(getProject).toHaveBeenCalledWith(projectId, expect.any(AbortSignal))
        expect(getProjectRevision).toHaveBeenCalledWith(
            projectId,
            revisionId,
            expect.any(AbortSignal),
        )

        expect(wrapper.text()).toContain(project.name)
        expect(wrapper.text()).toContain(revision.message)
        expect(wrapper.text()).toContain(revision.id.slice(0, 8))
        expect(wrapper.text()).toContain('does not duplicate')
        expect(wrapper.get('details code').text()).toBe(revisionId)

        expect(wrapper.get('#branch-name').attributes('required')).toBeDefined()
        expect(wrapper.find('input[name="headRevisionId"]').exists()).toBe(false)
        expect(createProjectBranch).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('does not show a form before the project has loaded', async () => {
        vi.mocked(getProject).mockReturnValueOnce(new Promise<Project>(() => {}))

        const { wrapper } = await renderPage(revisionId, true, false)

        expect(wrapper.get('[role="status"]').text()).toContain('Loading starting revision')
        expect(wrapper.find('form').exists()).toBe(false)

        const signal = vi.mocked(getProject).mock.calls[0]?.[1]
        wrapper.unmount()
        expect(signal?.aborted).toBe(true)
    })

    it('does not expose a form when the starting revision is inaccessible', async () => {
        vi.mocked(getProjectRevision).mockRejectedValueOnce(
            new RevisionRequestError(404, 'Not Found'),
        )

        const { wrapper } = await renderPage(revisionId, true, false)

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain('not accessible')
        })

        expect(wrapper.find('form').exists()).toBe(false)
        expect(createProjectBranch).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('rejects a revision response from another project', async () => {
        vi.mocked(getProjectRevision).mockResolvedValueOnce({
            ...revision,
            projectId: 'another-project',
        })

        const { wrapper } = await renderPage(revisionId, true, false)

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain(
                'Unable to load the starting revision',
            )
        })

        expect(wrapper.find('form').exists()).toBe(false)
        wrapper.unmount()
    })

    it('retries a recoverable context-loading error', async () => {
        vi.mocked(getProjectRevision)
            .mockRejectedValueOnce(new RevisionRequestError(503, 'Unavailable'))
            .mockResolvedValueOnce(revision)

        const { wrapper } = await renderPage(revisionId, true, false)

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain(
                'Unable to load the starting revision',
            )
        })

        await wrapper.get('button').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.find('#branch-name').exists()).toBe(true)
        })

        expect(getProjectRevision).toHaveBeenCalledTimes(2)
        wrapper.unmount()
    })

    it('redirects to login when context loading returns HTTP 401', async () => {
        vi.mocked(getProjectRevision).mockRejectedValueOnce(
            new RevisionRequestError(401, 'Unauthorized'),
        )

        const { wrapper, router } = await renderPage(revisionId, true, false)

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('login')
        })

        expect(router.currentRoute.value.query.redirect).toContain(
            `/projects/${projectId}/revisions/${revisionId}/branches/new`,
        )

        wrapper.unmount()
    })
})

describe('Create Branch submission', () => {
    it('rejects a blank or whitespace-only branch name', async () => {
        const { wrapper } = await renderPage()

        await wrapper.get('form').trigger('submit')

        expect(wrapper.get('#branch-name-error').text()).toContain('Branch name is required')

        await wrapper.get('#branch-name').setValue('   ')
        await wrapper.get('form').trigger('submit')

        expect(wrapper.get('#branch-name-error').text()).toContain('Branch name is required')
        expect(createProjectBranch).not.toHaveBeenCalled()

        wrapper.unmount()
    })

    it('submits a trimmed name with the full exact revision ID and returns to project', async () => {
        vi.mocked(createProjectBranch).mockResolvedValueOnce(createdBranch)

        const { wrapper, router } = await renderPage()

        await wrapper.get('#branch-name').setValue('  lightweight-design  ')
        await wrapper.get('form').trigger('submit')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('project-overview')
        })

        expect(createProjectBranch).toHaveBeenCalledExactlyOnceWith(
            projectId,
            {
                name: 'lightweight-design',
                headRevisionId: revisionId,
            },
            expect.any(AbortSignal),
        )

        expect(router.currentRoute.value.params.projectId).toBe(projectId)

        wrapper.unmount()
    })

    it('allows correction after a confirmed duplicate-name conflict', async () => {
        vi.mocked(createProjectBranch)
            .mockRejectedValueOnce(new ProjectRequestError(409, 'Conflict'))
            .mockResolvedValueOnce(createdBranch)

        const { wrapper, router } = await renderPage()

        await wrapper.get('#branch-name').setValue('existing-name')
        await wrapper.get('form').trigger('submit')

        await vi.waitFor(() => {
            expect(wrapper.get('#branch-name-error').text()).toContain('already exists')
        })

        expect(createProjectBranch).toHaveBeenCalledTimes(1)
        expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeUndefined()

        await wrapper.get('#branch-name').setValue('lightweight-design')

        expect(wrapper.find('#branch-name-error').exists()).toBe(false)

        await wrapper.get('form').trigger('submit')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('project-overview')
        })

        expect(createProjectBranch).toHaveBeenCalledTimes(2)

        wrapper.unmount()
    })

    it('permits deliberate correction after HTTP 400', async () => {
        vi.mocked(createProjectBranch)
            .mockRejectedValueOnce(new ProjectRequestError(400, 'Bad Request'))
            .mockResolvedValueOnce(createdBranch)

        const { wrapper, router } = await renderPage()

        await wrapper.get('#branch-name').setValue('invalid-name')
        await wrapper.get('form').trigger('submit')

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain('branch name was not accepted')
        })

        await wrapper.get('#branch-name').setValue('lightweight-design')
        await wrapper.get('form').trigger('submit')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('project-overview')
        })

        expect(createProjectBranch).toHaveBeenCalledTimes(2)
        wrapper.unmount()
    })

    it('redirects to login after a confirmed HTTP 401 response', async () => {
        vi.mocked(createProjectBranch).mockRejectedValueOnce(
            new ProjectRequestError(401, 'Unauthorized'),
        )

        const { wrapper, router } = await renderPage()

        await wrapper.get('#branch-name').setValue('lightweight-design')
        await wrapper.get('form').trigger('submit')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('login')
        })

        expect(router.currentRoute.value.query.redirect).toContain(
            `/projects/${projectId}/revisions/${revisionId}/branches/new`,
        )

        expect(createProjectBranch).toHaveBeenCalledTimes(1)
        wrapper.unmount()
    })

    it.each([
        [403, 'Forbidden'],
        [404, 'Not Found'],
    ])('blocks resubmission after confirmed HTTP %i', async (status, statusText) => {
        vi.mocked(createProjectBranch).mockRejectedValueOnce(
            new ProjectRequestError(status, statusText),
        )

        const { wrapper, router } = await renderPage()

        await wrapper.get('#branch-name').setValue('lightweight-design')
        await wrapper.get('form').trigger('submit')

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain('no longer available')
        })

        expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
        expect(wrapper.get('button[type="button"]').text()).toContain('Review project')

        await wrapper.get('form').trigger('submit')
        expect(createProjectBranch).toHaveBeenCalledTimes(1)

        await wrapper.get('button[type="button"]').trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('project-overview')
        })

        wrapper.unmount()
    })

    it.each([
        ['network failure', new TypeError('Failed to fetch')],
        ['HTTP 500', new ProjectRequestError(500, 'Server Error')],
        ['invalid success response', new ProjectResponseError('Invalid branch response')],
    ])('blocks resubmission after an uncertain result: %s', async (_reason, failure) => {
        vi.mocked(createProjectBranch).mockRejectedValueOnce(failure)

        const { wrapper } = await renderPage()

        await wrapper.get('#branch-name').setValue('lightweight-design')
        await wrapper.get('form').trigger('submit')

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain('could not be confirmed')
        })

        expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()

        await wrapper.get('form').trigger('submit')

        expect(createProjectBranch).toHaveBeenCalledTimes(1)
        wrapper.unmount()
    })

    it('blocks a successful response containing an unexpected branch name', async () => {
        vi.mocked(createProjectBranch).mockResolvedValueOnce({
            ...createdBranch,
            name: 'other-name',
        })

        const { wrapper } = await renderPage()

        await wrapper.get('#branch-name').setValue('lightweight-design')
        await wrapper.get('form').trigger('submit')

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain('response was inconsistent')
        })

        expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
        expect(createProjectBranch).toHaveBeenCalledTimes(1)

        wrapper.unmount()
    })

    it('prevents duplicate POSTs while a creation request is pending', async () => {
        let resolveCreation!: (value: ProjectBranch) => void

        vi.mocked(createProjectBranch).mockReturnValueOnce(
            new Promise<ProjectBranch>((resolve) => {
                resolveCreation = resolve
            }),
        )

        const { wrapper, router } = await renderPage()

        await wrapper.get('#branch-name').setValue('lightweight-design')
        await wrapper.get('form').trigger('submit')
        await wrapper.get('form').trigger('submit')

        expect(createProjectBranch).toHaveBeenCalledTimes(1)
        expect(wrapper.get('#branch-name').attributes('disabled')).toBeDefined()
        expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
        expect(wrapper.get('button[type="button"]').attributes('disabled')).toBeDefined()

        resolveCreation(createdBranch)

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('project-overview')
        })

        wrapper.unmount()
    })

    it('aborts a pending POST and ignores a late result after unmount', async () => {
        let resolveCreation!: (value: ProjectBranch) => void

        vi.mocked(createProjectBranch).mockReturnValueOnce(
            new Promise<ProjectBranch>((resolve) => {
                resolveCreation = resolve
            }),
        )

        const { wrapper, router } = await renderPage()
        const navigation = vi.spyOn(router, 'push')

        await wrapper.get('#branch-name').setValue('lightweight-design')
        await wrapper.get('form').trigger('submit')

        const signal = vi.mocked(createProjectBranch).mock.calls[0]?.[2]

        expect(signal).toBeInstanceOf(AbortSignal)
        expect(signal?.aborted).toBe(false)

        wrapper.unmount()
        expect(signal?.aborted).toBe(true)

        resolveCreation(createdBranch)
        await flushPromises()

        expect(navigation).not.toHaveBeenCalledWith({
            name: 'project-overview',
            params: { projectId },
        })
    })

    it('ignores obsolete loading responses after a revision change', async () => {
        const newerRevisionId = '66666666-6666-4666-8666-666666666666'

        let resolveOldProject!: (value: Project) => void
        let oldSignal: AbortSignal | undefined

        vi.mocked(getProject).mockImplementationOnce((_id, signal) => {
            oldSignal = signal
            return new Promise<Project>((resolve) => {
                resolveOldProject = resolve
            })
        })

        vi.mocked(getProjectRevision).mockImplementation(async (_projectId, id) => ({
            ...revision,
            id,
        }))

        const { wrapper } = await renderPage(revisionId, true, false)

        await vi.waitFor(() => {
            expect(getProject).toHaveBeenCalledTimes(1)
        })

        await wrapper.setProps({ revisionId: newerRevisionId })

        await vi.waitFor(() => {
            expect(wrapper.get('details code').text()).toBe(newerRevisionId)
        })

        expect(oldSignal?.aborted).toBe(true)

        resolveOldProject(project)
        await flushPromises()

        expect(wrapper.get('details code').text()).toBe(newerRevisionId)
        expect(createProjectBranch).not.toHaveBeenCalled()

        wrapper.unmount()
    })
})
