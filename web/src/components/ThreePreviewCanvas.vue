<script setup lang="ts">
    import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
    import {
        Box3,
        DirectionalLight,
        MathUtils,
        NeutralToneMapping,
        PMREMGenerator,
        PerspectiveCamera,
        Quaternion,
        Scene,
        Sphere,
        WebGLRenderer,
        type BufferGeometry,
        type Material,
        type Mesh,
        type Object3D,
        type Texture,
        type WebGLRenderTarget,
    } from 'three'
    import { RoomEnvironment } from 'three/addons/environments/RoomEnvironment.js'
    import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js'
    import { OrbitControls } from 'three/addons/controls/OrbitControls.js'

    const props = withDefaults(
        defineProps<{
            glbData?: ArrayBuffer | null
        }>(),
        {
            glbData: null,
        },
    )

    const emit = defineEmits<{
        loaded: []
        'load-error': [error: unknown]
    }>()

    const container = ref<HTMLDivElement | null>(null)

    const loader = new GLTFLoader()

    let scene: Scene | null = null
    let camera: PerspectiveCamera | null = null
    let renderer: WebGLRenderer | null = null
    let environmentTarget: WebGLRenderTarget | null = null
    let controls: OrbitControls | null = null
    let resizeObserver: ResizeObserver | null = null
    let currentModel: Object3D | null = null
    let loadGeneration = 0

    function isTexture(value: unknown): value is Texture {
        return (
            typeof value === 'object' &&
            value !== null &&
            'isTexture' in value &&
            (value as { isTexture?: boolean }).isTexture === true
        )
    }

    function disposeModel(root: Object3D) {
        const geometries = new Set<BufferGeometry>()
        const materials = new Set<Material>()
        const textures = new Set<Texture>()

        root.traverse((object) => {
            const mesh = object as Mesh

            if (!mesh.isMesh) {
                return
            }

            geometries.add(mesh.geometry)

            const meshMaterials = Array.isArray(mesh.material) ? mesh.material : [mesh.material]

            for (const material of meshMaterials) {
                materials.add(material)

                for (const value of Object.values(material)) {
                    if (isTexture(value)) {
                        textures.add(value)
                    }
                }
            }
        })

        const closedSources = new Set<object>()

        for (const texture of textures) {
            const sourceData = texture.source.data as unknown

            if (
                typeof sourceData === 'object' &&
                sourceData !== null &&
                !closedSources.has(sourceData)
            ) {
                closedSources.add(sourceData)

                const close = (sourceData as { close?: unknown }).close

                if (typeof close === 'function') {
                    close.call(sourceData)
                }
            }

            texture.dispose()
        }

        for (const material of materials) {
            material.dispose()
        }

        for (const geometry of geometries) {
            geometry.dispose()
        }
    }

    function removeCurrentModel() {
        if (!currentModel) {
            return
        }

        scene?.remove(currentModel)
        disposeModel(currentModel)
        currentModel = null
    }

    function fitCameraToModel(
        model: Object3D,
        targetCamera: PerspectiveCamera,
        targetControls: OrbitControls,
    ) {
        model.updateWorldMatrix(true, true)

        const box = new Box3().setFromObject(model, true)

        if (box.isEmpty()) {
            return
        }

        const sphere = box.getBoundingSphere(new Sphere())

        if (!Number.isFinite(sphere.radius)) {
            return
        }

        const radius = Math.max(sphere.radius, 0.001)
        const verticalFov = MathUtils.degToRad(targetCamera.getEffectiveFOV())
        const horizontalFov = 2 * Math.atan(Math.tan(verticalFov / 2) * targetCamera.aspect)
        const fitFov = Math.min(verticalFov, horizontalFov)
        const distance = (radius / Math.sin(fitFov / 2)) * 1.2

        const direction = targetCamera.position.clone().sub(targetControls.target)

        if (direction.lengthSq() === 0) {
            direction.set(1, 1, 1)
        }

        direction.normalize()

        targetControls.target.copy(sphere.center)
        targetCamera.position.copy(sphere.center).addScaledVector(direction, distance)

        targetCamera.near = Math.max(radius / 100, 0.001)
        targetCamera.far = Math.max(distance + radius * 4, targetCamera.near * 100)
        targetCamera.updateProjectionMatrix()

        targetControls.update()
    }

    async function replaceModel(data: ArrayBuffer | null) {
        const generation = ++loadGeneration

        removeCurrentModel()

        if (!data || !scene || !camera || !controls) {
            return
        }

        try {
            const gltf = await loader.parseAsync(data, '')

            if (generation !== loadGeneration || !scene || !camera || !controls) {
                disposeModel(gltf.scene)
                return
            }

            currentModel = gltf.scene
            scene.add(currentModel)

            fitCameraToModel(currentModel, camera, controls)
            emit('loaded')
        } catch (error) {
            if (generation === loadGeneration) {
                emit('load-error', error)
            }
        }
    }

    watch(
        () => props.glbData,
        (data) => {
            if (scene) {
                void replaceModel(data)
            }
        },
    )

    onMounted(() => {
        const host = container.value

        if (!host) {
            return
        }

        const nextScene = new Scene()
        const nextCamera = new PerspectiveCamera(45, 1, 0.01, 10_000)
        const nextRenderer = new WebGLRenderer({
            antialias: true,
            alpha: true,
        })

        nextRenderer.toneMapping = NeutralToneMapping
        nextRenderer.toneMappingExposure = 1

        nextCamera.position.set(3, 3, 3)

        nextRenderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2))
        host.appendChild(nextRenderer.domElement)

        const nextControls = new OrbitControls(nextCamera, nextRenderer.domElement)

        nextControls.enableDamping = true
        nextControls.target.set(0, 0, 0)
        nextControls.update()

        // Camera-mounted inspection lights remain separate from model geometry.
        // Both lights illuminate the scene along the camera's viewing direction.
        const inspectionKey = new DirectionalLight(0xffffff, 0.7)
        inspectionKey.position.set(-1.5, 2, 2.5)
        inspectionKey.target.position.set(0, 0, -1)

        const inspectionFill = new DirectionalLight(0xffffff, 0.3)
        inspectionFill.position.set(1.5, -0.75, 2)
        inspectionFill.target.position.set(0, 0, -1)

        nextCamera.add(inspectionKey, inspectionKey.target, inspectionFill, inspectionFill.target)

        // Three.js updates camera-attached light transforms through the scene.
        nextScene.add(nextCamera)

        // Provide image-based lighting for glTF physically based materials.
        const pmremGenerator = new PMREMGenerator(nextRenderer)
        const roomEnvironment = new RoomEnvironment()

        try {
            environmentTarget = pmremGenerator.fromScene(roomEnvironment)
            nextScene.environment = environmentTarget.texture
            nextScene.environmentIntensity = 0.45
        } finally {
            roomEnvironment.dispose()
            pmremGenerator.dispose()
        }

        const resize = () => {
            const width = Math.max(host.clientWidth, 1)
            const height = Math.max(host.clientHeight, 1)

            nextRenderer.setSize(width, height, false)

            nextCamera.aspect = width / height
            nextCamera.updateProjectionMatrix()
        }

        const nextResizeObserver = new ResizeObserver(resize)

        nextResizeObserver.observe(host)
        resize()

        // Keep studio illumination aligned with the inspection camera.
        // Reuse the quaternion to avoid allocations during rendering.
        const inverseCameraRotation = new Quaternion()

        nextRenderer.setAnimationLoop(() => {
            nextControls.update()

            inverseCameraRotation.copy(nextCamera.quaternion).invert()
            nextScene.environmentRotation.setFromQuaternion(inverseCameraRotation)

            nextRenderer.render(nextScene, nextCamera)
        })

        scene = nextScene
        camera = nextCamera
        renderer = nextRenderer
        controls = nextControls
        resizeObserver = nextResizeObserver

        if (props.glbData) {
            void replaceModel(props.glbData)
        }
    })

    onBeforeUnmount(() => {
        ++loadGeneration

        removeCurrentModel()

        resizeObserver?.disconnect()
        renderer?.setAnimationLoop(null)
        controls?.dispose()

        if (scene) {
            scene.environment = null
        }

        environmentTarget?.dispose()
        environmentTarget = null

        scene?.clear()

        const canvas = renderer?.domElement

        renderer?.dispose()
        canvas?.remove()

        resizeObserver = null
        controls = null
        renderer = null
        camera = null
        scene = null
    })
</script>

<template>
    <div ref="container" class="three-preview-canvas" data-testid="three-preview-canvas"></div>
</template>

<style scoped>
    .three-preview-canvas {
        width: 100%;
        height: 100%;
        min-height: 20rem;
        overflow: hidden;
    }

    .three-preview-canvas :deep(canvas) {
        display: block;
        width: 100%;
        height: 100%;
    }
</style>
