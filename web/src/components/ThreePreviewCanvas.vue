<script setup lang="ts">
    import { onBeforeUnmount, onMounted, ref } from 'vue'
    import { AmbientLight, DirectionalLight, PerspectiveCamera, Scene, WebGLRenderer } from 'three'
    import { OrbitControls } from 'three/addons/controls/OrbitControls.js'

    const container = ref<HTMLDivElement | null>(null)

    let scene: Scene | null = null
    let renderer: WebGLRenderer | null = null
    let controls: OrbitControls | null = null
    let resizeObserver: ResizeObserver | null = null

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

        nextCamera.position.set(3, 3, 3)

        nextRenderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2))
        host.appendChild(nextRenderer.domElement)

        const nextControls = new OrbitControls(nextCamera, nextRenderer.domElement)

        nextControls.enableDamping = true
        nextControls.target.set(0, 0, 0)
        nextControls.update()

        const ambientLight = new AmbientLight(0xffffff, 1.25)
        const directionalLight = new DirectionalLight(0xffffff, 2)

        directionalLight.position.set(4, 6, 4)

        nextScene.add(ambientLight, directionalLight)

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

        nextRenderer.setAnimationLoop(() => {
            nextControls.update()
            nextRenderer.render(nextScene, nextCamera)
        })

        scene = nextScene
        renderer = nextRenderer
        controls = nextControls
        resizeObserver = nextResizeObserver
    })

    onBeforeUnmount(() => {
        resizeObserver?.disconnect()
        renderer?.setAnimationLoop(null)
        controls?.dispose()
        scene?.clear()

        const canvas = renderer?.domElement

        renderer?.dispose()
        canvas?.remove()

        resizeObserver = null
        controls = null
        renderer = null
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
