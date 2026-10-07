<script setup lang="ts">
    import { ref, watch } from 'vue'

    import { fetchProjectFilePreview } from '../api/projectFilePreview'
    import ThreePreviewCanvas from './ThreePreviewCanvas.vue'

    type PreviewState = 'loading' | 'available' | 'not-found' | 'error'

    const props = defineProps<{
        projectId: string
        projectFileId: string
    }>()

    const state = ref<PreviewState>('loading')
    const glbData = ref<ArrayBuffer | null>(null)
    const errorMessage = ref('')

    let requestGeneration = 0

    watch(
        () => [props.projectId, props.projectFileId] as const,
        async ([projectId, projectFileId], _previous, onCleanup) => {
            const generation = ++requestGeneration
            const controller = new AbortController()

            onCleanup(() => {
                controller.abort()
            })

            state.value = 'loading'
            glbData.value = null
            errorMessage.value = ''

            try {
                const result = await fetchProjectFilePreview(
                    projectId,
                    projectFileId,
                    controller.signal,
                )

                if (generation !== requestGeneration || controller.signal.aborted) {
                    return
                }

                if (result.status === 'not-found') {
                    state.value = 'not-found'
                    return
                }

                glbData.value = result.data
                state.value = 'available'
            } catch {
                if (generation !== requestGeneration || controller.signal.aborted) {
                    return
                }

                state.value = 'error'
                errorMessage.value = 'The preview could not be loaded.'
            }
        },
        {
            immediate: true,
        },
    )

    function handleLoadError() {
        glbData.value = null
        state.value = 'error'
        errorMessage.value = 'The preview could not be rendered.'
    }
</script>

<template>
    <section class="project-file-preview" aria-live="polite">
        <p v-if="state === 'loading'" data-testid="preview-loading" role="status">
            Loading preview…
        </p>

        <p v-else-if="state === 'not-found'" data-testid="preview-not-found">
            No preview is available for this file yet.
        </p>

        <p v-else-if="state === 'error'" data-testid="preview-error" role="alert">
            {{ errorMessage }}
        </p>

        <ThreePreviewCanvas v-else :glb-data="glbData" @load-error="handleLoadError" />
    </section>
</template>

<style scoped>
    .project-file-preview {
        width: 100%;
        min-height: 20rem;
    }

    .project-file-preview > p {
        margin: 0;
        padding: 2rem;
    }
</style>
