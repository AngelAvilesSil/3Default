import { mount } from '@vue/test-utils'
import { createMemoryHistory } from 'vue-router'
import { describe, expect, it, vi } from 'vitest'

import ProjectFileUploadView from '../views/ProjectFileUploadView.vue'
import { createAppRouter } from '../router'

const projectId = '11111111-1111-4111-8111-111111111111'
const otherProjectId = '22222222-2222-4222-8222-222222222222'

function renderView() {
    const router = createAppRouter(createMemoryHistory())

    const wrapper = mount(ProjectFileUploadView, {
        props: { projectId },
        global: {
            plugins: [router],
        },
    })

    return { wrapper, router }
}

function makeFile(name: string, content: string): File {
    return new File([content], name, {
        type: 'application/octet-stream',
        lastModified: 1700000000000,
    })
}

async function chooseFiles(wrapper: ReturnType<typeof renderView>['wrapper'], files: File[]) {
    const input = wrapper.get('input[type="file"]')

    Object.defineProperty(input.element, 'files', {
        configurable: true,
        value: files,
    })

    await input.trigger('change')
}

async function dropFiles(wrapper: ReturnType<typeof renderView>['wrapper'], files: File[]) {
    await wrapper.get('[data-testid="file-drop-zone"]').trigger('drop', {
        dataTransfer: { files },
    })
}

describe('project file upload selection', () => {
    it('retains project-specific back navigation', () => {
        const { wrapper } = renderView()

        expect(wrapper.get('h2').text()).toBe('Upload files')

        expect(wrapper.get('a').attributes('href')).toBe(`/projects/${projectId}`)

        wrapper.unmount()
    })

    it('provides a native multi-file picker and accessible choose action', async () => {
        const { wrapper } = renderView()

        const input = wrapper.get('input[type="file"]')

        expect(input.attributes('multiple')).toBeDefined()

        const click = vi
            .spyOn(input.element as HTMLInputElement, 'click')
            .mockImplementation(() => {})

        await wrapper.get('button[aria-label="Choose files"]').trigger('click')

        expect(click).toHaveBeenCalledOnce()

        wrapper.unmount()
    })

    it('lists multiple selected files without uploading them', async () => {
        const { wrapper } = renderView()

        await chooseFiles(wrapper, [
            makeFile('gripper.step', 'STEP source'),
            makeFile('housing.glb', 'GLB source'),
        ])

        const rows = wrapper.findAll('ul[aria-label="Selected files"] > li')

        expect(rows).toHaveLength(2)
        expect(rows[0]!.text()).toContain('gripper.step')
        expect(rows[1]!.text()).toContain('housing.glb')
        expect(wrapper.text()).toContain('2 files')

        wrapper.unmount()
    })

    it('displays a readable file size', async () => {
        const { wrapper } = renderView()

        await chooseFiles(wrapper, [makeFile('large.step', 'x'.repeat(2048))])

        expect(wrapper.get('ul[aria-label="Selected files"]').text()).toContain('2 KB')

        wrapper.unmount()
    })

    it('adds dropped files to the selection', async () => {
        const { wrapper } = renderView()

        await dropFiles(wrapper, [makeFile('dropped.step', 'CAD content')])

        expect(wrapper.get('ul[aria-label="Selected files"]').text()).toContain('dropped.step')

        wrapper.unmount()
    })

    it('combines picker and dropped files', async () => {
        const { wrapper } = renderView()

        await chooseFiles(wrapper, [makeFile('first.step', 'first')])

        await dropFiles(wrapper, [makeFile('second.step', 'second')])

        expect(wrapper.findAll('ul[aria-label="Selected files"] > li')).toHaveLength(2)

        wrapper.unmount()
    })

    it('does not duplicate the same selected file', async () => {
        const { wrapper } = renderView()

        const file = makeFile('gripper.step', 'source')

        await chooseFiles(wrapper, [file])
        await dropFiles(wrapper, [file])
        await chooseFiles(wrapper, [file])

        expect(wrapper.findAll('ul[aria-label="Selected files"] > li')).toHaveLength(1)

        wrapper.unmount()
    })

    it('removes an individual file without clearing the others', async () => {
        const { wrapper } = renderView()

        await chooseFiles(wrapper, [
            makeFile('gripper.step', 'first'),
            makeFile('housing.glb', 'second'),
        ])

        await wrapper.get('button[aria-label="Remove gripper.step"]').trigger('click')

        const directory = wrapper.get('ul[aria-label="Selected files"]')

        expect(directory.text()).not.toContain('gripper.step')
        expect(directory.text()).toContain('housing.glb')
        expect(directory.findAll('li')).toHaveLength(1)

        wrapper.unmount()
    })

    it('clears local selections when the project changes', async () => {
        const { wrapper } = renderView()

        await chooseFiles(wrapper, [makeFile('gripper.step', 'source')])

        await wrapper.setProps({
            projectId: otherProjectId,
        })

        expect(wrapper.find('ul[aria-label="Selected files"]').exists()).toBe(false)

        expect(wrapper.get('a').attributes('href')).toBe(`/projects/${otherProjectId}`)

        wrapper.unmount()
    })
})
