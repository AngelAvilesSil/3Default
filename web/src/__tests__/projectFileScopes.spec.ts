import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { listProjectFiles, type ProjectFile } from '../api/projectFiles'
import { listProjectRevisionFiles } from '../api/revisions'
import ProjectFilesPanel from '../components/ProjectFilesPanel.vue'

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
const headRevisionId = '22222222-2222-4222-8222-222222222222'

const committedFile: ProjectFile = {
    id: '33333333-3333-4333-8333-333333333333',
    projectId,
    uploadedByUserId: '44444444-4444-4444-8444-444444444444',
    contentSha256: 'a'.repeat(64),
    originalFilename: 'committed-gripper.step',
    mediaType: 'model/step',
    createdAt: '2026-10-21T12:00:00Z',
}

const uploadedOnlyFile: ProjectFile = {
    ...committedFile,
    id: '55555555-5555-4555-8555-555555555555',
    contentSha256: 'b'.repeat(64),
    originalFilename: 'uploaded-only.step',
}

describe('project file scopes', () => {
    beforeEach(() => {
        vi.resetAllMocks()

        vi.mocked(listProjectFiles).mockResolvedValue([committedFile, uploadedOnlyFile])

        vi.mocked(listProjectRevisionFiles).mockResolvedValue([committedFile])
    })

    it('shows a headless branch as an empty revision without fetching uploads', async () => {
        const wrapper = mount(ProjectFilesPanel, {
            props: {
                projectId,
                scope: 'revision',
                revisionId: null,
            },
        })

        try {
            await vi.waitFor(() => {
                expect(wrapper.text()).toContain('This branch has no revisions yet.')
            })

            expect(wrapper.text()).not.toContain('unavailable or not accessible')
            expect(wrapper.text()).not.toContain('uploaded-only.step')
            expect(listProjectRevisionFiles).not.toHaveBeenCalled()
            expect(listProjectFiles).not.toHaveBeenCalled()
        } finally {
            wrapper.unmount()
        }
    })

    it('switches from revision membership to all uploaded files', async () => {
        const wrapper = mount(ProjectFilesPanel, {
            props: {
                projectId,
                scope: 'revision',
                revisionId: headRevisionId,
            },
        })

        try {
            await vi.waitFor(() => {
                expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(1)
            })

            expect(wrapper.text()).toContain('committed-gripper.step')
            expect(wrapper.text()).not.toContain('uploaded-only.step')
            expect(listProjectFiles).not.toHaveBeenCalled()

            await wrapper.setProps({
                scope: 'uploaded',
                revisionId: undefined,
            })

            await vi.waitFor(() => {
                expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(2)
            })

            expect(wrapper.text()).toContain('committed-gripper.step')
            expect(wrapper.text()).toContain('uploaded-only.step')
            expect(listProjectRevisionFiles).toHaveBeenCalledTimes(1)
            expect(listProjectFiles).toHaveBeenCalledTimes(1)
        } finally {
            wrapper.unmount()
        }
    })

    it('ignores an obsolete revision response after switching to uploaded files', async () => {
        let resolveOld!: (files: ProjectFile[]) => void

        vi.mocked(listProjectRevisionFiles).mockImplementationOnce(
            () =>
                new Promise<ProjectFile[]>((resolve) => {
                    resolveOld = resolve
                }),
        )

        const wrapper = mount(ProjectFilesPanel, {
            props: {
                projectId,
                scope: 'revision',
                revisionId: headRevisionId,
            },
        })

        try {
            await vi.waitFor(() => {
                expect(listProjectRevisionFiles).toHaveBeenCalledTimes(1)
            })

            const oldSignal = vi.mocked(listProjectRevisionFiles).mock.calls[0]?.[2]

            expect(oldSignal).toBeDefined()
            expect(oldSignal?.aborted).toBe(false)

            await wrapper.setProps({
                scope: 'uploaded',
                revisionId: undefined,
            })

            await vi.waitFor(() => {
                expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(2)
            })

            expect(oldSignal?.aborted).toBe(true)

            resolveOld([committedFile])
            await flushPromises()

            expect(wrapper.text()).toContain('committed-gripper.step')
            expect(wrapper.text()).toContain('uploaded-only.step')
            expect(wrapper.findAll('ul[aria-label="Project files"] > li')).toHaveLength(2)
        } finally {
            wrapper.unmount()
        }
    })

    it('shows only exact head-revision files in revision scope', async () => {
        const wrapper = mount(ProjectFilesPanel, {
            props: {
                projectId,
                scope: 'revision',
                revisionId: headRevisionId,
            },
        })

        try {
            await vi.waitFor(() => {
                expect(listProjectRevisionFiles).toHaveBeenCalledWith(
                    projectId,
                    headRevisionId,
                    expect.any(AbortSignal),
                )
            })

            const rows = wrapper.findAll('ul[aria-label="Project files"] > li')

            expect(rows).toHaveLength(1)
            expect(rows[0]!.text()).toContain('committed-gripper.step')

            expect(wrapper.text()).not.toContain('uploaded-only.step')
            expect(listProjectFiles).not.toHaveBeenCalled()
        } finally {
            wrapper.unmount()
        }
    })
})
