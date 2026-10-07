import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import ThreePreviewCanvas from '../components/ThreePreviewCanvas.vue'

const threeMocks = vi.hoisted(() => ({
    sceneAdd: vi.fn<(...objects: unknown[]) => void>(),
    sceneClear: vi.fn<() => void>(),
    cameraPositionSet: vi.fn<(x: number, y: number, z: number) => void>(),
    cameraUpdateProjectionMatrix: vi.fn<() => void>(),
    rendererSetPixelRatio: vi.fn<(pixelRatio: number) => void>(),
    rendererSetSize: vi.fn<(width: number, height: number, updateStyle?: boolean) => void>(),
    rendererRender: vi.fn<(scene: unknown, camera: unknown) => void>(),
    rendererSetAnimationLoop: vi.fn<(callback: (() => void) | null) => void>(),
    rendererDispose: vi.fn<() => void>(),
    controlsTargetSet: vi.fn<(x: number, y: number, z: number) => void>(),
    controlsUpdate: vi.fn<() => void>(),
    controlsDispose: vi.fn<() => void>(),
    directionalLightPositionSet: vi.fn<(x: number, y: number, z: number) => void>(),
}))

vi.mock('three', () => ({
    AmbientLight: class {},
    DirectionalLight: class {
        position = {
            set: threeMocks.directionalLightPositionSet,
        }
    },
    PerspectiveCamera: class {
        aspect = 1
        position = {
            set: threeMocks.cameraPositionSet,
        }
        updateProjectionMatrix = threeMocks.cameraUpdateProjectionMatrix
    },
    Scene: class {
        add = threeMocks.sceneAdd
        clear = threeMocks.sceneClear
    },
    WebGLRenderer: class {
        domElement = document.createElement('canvas')
        setPixelRatio = threeMocks.rendererSetPixelRatio
        setSize = threeMocks.rendererSetSize
        render = threeMocks.rendererRender
        setAnimationLoop = threeMocks.rendererSetAnimationLoop
        dispose = threeMocks.rendererDispose
    },
}))

vi.mock('three/addons/controls/OrbitControls.js', () => ({
    OrbitControls: class {
        enableDamping = false
        target = {
            set: threeMocks.controlsTargetSet,
        }
        update = threeMocks.controlsUpdate
        dispose = threeMocks.controlsDispose
    },
}))

describe('ThreePreviewCanvas', () => {
    const observe = vi.fn<(target: Element) => void>()
    const disconnect = vi.fn<() => void>()
    let resizeCallback: ResizeObserverCallback | null = null

    beforeEach(() => {
        vi.clearAllMocks()
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
        expect(threeMocks.cameraPositionSet).toHaveBeenCalledWith(3, 3, 3)
        expect(threeMocks.controlsTargetSet).toHaveBeenCalledWith(0, 0, 0)
        expect(threeMocks.sceneAdd).toHaveBeenCalledTimes(1)
        expect(observe).toHaveBeenCalledTimes(1)

        expect(threeMocks.rendererSetSize).toHaveBeenCalledWith(1, 1, false)
        expect(threeMocks.cameraUpdateProjectionMatrix).toHaveBeenCalledTimes(1)

        expect(threeMocks.rendererSetAnimationLoop).toHaveBeenCalledWith(expect.any(Function))

        const animationLoop = threeMocks.rendererSetAnimationLoop.mock.calls[0]?.[0]

        expect(animationLoop).toBeDefined()

        animationLoop?.()

        expect(threeMocks.controlsUpdate).toHaveBeenCalledTimes(2)
        expect(threeMocks.rendererRender).toHaveBeenCalledTimes(1)

        wrapper.unmount()

        expect(disconnect).toHaveBeenCalledTimes(1)
        expect(threeMocks.rendererSetAnimationLoop).toHaveBeenLastCalledWith(null)
        expect(threeMocks.controlsDispose).toHaveBeenCalledTimes(1)
        expect(threeMocks.sceneClear).toHaveBeenCalledTimes(1)
        expect(threeMocks.rendererDispose).toHaveBeenCalledTimes(1)
    })

    it('updates the renderer and camera when its container is resized', () => {
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

        expect(threeMocks.rendererSetSize).toHaveBeenLastCalledWith(640, 360, false)
        expect(threeMocks.cameraUpdateProjectionMatrix).toHaveBeenCalledTimes(2)

        wrapper.unmount()
    })
})
