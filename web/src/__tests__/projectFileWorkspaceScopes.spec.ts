import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { listProjectFiles, type ProjectFile } from '../api/projectFiles'
import { listProjectRevisionFiles } from '../api/revisions'
import ProjectFileWorkspace from '../components/ProjectFileWorkspace.vue'

vi.mock('../api/projectFiles', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/projectFiles')>()

    return {
        ...original,
        listProjectFiles: vi.fn<typeof original.listProjectFiles>(),
    }
})

vi.mock('../api/revisions', async (importOriginal) => {
    const original = await importOriginal<typeof import('../api/revisions')>()

    return {
        ...original,
        listProjectRevisionFiles: vi.fn<typeof original.listProjectRevisionFiles>(),
    }
})

const projectId = '11111111-1111-4111-8111-111111111111'
const branchId = '22222222-2222-4222-8222-222222222222'
const headRevisionId = '33333333-3333-4333-8333-333333333333'

const committedFile: ProjectFile = {
    id: '44444444-4444-4444-8444-444444444444',
    projectId,
    uploadedByUserId: '55555555-5555-4555-8555-555555555555',
    contentSha256: 'a'.repeat(64),
    originalFilename: 'committed-gripper.step',
    mediaType: 'model/step',
    createdAt: '2026-10-21T12:00:00Z',
}

const uploadedOnlyFile: ProjectFile = {
    ...committedFile,
    id: '66666666-6666-4666-8666-666666666666',
    contentSha256: 'b'.repeat(64),
    originalFilename: 'uploaded-only.step',
}

describe('project file workspace scopes', () => {
    beforeEach(() => {
        vi.resetAllMocks()

        vi.mocked(listProjectFiles).mockResolvedValue([committedFile, uploadedOnlyFile])
        vi.mocked(listProjectRevisionFiles).mockResolvedValue([committedFile])
    })

    it('closes the selected preview whenever file scope changes', async () => {
        const wrapper = mount(ProjectFileWorkspace, {
            props: { projectId, branchId, headRevisionId },
            global: {
                stubs: { ProjectFilePreview: true },
            },
        })

        try {
            await vi.waitFor(() => {
                expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(1)
            })

            await wrapper.get('ul[aria-label="Project files"] button').trigger('click')

            expect(wrapper.find('.project-file-workspace__preview').exists()).toBe(true)

            await wrapper.get('button[aria-label="All uploaded files"]').trigger('click')

            expect(wrapper.find('.project-file-workspace__preview').exists()).toBe(false)

            await vi.waitFor(() => {
                expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(2)
            })

            const uploadedButtons = wrapper.findAll('ul[aria-label="Project files"] button')
            await uploadedButtons[1]!.trigger('click')

            expect(wrapper.find('.project-file-workspace__preview').exists()).toBe(true)

            await wrapper.get('button[aria-label="Current revision"]').trigger('click')

            expect(wrapper.find('.project-file-workspace__preview').exists()).toBe(false)
        } finally {
            wrapper.unmount()
        }
    })

    it('resets to current revision and clears preview when the branch changes', async () => {
        const nextBranchId = '77777777-7777-4777-8777-777777777777'
        const nextRevisionId = '88888888-8888-4888-8888-888888888888'

        vi.mocked(listProjectRevisionFiles).mockImplementation(async (_id, revisionId) =>
            revisionId === nextRevisionId ? [uploadedOnlyFile] : [committedFile],
        )

        const wrapper = mount(ProjectFileWorkspace, {
            props: { projectId, branchId, headRevisionId },
            global: {
                stubs: { ProjectFilePreview: true },
            },
        })

        try {
            await vi.waitFor(() => {
                expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(1)
            })

            await wrapper.get('button[aria-label="All uploaded files"]').trigger('click')

            await vi.waitFor(() => {
                expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(2)
            })

            const uploadedButtons = wrapper.findAll('ul[aria-label="Project files"] button')
            await uploadedButtons[1]!.trigger('click')

            expect(wrapper.find('.project-file-workspace__preview').exists()).toBe(true)

            await wrapper.setProps({
                branchId: nextBranchId,
                headRevisionId: nextRevisionId,
            })

            await vi.waitFor(() => {
                expect(listProjectRevisionFiles).toHaveBeenCalledWith(
                    projectId,
                    nextRevisionId,
                    expect.any(AbortSignal),
                )
                expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(1)
                expect(wrapper.text()).toContain('uploaded-only.step')
            })

            expect(
                wrapper.get('button[aria-label="Current revision"]').attributes('aria-pressed'),
            ).toBe('true')
            expect(
                wrapper.get('button[aria-label="All uploaded files"]').attributes('aria-pressed'),
            ).toBe('false')
            expect(wrapper.find('.project-file-workspace__preview').exists()).toBe(false)
            expect(wrapper.text()).not.toContain('committed-gripper.step')
        } finally {
            wrapper.unmount()
        }
    })

    it('clears the old preview when the current branch head changes', async () => {
        const nextRevisionId = '88888888-8888-4888-8888-888888888888'

        vi.mocked(listProjectRevisionFiles).mockImplementation(async (_id, revisionId) =>
            revisionId === nextRevisionId ? [uploadedOnlyFile] : [committedFile],
        )

        const wrapper = mount(ProjectFileWorkspace, {
            props: { projectId, branchId, headRevisionId },
            global: {
                stubs: { ProjectFilePreview: true },
            },
        })

        try {
            await vi.waitFor(() => {
                expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(1)
            })

            await wrapper.get('ul[aria-label="Project files"] button').trigger('click')

            expect(wrapper.find('.project-file-workspace__preview').exists()).toBe(true)

            await wrapper.setProps({ headRevisionId: nextRevisionId })

            await vi.waitFor(() => {
                expect(listProjectRevisionFiles).toHaveBeenCalledWith(
                    projectId,
                    nextRevisionId,
                    expect.any(AbortSignal),
                )
                expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(1)
                expect(wrapper.get('ul[aria-label="Project files"]').text()).toContain(
                    'uploaded-only.step',
                )
            })

            expect(
                wrapper.get('button[aria-label="Current revision"]').attributes('aria-pressed'),
            ).toBe('true')
            expect(wrapper.find('.project-file-workspace__preview').exists()).toBe(false)
            expect(wrapper.text()).not.toContain('committed-gripper.step')
        } finally {
            wrapper.unmount()
        }
    })

    it('defaults to current revision and explicitly switches to uploaded files', async () => {
        const wrapper = mount(ProjectFileWorkspace, {
            props: {
                projectId,
                branchId,
                headRevisionId,
            },
        })

        try {
            const scopes = wrapper.get('[role="group"][aria-label="File scope"]')
            const current = scopes.get('button[aria-label="Current revision"]')
            const uploaded = scopes.get('button[aria-label="All uploaded files"]')

            expect(current.text()).toContain('Current revision')
            expect(uploaded.text()).toContain('All uploaded files')
            expect(current.attributes('aria-pressed')).toBe('true')
            expect(uploaded.attributes('aria-pressed')).toBe('false')

            await vi.waitFor(() => {
                expect(listProjectRevisionFiles).toHaveBeenCalledWith(
                    projectId,
                    headRevisionId,
                    expect.any(AbortSignal),
                )

                expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(1)
            })

            expect(wrapper.text()).toContain('committed-gripper.step')
            expect(wrapper.text()).not.toContain('uploaded-only.step')
            expect(listProjectFiles).not.toHaveBeenCalled()

            await uploaded.trigger('click')

            await vi.waitFor(() => {
                expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(2)
            })

            expect(current.attributes('aria-pressed')).toBe('false')
            expect(uploaded.attributes('aria-pressed')).toBe('true')
            expect(wrapper.text()).toContain('committed-gripper.step')
            expect(wrapper.text()).toContain('uploaded-only.step')
            expect(listProjectFiles).toHaveBeenCalledTimes(1)
        } finally {
            wrapper.unmount()
        }
    })
})
