<script setup lang="ts">
    import { defineAsyncComponent, ref, watch } from 'vue'

    import {
        fetchProjectFilePreview,
        ProjectFilePreviewRequestError,
    } from '../api/projectFilePreview'
    import AppButton from './ui/AppButton.vue'

    const ThreePreviewCanvas = defineAsyncComponent(() => import('./ThreePreviewCanvas.vue'))

    const props = defineProps<{
        projectId: string
        projectFileId: string
    }>()

    const emit = defineEmits<{
        unauthorized: []
    }>()

    type PreviewState = 'loading' | 'available' | 'not-found' | 'error' | 'unauthorized'

    const state = ref<PreviewState>('loading')
    const glbData = ref<ArrayBuffer | null>(null)
    const retryGeneration = ref(0)
    const errorMessage = ref('')

    let requestGeneration = 0

    function retryPreview(): void {
        retryGeneration.value += 1
    }

    function handleRenderError(): void {
        glbData.value = null
        errorMessage.value = 'The existing GLB preview could not be rendered.'
        state.value = 'error'
    }

    watch(
        () => [props.projectId, props.projectFileId, retryGeneration.value] as const,
        async ([projectId, projectFileId], _previous, onCleanup) => {
            const generation = ++requestGeneration
            const controller = new AbortController()

            onCleanup(() => controller.abort())

            const isCurrent = () => generation === requestGeneration && !controller.signal.aborted

            state.value = 'loading'
            glbData.value = null
            errorMessage.value = ''

            try {
                // GET only. Never create, poll, or retry a conversion job.
                const result = await fetchProjectFilePreview(
                    projectId,
                    projectFileId,
                    controller.signal,
                )

                if (!isCurrent()) {
                    return
                }

                if (result.status === 'not-found') {
                    state.value = 'not-found'
                    return
                }

                glbData.value = result.data
                state.value = 'available'
            } catch (error) {
                if (!isCurrent()) {
                    return
                }

                glbData.value = null

                if (error instanceof ProjectFilePreviewRequestError && error.status === 401) {
                    state.value = 'unauthorized'
                    emit('unauthorized')
                    return
                }

                errorMessage.value = 'Unable to load the existing GLB preview.'
                state.value = 'error'
            }
        },
        { immediate: true },
    )
</script>

<template>
    <section class="historical-file-preview" aria-label="Existing historical file preview">
        <p v-if="state === 'loading'" data-testid="historical-preview-loading" role="status">
            Checking for an existing GLB preview...
        </p>

        <p v-else-if="state === 'not-found'" data-testid="historical-preview-missing">
            No existing GLB preview is available for this source file. Historical inspection does
            not start conversion.
        </p>

        <p
            v-else-if="state === 'unauthorized'"
            data-testid="historical-preview-unauthorized"
            role="alert"
        >
            Session expired. Please sign in again.
        </p>

        <p v-else-if="state === 'error'" data-testid="historical-preview-error" role="alert">
            {{ errorMessage }}
        </p>

        <ThreePreviewCanvas
            v-else-if="state === 'available' && glbData"
            :glb-data="glbData"
            @load-error="handleRenderError"
        />

        <AppButton
            v-if="state === 'not-found' || state === 'error'"
            variant="secondary"
            @click="retryPreview"
        >
            Retry existing preview
        </AppButton>
    </section>
</template>

<style scoped>
    .historical-file-preview {
        min-width: 0;
    }

    .historical-file-preview > p {
        margin: var(--space-3) 0;
        color: var(--color-text-muted);
    }

    .historical-file-preview > [role='alert'] {
        color: var(--color-text);
    }
</style>
