import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getCurrentUser } from '../api/auth'
import { getProject, listProjectBranches, type Project, type ProjectBranch } from '../api/projects'
import { createAppRouter } from '../router'
import ProjectOverviewView from '../views/ProjectOverviewView.vue'

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

const project: Project = {
    id: '11111111-1111-4111-8111-111111111111',
    name: 'Robotic Gripper',
    description: 'Mechanical gripper development',
    visibility: 'private',
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:01:00Z',
}

const mainBranch: ProjectBranch = {
    id: '33333333-3333-4333-8333-333333333333',
    projectId: project.id,
    name: 'main',
    headRevisionId: null,
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:00:00Z',
}

const designBranch: ProjectBranch = {
    ...mainBranch,
    id: '44444444-4444-4444-8444-444444444444',
    name: 'gripper-redesign',
    headRevisionId: '55555555-5555-4555-8555-555555555555',
}

const user = {
    id: '66666666-6666-4666-8666-666666666666',
    email: 'person@example.com',
    displayName: 'Person',
    createdAt: '2026-10-11T12:00:00Z',
    updatedAt: '2026-10-11T12:00:00Z',
}

let pinia: Pinia

async function mountOverview(path: string) {
    const router = createAppRouter(createMemoryHistory())

    await router.push(path)
    await router.isReady()

    const wrapper = mount(ProjectOverviewView, {
        global: {
            plugins: [pinia, router],
            stubs: {
                ProjectFileWorkspace: true,
            },
        },
    })

    await vi.waitFor(() => {
        expect(listProjectBranches).toHaveBeenCalledWith(project.id, expect.any(AbortSignal))
    })

    await flushPromises()

    return { wrapper, router }
}

describe('project overview branch selection', () => {
    beforeEach(() => {
        vi.resetAllMocks()

        pinia = createPinia()
        setActivePinia(pinia)

        vi.mocked(getCurrentUser).mockResolvedValue({
            status: 'authenticated',
            user,
        })

        vi.mocked(getProject).mockResolvedValue(project)

        // Deliberately place the non-main branch first.
        // Default selection must not depend on API ordering.
        vi.mocked(listProjectBranches).mockResolvedValue([designBranch, mainBranch])
    })

    it('groups the selected branch controls into one compact toolbar', async () => {
        const { wrapper } = await mountOverview(
            `/projects/${project.id}?branchId=${designBranch.id}`,
        )

        const toolbar = wrapper.get('.project-overview__branch-toolbar')
        const selector = toolbar.get('button[aria-label="Choose design branch"]')

        expect(selector.text()).toContain('gripper-redesign')

        const context = toolbar.get('.project-overview__selection')

        expect(context.text()).toContain('Head revision')
        expect(context.get('.revision-id__short').text()).toBe('55555555')
        expect(context.text()).not.toContain('Selected branch:')

        expect(
            toolbar.findAll('button[aria-label="View history of gripper-redesign"]'),
        ).toHaveLength(1)

        expect(
            toolbar.findAll('button[aria-label="Create revision on gripper-redesign"]'),
        ).toHaveLength(1)

        expect(wrapper.findAll('button[aria-label="Choose design branch"]')).toHaveLength(1)
        expect(wrapper.text()).not.toContain('Selected branch:')

        wrapper.unmount()
    })

    it('selects main by default in a single branch selector', async () => {
        const { wrapper, router } = await mountOverview(`/projects/${project.id}`)

        const selector = wrapper.find('button[aria-label="Choose design branch"]')

        expect(selector.exists()).toBe(true)
        expect(selector.text()).toContain('main')
        expect(selector.text()).not.toContain('gripper-redesign')

        expect(router.currentRoute.value.query.branchId).toBeUndefined()

        wrapper.unmount()
    })

    it('honors an explicit branch URL and routes its history action', async () => {
        const { wrapper, router } = await mountOverview(
            `/projects/${project.id}?branchId=${designBranch.id}`,
        )

        const selector = wrapper.find('button[aria-label="Choose design branch"]')

        expect(selector.exists()).toBe(true)
        expect(selector.text()).toContain('gripper-redesign')
        expect(selector.text()).not.toContain('main')

        const historyActions = wrapper.findAll('button[aria-label^="View history of "]')

        expect(historyActions).toHaveLength(1)

        await wrapper.get('button[aria-label="View history of gripper-redesign"]').trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.name).toBe('project-branch-history')
        })

        expect(router.currentRoute.value.params).toMatchObject({
            projectId: project.id,
            branchId: designBranch.id,
        })

        wrapper.unmount()
    })

    it('does not silently replace an unavailable branch from the URL', async () => {
        const missingId = '99999999-9999-4999-8999-999999999999'

        const { wrapper, router } = await mountOverview(
            `/projects/${project.id}?branchId=${missingId}`,
        )

        const selector = wrapper.get('button[aria-label="Choose design branch"]')

        expect(selector.text()).toContain('Choose branch')
        expect(wrapper.get('[role="alert"]').text()).toContain(
            'The branch in this link is unavailable',
        )
        expect(wrapper.findAll('button[aria-label^="View history of "]')).toHaveLength(0)
        expect(wrapper.findAll('button[aria-label^="Create revision on "]')).toHaveLength(0)
        expect(router.currentRoute.value.query.branchId).toBe(missingId)

        await selector.trigger('click')

        const mainChoice = wrapper
            .get('#project-branch-choices')
            .findAll('button')
            .find((choice) => choice.text().trim() === 'main')

        expect(mainChoice).toBeDefined()

        await mainChoice!.trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.query.branchId).toBe(mainBranch.id)
            expect(selector.text()).toContain('main')
        })

        wrapper.unmount()
    })

    it('restores branch context through browser back and forward navigation', async () => {
        const { wrapper, router } = await mountOverview(`/projects/${project.id}`)

        const selector = wrapper.get('button[aria-label="Choose design branch"]')

        expect(selector.text()).toContain('main')

        await selector.trigger('click')

        const designChoice = wrapper
            .get('#project-branch-choices')
            .findAll('button')
            .find((choice) => choice.text().trim() === 'gripper-redesign')

        expect(designChoice).toBeDefined()

        await designChoice!.trigger('click')

        await vi.waitFor(() => {
            expect(router.currentRoute.value.query.branchId).toBe(designBranch.id)
            expect(selector.text()).toContain('gripper-redesign')
        })

        router.back()

        await vi.waitFor(() => {
            expect(router.currentRoute.value.query.branchId).toBeUndefined()
            expect(selector.text()).toContain('main')
        })

        router.forward()

        await vi.waitFor(() => {
            expect(router.currentRoute.value.query.branchId).toBe(designBranch.id)
            expect(selector.text()).toContain('gripper-redesign')
        })

        expect(getProject).toHaveBeenCalledTimes(1)
        expect(listProjectBranches).toHaveBeenCalledTimes(1)

        wrapper.unmount()
    })

    it('filters branch choices using an accessible search field', async () => {
        const { wrapper } = await mountOverview(`/projects/${project.id}`)

        await wrapper.get('button[aria-label="Choose design branch"]').trigger('click')

        const search = wrapper.find('input[aria-label="Search branches"]')

        // Expected RED: the first selector implementation has no search field.
        expect(search.exists()).toBe(true)

        await search.setValue('redesign')

        expect(
            wrapper
                .get('#project-branch-choices')
                .findAll('button')
                .map((choice) => choice.text().trim()),
        ).toEqual(['gripper-redesign'])

        await search.setValue('does-not-exist')

        expect(wrapper.get('#project-branch-choices').findAll('button')).toHaveLength(0)
        expect(wrapper.text()).toContain('No matching branches')

        wrapper.unmount()
    })

    it('filters case-insensitively and clears search when reopened', async () => {
        const { wrapper } = await mountOverview(`/projects/${project.id}`)

        const trigger = wrapper.get('button[aria-label="Choose design branch"]')

        await trigger.trigger('click')

        const search = wrapper.get('input[aria-label="Search branches"]')

        await search.setValue('  REDESIGN  ')

        expect(
            wrapper
                .get('#project-branch-choices')
                .findAll('button')
                .map((choice) => choice.text().trim()),
        ).toEqual(['gripper-redesign'])

        await trigger.trigger('click')

        expect(wrapper.find('#project-branch-choices').exists()).toBe(false)

        await trigger.trigger('click')

        const reopenedSearch = wrapper.get('input[aria-label="Search branches"]')

        expect((reopenedSearch.element as HTMLInputElement).value).toBe('')
        expect(wrapper.get('#project-branch-choices').findAll('button')).toHaveLength(2)

        wrapper.unmount()
    })

    it('focuses branch search when opening the picker', async () => {
        const { wrapper } = await mountOverview(`/projects/${project.id}`)

        const host = document.createElement('div')
        document.body.appendChild(host)
        host.appendChild(wrapper.element)

        try {
            const trigger = wrapper.get('button[aria-label="Choose design branch"]')

            ;(trigger.element as HTMLButtonElement).focus()

            expect(document.activeElement).toBe(trigger.element)

            await trigger.trigger('click')

            const search = wrapper.get('input[aria-label="Search branches"]')

            // Expected RED: opening currently does not move focus to search.
            await vi.waitFor(() => {
                expect(document.activeElement).toBe(search.element)
            })
        } finally {
            wrapper.unmount()
            host.remove()
        }
    })

    it('closes on Escape and restores focus to the picker trigger', async () => {
        const { wrapper } = await mountOverview(`/projects/${project.id}`)

        const host = document.createElement('div')
        document.body.appendChild(host)
        host.appendChild(wrapper.element)

        try {
            const trigger = wrapper.get('button[aria-label="Choose design branch"]')

            await trigger.trigger('click')

            const search = wrapper.get('input[aria-label="Search branches"]')

            ;(search.element as HTMLInputElement).focus()

            expect(document.activeElement).toBe(search.element)

            await search.trigger('keydown', { key: 'Escape' })

            expect(wrapper.find('#project-branch-choices').exists()).toBe(false)

            // Expected RED: Escape currently closes without restoring focus.
            expect(document.activeElement).toBe(trigger.element)
        } finally {
            wrapper.unmount()
            host.remove()
        }
    })
})
