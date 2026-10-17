import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
    BoxGeometry,
    DirectionalLight,
    Group,
    Mesh,
    MeshStandardMaterial,
    NeutralToneMapping,
    PerspectiveCamera,
    Quaternion,
    Scene,
    Texture,
    Vector3,
} from 'three'

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

const environmentMocks = vi.hoisted(() => ({
    fromScene: vi.fn<() => { texture: Texture; dispose: () => void }>(),
    generatorDispose: vi.fn<() => void>(),
    roomDispose: vi.fn<() => void>(),
    targetDispose: vi.fn<() => void>(),
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
        PMREMGenerator: class {
            fromScene = environmentMocks.fromScene
            dispose = environmentMocks.generatorDispose
        },
    }
})

vi.mock('three/addons/environments/RoomEnvironment.js', () => ({
    RoomEnvironment: class {
        dispose = environmentMocks.roomDispose
    },
}))

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
        environmentMocks.fromScene.mockReturnValue({
            texture: new Texture(),
            dispose: environmentMocks.targetDispose,
        })
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

    it('provides PBR environment lighting and releases its resources', () => {
        const wrapper = mount(ThreePreviewCanvas)

        expect(environmentMocks.fromScene).toHaveBeenCalledTimes(1)
        expect(environmentMocks.roomDispose).toHaveBeenCalledTimes(1)
        expect(environmentMocks.generatorDispose).toHaveBeenCalledTimes(1)

        const animationLoop = rendererMocks.setAnimationLoop.mock.calls[0]?.[0]
        expect(animationLoop).toBeDefined()
        animationLoop?.()

        const renderedScene = rendererMocks.render.mock.calls[0]?.[0] as Scene
        const generatedEnvironment = environmentMocks.fromScene.mock.results[0]?.value

        expect(renderedScene.environment).toBe(generatedEnvironment?.texture)
        expect(environmentMocks.targetDispose).not.toHaveBeenCalled()

        wrapper.unmount()

        expect(renderedScene.environment).toBeNull()
        expect(environmentMocks.targetDispose).toHaveBeenCalledTimes(1)
    })

    it('uses calibrated studio environment without world-fixed direct lights', () => {
        const wrapper = mount(ThreePreviewCanvas)

        const animationLoop = rendererMocks.setAnimationLoop.mock.calls[0]?.[0]
        expect(animationLoop).toBeDefined()
        animationLoop?.()

        const renderedScene = rendererMocks.render.mock.calls[0]?.[0] as Scene

        const lights = renderedScene.children.filter(
            (object) => 'isLight' in object && object.isLight === true,
        )

        expect(lights).toHaveLength(0)
        expect(renderedScene.environmentIntensity).toBeGreaterThan(0)
        expect(renderedScene.environmentIntensity).toBeLessThanOrEqual(0.5)

        const renderContext = rendererMocks.render.mock.contexts[0] as {
            toneMapping?: number
            toneMappingExposure?: number
        }

        expect(renderContext.toneMapping).toBe(NeutralToneMapping)
        expect(renderContext.toneMappingExposure).toBe(1)

        wrapper.unmount()
    })

    it('mounts inspection lights on the camera rather than the model', () => {
        const wrapper = mount(ThreePreviewCanvas)

        const animationLoop = rendererMocks.setAnimationLoop.mock.calls[0]?.[0]
        expect(animationLoop).toBeDefined()
        animationLoop?.()

        const renderedScene = rendererMocks.render.mock.calls[0]?.[0] as Scene
        const renderedCamera = rendererMocks.render.mock.calls[0]?.[1] as PerspectiveCamera

        expect(renderedCamera.parent).toBe(renderedScene)

        const lights = renderedCamera.children.filter(
            (object): object is DirectionalLight => object instanceof DirectionalLight,
        )

        expect(lights).toHaveLength(2)

        const keyLight = lights[0]!
        const fillLight = lights[1]!

        expect(keyLight.intensity).toBeGreaterThan(fillLight.intensity)
        expect(keyLight.intensity).toBeLessThanOrEqual(1)
        expect(fillLight.intensity).toBeGreaterThan(0)

        for (const light of lights) {
            expect(light.parent).toBe(renderedCamera)
            expect(light.target.parent).toBe(renderedCamera)
            expect(light.target.position.z).toBeLessThan(0)
            expect(light.castShadow).toBe(false)
        }

        const worldLights = renderedScene.children.filter(
            (object) => object instanceof DirectionalLight,
        )

        expect(worldLights).toHaveLength(0)

        renderedCamera.updateMatrixWorld(true)

        const initialPosition = keyLight.getWorldPosition(new Vector3())
        const initialDirection = keyLight.target
            .getWorldPosition(new Vector3())
            .sub(initialPosition)
            .normalize()

        renderedCamera.quaternion.setFromAxisAngle(new Vector3(0, 1, 0), Math.PI / 2)

        renderedCamera.updateMatrixWorld(true)

        const rotatedPosition = keyLight.getWorldPosition(new Vector3())
        const rotatedDirection = keyLight.target
            .getWorldPosition(new Vector3())
            .sub(rotatedPosition)
            .normalize()

        expect(rotatedPosition.distanceTo(initialPosition)).toBeGreaterThan(0.1)
        expect(rotatedDirection.distanceTo(initialDirection)).toBeGreaterThan(0.1)

        wrapper.unmount()
    })

    it('keeps studio illumination aligned with the orbiting camera', () => {
        const wrapper = mount(ThreePreviewCanvas)

        const animationLoop = rendererMocks.setAnimationLoop.mock.calls[0]?.[0]
        expect(animationLoop).toBeDefined()

        animationLoop?.()

        const renderedScene = rendererMocks.render.mock.calls[0]?.[0] as Scene
        const renderedCamera = rendererMocks.render.mock.calls[0]?.[1] as PerspectiveCamera

        expect(renderedScene.environment).not.toBeNull()

        const initialEnvironmentRotation = new Quaternion().setFromEuler(
            renderedScene.environmentRotation,
        )

        renderedCamera.quaternion.setFromAxisAngle(new Vector3(0, 1, 0), Math.PI / 2)

        animationLoop?.()

        const actualRotation = new Quaternion().setFromEuler(renderedScene.environmentRotation)

        const expectedRotation = renderedCamera.quaternion.clone().invert()

        expect(Math.abs(actualRotation.dot(expectedRotation))).toBeCloseTo(1, 6)
        expect(Math.abs(actualRotation.dot(initialEnvironmentRotation))).toBeLessThan(0.999)

        wrapper.unmount()
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
