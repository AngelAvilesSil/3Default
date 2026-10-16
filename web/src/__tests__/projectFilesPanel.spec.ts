import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { listProjectFiles, ProjectFileRequestError, type ProjectFile } from '../api/projectFiles'
import ProjectFilesPanel from '../components/ProjectFilesPanel.vue'

vi.mock('../api/projectFiles', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/projectFiles')>()

    return {
        ...original,
        listProjectFiles: vi.fn<typeof original.listProjectFiles>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const secondProjectId = '22222222-2222-4222-8222-222222222222'

const firstFile: ProjectFile = {
    id: '33333333-3333-4333-8333-333333333333',
    projectId,
    uploadedByUserId: '44444444-4444-4444-8444-444444444444',
    contentSha256: 'a'.repeat(64),
    originalFilename: 'gripper.step',
    mediaType: 'model/step',
    createdAt: '2026-10-15T12:00:00Z',
}

const secondFile: ProjectFile = {
    ...firstFile,
    id: '55555555-5555-4555-8555-555555555555',
    contentSha256: 'b'.repeat(64),
    originalFilename: 'housing.glb',
    mediaType: 'model/gltf-binary',
}

beforeEach(() => {
    vi.resetAllMocks()
})

describe('project files panel', () => {
    it('loads and displays uploaded project files', async () => {
        vi.mocked(listProjectFiles).mockResolvedValueOnce([firstFile, secondFile])

        const wrapper = mount(ProjectFilesPanel, {
            props: { projectId },
        })

        await vi.waitFor(() => {
            expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(2)
        })

        expect(wrapper.text()).toContain('Project files')
        expect(wrapper.text()).toContain('2 files')
        expect(wrapper.text()).toContain('gripper.step')
        expect(wrapper.text()).toContain('housing.glb')
        expect(listProjectFiles).toHaveBeenCalledWith(projectId, expect.any(AbortSignal))

        wrapper.unmount()
    })

    it('shows a loading state while files are being retrieved', async () => {
        vi.mocked(listProjectFiles).mockReturnValueOnce(new Promise<ProjectFile[]>(() => {}))

        const wrapper = mount(ProjectFilesPanel, {
            props: { projectId },
        })

        expect(wrapper.get('[role="status"]').text()).toContain('Loading files')

        wrapper.unmount()
    })

    it('shows an appropriate empty state', async () => {
        vi.mocked(listProjectFiles).mockResolvedValueOnce([])

        const wrapper = mount(ProjectFilesPanel, {
            props: { projectId },
        })

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('No files uploaded yet')
        })

        expect(wrapper.find('ul[aria-label="Project files"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('retries a failed request without leaving the panel', async () => {
        vi.mocked(listProjectFiles)
            .mockRejectedValueOnce(new TypeError('Failed to fetch'))
            .mockResolvedValueOnce([firstFile])

        const wrapper = mount(ProjectFilesPanel, {
            props: { projectId },
        })

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain('Unable to load files')
        })

        await wrapper.get('button').trigger('click')

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('gripper.step')
        })

        expect(listProjectFiles).toHaveBeenCalledTimes(2)
        expect(wrapper.find('[role="alert"]').exists()).toBe(false)

        wrapper.unmount()
    })

    it('reports expired authentication to its parent', async () => {
        vi.mocked(listProjectFiles).mockRejectedValueOnce(
            new ProjectFileRequestError(401, 'Unauthorized'),
        )

        const wrapper = mount(ProjectFilesPanel, {
            props: { projectId },
        })

        await vi.waitFor(() => {
            expect(wrapper.emitted('unauthorized')).toHaveLength(1)
        })

        expect(wrapper.text()).not.toContain('gripper.step')

        wrapper.unmount()
    })

    it('rejects inconsistent project-file records defensively', async () => {
        vi.mocked(listProjectFiles).mockResolvedValueOnce([
            {
                ...firstFile,
                projectId: secondProjectId,
            },
        ])

        const wrapper = mount(ProjectFilesPanel, {
            props: { projectId },
        })

        await vi.waitFor(() => {
            expect(wrapper.get('[role="alert"]').text()).toContain('Unable to load files')
        })

        expect(wrapper.text()).not.toContain('gripper.step')

        wrapper.unmount()
    })

    it('cancels obsolete requests when the selected project changes', async () => {
        let resolveFirst!: (files: ProjectFile[]) => void

        const firstRequest = new Promise<ProjectFile[]>((resolve) => {
            resolveFirst = resolve
        })

        const otherProjectFile: ProjectFile = {
            ...secondFile,
            projectId: secondProjectId,
            originalFilename: 'gearbox.glb',
        }

        vi.mocked(listProjectFiles).mockImplementation((id) =>
            id === projectId ? firstRequest : Promise.resolve([otherProjectFile]),
        )

        const wrapper = mount(ProjectFilesPanel, {
            props: { projectId },
        })

        await vi.waitFor(() => {
            expect(listProjectFiles).toHaveBeenCalledTimes(1)
        })

        const oldSignal = vi.mocked(listProjectFiles).mock.calls[0]?.[1]

        await wrapper.setProps({ projectId: secondProjectId })

        await vi.waitFor(() => {
            expect(wrapper.text()).toContain('gearbox.glb')
        })

        expect(oldSignal?.aborted).toBe(true)

        resolveFirst([firstFile])
        await flushPromises()

        expect(wrapper.text()).toContain('gearbox.glb')
        expect(wrapper.text()).not.toContain('gripper.step')

        wrapper.unmount()
    })

    it('aborts a pending request when unmounted', async () => {
        vi.mocked(listProjectFiles).mockReturnValueOnce(new Promise<ProjectFile[]>(() => {}))

        const wrapper = mount(ProjectFilesPanel, {
            props: { projectId },
        })

        const signal = vi.mocked(listProjectFiles).mock.calls[0]?.[1]

        expect(signal?.aborted).toBe(false)

        wrapper.unmount()

        expect(signal?.aborted).toBe(true)
    })
})
