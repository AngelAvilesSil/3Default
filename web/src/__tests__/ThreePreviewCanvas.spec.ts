import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { BoxGeometry, Group, Mesh, MeshStandardMaterial, Texture } from 'three'

import ThreePreviewCanvas from '../components/ThreePreviewCanvas.vue'

const rendererMocks = vi.hoisted(() => ({
    setPixelRatio: vi.fn<(pixelRatio: number) => void>(),
    setSize: vi.fn<(width: number, height: number, updateStyle?: boolean) => void>(),
    render: vi.fn<(scene: unknown, camera: unknown) => void>(),
    setAnimationLoop: vi.fn<(callback: (() => void) | null) => void>(),
    dispose: vi.fn<() => void>(),
}))

const controlsMocks = vi.hoisted(() => ({
    targetSet: vi.fn<(x: number, y: number, z: number) => void>(),
    targetCopy: vi.fn<(value: { x: number; y: number; z: number }) => void>(),
    update: vi.fn<() => void>(),
    dispose: vi.fn<() => void>(),
}))

const loaderMocks = vi.hoisted(() => ({
    parseAsync: vi.fn<(data: string | ArrayBuffer, path: string) => Promise<{ scene: unknown }>>(),
}))

vi.mock('three', async (importOriginal) => {
    const actual = await importOriginal<typeof import('three')>()

    return {
        ...actual,
        WebGLRenderer: class {
            domElement = document.createElement('canvas')
            setPixelRatio = rendererMocks.setPixelRatio
            setSize = rendererMocks.setSize
            render = rendererMocks.render
            setAnimationLoop = rendererMocks.setAnimationLoop
            dispose = rendererMocks.dispose
        },
    }
})

vi.mock('three/addons/controls/OrbitControls.js', () => ({
    OrbitControls: class {
        enableDamping = false

        target = {
            x: 0,
            y: 0,
            z: 0,

            set(x: number, y: number, z: number) {
                this.x = x
                this.y = y
                this.z = z
                controlsMocks.targetSet(x, y, z)
                return this
            },

            copy(value: { x: number; y: number; z: number }) {
                this.x = value.x
                this.y = value.y
                this.z = value.z
                controlsMocks.targetCopy(value)
                return this
            },
        }

        update() {
            controlsMocks.update()
        }

        dispose() {
            controlsMocks.dispose()
        }
    },
}))

vi.mock('three/addons/loaders/GLTFLoader.js', () => ({
    GLTFLoader: class {
        parseAsync = loaderMocks.parseAsync
    },
}))

function createDisposableModel() {
    const sourceClose = vi.fn<() => void>()
    const texture = new Texture({
        close: sourceClose,
    } as unknown as TexImageSource)
    const geometry = new BoxGeometry(2, 4, 6)
    const material = new MeshStandardMaterial({
        map: texture,
    })

    const model = new Group()
    model.add(new Mesh(geometry, material))

    return {
        model,
        sourceClose,
        geometryDispose: vi.spyOn(geometry, 'dispose'),
        materialDispose: vi.spyOn(material, 'dispose'),
        textureDispose: vi.spyOn(texture, 'dispose'),
    }
}

describe('ThreePreviewCanvas', () => {
    const observe = vi.fn<(target: Element) => void>()
    const disconnect = vi.fn<() => void>()
    let resizeCallback: ResizeObserverCallback | null = null

    beforeEach(() => {
        vi.clearAllMocks()
        loaderMocks.parseAsync.mockReset()
        resizeCallback = null

        vi.stubGlobal(
            'ResizeObserver',
            class {
                constructor(callback: ResizeObserverCallback) {
                    resizeCallback = callback
                }

                observe = observe
                disconnect = disconnect
            },
        )
    })

    it('creates the Three.js scene and releases browser resources on unmount', () => {
        const wrapper = mount(ThreePreviewCanvas, {
            attachTo: document.body,
        })

        expect(wrapper.find('canvas').exists()).toBe(true)
        expect(controlsMocks.targetSet).toHaveBeenCalledWith(0, 0, 0)
        expect(observe).toHaveBeenCalledTimes(1)

        expect(rendererMocks.setSize).toHaveBeenCalledWith(1, 1, false)

        expect(rendererMocks.setAnimationLoop).toHaveBeenCalledWith(expect.any(Function))

        const animationLoop = rendererMocks.setAnimationLoop.mock.calls[0]?.[0]

        expect(animationLoop).toBeDefined()

        animationLoop?.()

        expect(controlsMocks.update).toHaveBeenCalledTimes(2)
        expect(rendererMocks.render).toHaveBeenCalledTimes(1)

        wrapper.unmount()

        expect(disconnect).toHaveBeenCalledTimes(1)
        expect(rendererMocks.setAnimationLoop).toHaveBeenLastCalledWith(null)
        expect(controlsMocks.dispose).toHaveBeenCalledTimes(1)
        expect(rendererMocks.dispose).toHaveBeenCalledTimes(1)
    })

    it('updates the renderer when its container is resized', () => {
        const wrapper = mount(ThreePreviewCanvas)

        const host = wrapper.get('[data-testid="three-preview-canvas"]').element as HTMLDivElement

        Object.defineProperty(host, 'clientWidth', {
            configurable: true,
            value: 640,
        })
        Object.defineProperty(host, 'clientHeight', {
            configurable: true,
            value: 360,
        })

        resizeCallback?.([], {} as ResizeObserver)

        expect(rendererMocks.setSize).toHaveBeenLastCalledWith(640, 360, false)

        wrapper.unmount()
    })

    it('parses GLB data, adds the model, and fits the controls target', async () => {
        const data = new ArrayBuffer(16)
        const resources = createDisposableModel()

        loaderMocks.parseAsync.mockResolvedValueOnce({
            scene: resources.model,
        })

        const wrapper = mount(ThreePreviewCanvas, {
            props: {
                glbData: data,
            },
        })

        await flushPromises()

        expect(loaderMocks.parseAsync).toHaveBeenCalledWith(data, '')
        expect(resources.model.parent).not.toBeNull()
        expect(controlsMocks.targetCopy).toHaveBeenCalledTimes(1)
        expect(wrapper.emitted('loaded')).toHaveLength(1)
        expect(wrapper.emitted('load-error')).toBeUndefined()

        wrapper.unmount()

        expect(resources.model.parent).toBeNull()
        expect(resources.textureDispose).toHaveBeenCalledTimes(1)
        expect(resources.materialDispose).toHaveBeenCalledTimes(1)
        expect(resources.geometryDispose).toHaveBeenCalledTimes(1)
        expect(resources.sourceClose).toHaveBeenCalledTimes(1)
    })

    it('disposes the previous model when GLB data is replaced', async () => {
        const first = createDisposableModel()
        const second = createDisposableModel()

        loaderMocks.parseAsync
            .mockResolvedValueOnce({
                scene: first.model,
            })
            .mockResolvedValueOnce({
                scene: second.model,
            })

        const wrapper = mount(ThreePreviewCanvas, {
            props: {
                glbData: new ArrayBuffer(8),
            },
        })

        await flushPromises()

        await wrapper.setProps({
            glbData: new ArrayBuffer(12),
        })
        await flushPromises()

        expect(first.model.parent).toBeNull()
        expect(first.textureDispose).toHaveBeenCalledTimes(1)
        expect(first.materialDispose).toHaveBeenCalledTimes(1)
        expect(first.geometryDispose).toHaveBeenCalledTimes(1)
        expect(first.sourceClose).toHaveBeenCalledTimes(1)

        expect(second.model.parent).not.toBeNull()
        expect(wrapper.emitted('loaded')).toHaveLength(2)

        wrapper.unmount()

        expect(second.textureDispose).toHaveBeenCalledTimes(1)
        expect(second.materialDispose).toHaveBeenCalledTimes(1)
        expect(second.geometryDispose).toHaveBeenCalledTimes(1)
        expect(second.sourceClose).toHaveBeenCalledTimes(1)
    })

    it('emits a load error when GLB parsing fails', async () => {
        const error = new Error('invalid GLB')

        loaderMocks.parseAsync.mockRejectedValueOnce(error)

        const wrapper = mount(ThreePreviewCanvas, {
            props: {
                glbData: new ArrayBuffer(4),
            },
        })

        await flushPromises()

        expect(wrapper.emitted('loaded')).toBeUndefined()
        expect(wrapper.emitted('load-error')).toEqual([[error]])

        wrapper.unmount()
    })
})
