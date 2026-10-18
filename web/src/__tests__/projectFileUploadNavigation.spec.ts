import { mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { listProjectFiles } from '../api/projectFiles'
import ProjectFilesPanel from '../components/ProjectFilesPanel.vue'
import { createAppRouter } from '../router'

vi.mock('../api/projectFiles', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/projectFiles')>()

    return {
        ...original,
        listProjectFiles: vi.fn<typeof original.listProjectFiles>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'

beforeEach(() => {
    vi.resetAllMocks()
})

describe('project file upload navigation', () => {
    it('registers the upload page as a protected lazy-loaded route', () => {
        const router = createAppRouter(createMemoryHistory())

        const route = router
            .getRoutes()
            .find((candidate) => candidate.name === 'project-file-upload')

        expect(route).toBeDefined()
        expect(route?.path).toBe('/projects/:projectId/files/upload')
        expect(route?.meta.requiresAuth).toBe(true)
        expect(typeof route?.components?.default).toBe('function')
    })

    it('resolves the upload URL for its project', () => {
        const router = createAppRouter(createMemoryHistory())

        const destination = router.resolve({
            name: 'project-file-upload',
            params: { projectId },
        })

        expect(destination.path).toBe(`/projects/${projectId}/files/upload`)
    })

    it('offers Add file even when the project directory is empty', async () => {
        vi.mocked(listProjectFiles).mockResolvedValueOnce([])

        const wrapper = mount(ProjectFilesPanel, {
            props: { projectId },
        })

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('No files uploaded yet')
        })

        const addButton = wrapper.get('button[aria-label="Add file"]')

        expect(addButton.text()).toContain('Add file')

        await addButton.trigger('click')

        expect(wrapper.emitted('add-file')).toEqual([[]])

        wrapper.unmount()
    })

    it('keeps file selection separate from Add file', async () => {
        vi.mocked(listProjectFiles).mockResolvedValueOnce([
            {
                id: '22222222-2222-4222-8222-222222222222',
                projectId,
                uploadedByUserId: '33333333-3333-4333-8333-333333333333',
                contentSha256: 'a'.repeat(64),
                originalFilename: 'gripper.step',
                mediaType: 'model/step',
                createdAt: '2026-10-18T12:00:00Z',
            },
        ])

        const wrapper = mount(ProjectFilesPanel, {
            props: { projectId },
        })

        await vi.waitFor(() => {
            expect(wrapper.findAll('ul[aria-label="Project files"] button')).toHaveLength(1)
        })

        await wrapper.get('button[aria-label="Add file"]').trigger('click')

        expect(wrapper.emitted('add-file')).toEqual([[]])
        expect(wrapper.emitted('select-file')).toBeUndefined()

        wrapper.unmount()
    })
})
