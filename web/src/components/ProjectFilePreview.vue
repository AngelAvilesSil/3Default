<script setup lang="ts">
    import { ref, watch } from 'vue'

    import {
        ConversionJobRequestError,
        createProjectFileConversionJob,
        getProjectConversionJob,
        listProjectFileConversionJobs,
        type ConversionJob,
    } from '../api/conversionJobs'
    import {
        fetchProjectFilePreview,
        ProjectFilePreviewRequestError,
    } from '../api/projectFilePreview'
    import ThreePreviewCanvas from './ThreePreviewCanvas.vue'

    type PreviewState = 'loading' | 'available' | 'not-found' | 'preparing' | 'error'

    const props = defineProps<{
        projectId: string
        projectFileId: string
        sourceFilename?: string
    }>()

    const emit = defineEmits<{
        unauthorized: []
    }>()

    const state = ref<PreviewState>('loading')
    const glbData = ref<ArrayBuffer | null>(null)
    const errorMessage = ref('')
    const notFoundMessage = ref('No preview is available for this file yet.')

    let requestGeneration = 0

    const POLL_INTERVAL_MS = 1500
    const MAX_POLL_ATTEMPTS = 240

    function waitForPoll(signal: AbortSignal): Promise<void> {
        return new Promise<void>((resolve) => {
            if (signal.aborted) {
                resolve()
                return
            }

            const timeoutId = setTimeout(complete, POLL_INTERVAL_MS)

            function complete(): void {
                clearTimeout(timeoutId)
                signal.removeEventListener('abort', complete)
                resolve()
            }

            signal.addEventListener('abort', complete, { once: true })
        })
    }

    watch(
        () => [props.projectId, props.projectFileId, props.sourceFilename] as const,
        async ([projectId, projectFileId, sourceFilename], _previous, onCleanup) => {
            const generation = ++requestGeneration
            const controller = new AbortController()

            const isCurrent = () => generation === requestGeneration && !controller.signal.aborted

            onCleanup(() => {
                controller.abort()
            })

            state.value = 'loading'
            glbData.value = null
            errorMessage.value = ''
            notFoundMessage.value = 'No preview is available for this file yet.'

            function showError(message: string): void {
                if (!isCurrent()) {
                    return
                }

                glbData.value = null
                errorMessage.value = message
                state.value = 'error'
            }

            function handleAuthentication(error: unknown): boolean {
                const unauthorized =
                    (error instanceof ConversionJobRequestError ||
                        error instanceof ProjectFilePreviewRequestError) &&
                    error.status === 401

                if (!unauthorized || !isCurrent()) {
                    return false
                }

                showError('Session expired. Please sign in again.')
                emit('unauthorized')
                return true
            }

            function matchesSelectedFile(job: ConversionJob, expectedJobId?: string): boolean {
                return (
                    job.projectId === projectId &&
                    job.projectFileId === projectFileId &&
                    (expectedJobId === undefined || job.id === expectedJobId)
                )
            }

            async function displayCompletedPreview(): Promise<void> {
                try {
                    const result = await fetchProjectFilePreview(
                        projectId,
                        projectFileId,
                        controller.signal,
                    )

                    if (!isCurrent()) {
                        return
                    }

                    if (result.status === 'not-found') {
                        showError('The conversion succeeded, but its preview is unavailable.')
                        return
                    }

                    glbData.value = result.data
                    state.value = 'available'
                } catch (error) {
                    if (!isCurrent()) {
                        return
                    }

                    if (handleAuthentication(error)) {
                        return
                    }

                    showError('The completed preview could not be loaded.')
                }
            }

            async function followJob(initialJob: ConversionJob): Promise<void> {
                if (!isCurrent()) {
                    return
                }

                if (!matchesSelectedFile(initialJob)) {
                    showError('Unable to verify preview conversion status.')
                    return
                }

                if (initialJob.status === 'failed') {
                    showError(
                        'Preview generation failed. The source file may be unsupported or invalid.',
                    )
                    return
                }

                if (initialJob.status === 'succeeded') {
                    await displayCompletedPreview()
                    return
                }

                state.value = 'preparing'

                // Never overlap polling requests. Wait after each
                // response and stop after a bounded period.
                for (let attempt = 0; attempt < MAX_POLL_ATTEMPTS; attempt += 1) {
                    await waitForPoll(controller.signal)

                    if (!isCurrent()) {
                        return
                    }

                    let currentJob: ConversionJob

                    try {
                        currentJob = await getProjectConversionJob(
                            projectId,
                            initialJob.id,
                            controller.signal,
                        )
                    } catch (error) {
                        if (!isCurrent()) {
                            return
                        }

                        if (handleAuthentication(error)) {
                            return
                        }

                        showError(
                            'Unable to check preview conversion status. Reopen the file to check again.',
                        )
                        return
                    }

                    if (!isCurrent()) {
                        return
                    }

                    if (!matchesSelectedFile(currentJob, initialJob.id)) {
                        showError('Unable to verify preview conversion status.')
                        return
                    }

                    if (currentJob.status === 'succeeded') {
                        await displayCompletedPreview()
                        return
                    }

                    if (currentJob.status === 'failed') {
                        showError(
                            'Preview generation failed. The source file may be unsupported or invalid.',
                        )
                        return
                    }
                }

                showError(
                    'Preview preparation is taking longer than expected. Reopen the file to check its status.',
                )
            }

            async function reconcileCreation(uncertain: boolean): Promise<void> {
                let jobs: ConversionJob[]

                try {
                    jobs = await listProjectFileConversionJobs(
                        projectId,
                        projectFileId,
                        controller.signal,
                    )
                } catch (error) {
                    if (!isCurrent()) {
                        return
                    }

                    if (handleAuthentication(error)) {
                        return
                    }

                    showError(
                        uncertain
                            ? 'Preview preparation could not be confirmed. Reopen the file to check its status.'
                            : 'Unable to check the existing preview conversion.',
                    )
                    return
                }

                if (!isCurrent()) {
                    return
                }

                const activeJob = jobs.find(
                    (job) => job.status === 'pending' || job.status === 'running',
                )

                const existingJob = activeJob ?? jobs[0]

                if (existingJob === undefined) {
                    showError(
                        uncertain
                            ? 'Preview preparation could not be confirmed. Reopen the file to check its status.'
                            : 'An existing preview conversion could not be found.',
                    )
                    return
                }

                await followJob(existingJob)
            }

            try {
                // Always prefer an already available preview.
                const preview = await fetchProjectFilePreview(
                    projectId,
                    projectFileId,
                    controller.signal,
                )

                if (!isCurrent()) {
                    return
                }

                if (preview.status === 'available') {
                    glbData.value = preview.data
                    state.value = 'available'
                    return
                }

                // Preserve read-only compatibility when source
                // filename metadata was not provided.
                if (sourceFilename === undefined) {
                    state.value = 'not-found'
                    return
                }

                const filename = sourceFilename.trim().toLowerCase()

                const supported =
                    filename.endsWith('.step') ||
                    filename.endsWith('.stp') ||
                    filename.endsWith('.glb')

                if (!supported) {
                    notFoundMessage.value = 'Preview is not supported for this file format yet.'
                    state.value = 'not-found'
                    return
                }

                let jobs: ConversionJob[]

                try {
                    jobs = await listProjectFileConversionJobs(
                        projectId,
                        projectFileId,
                        controller.signal,
                    )
                } catch (error) {
                    if (!isCurrent()) {
                        return
                    }

                    if (handleAuthentication(error)) {
                        return
                    }

                    showError('Unable to check preview preparation. Please try again.')
                    return
                }

                if (!isCurrent()) {
                    return
                }

                const activeJob = jobs.find(
                    (job) => job.status === 'pending' || job.status === 'running',
                )

                if (activeJob !== undefined) {
                    await followJob(activeJob)
                    return
                }

                const latestJob = jobs[0]

                if (latestJob?.status === 'failed') {
                    showError(
                        'Preview generation failed. The source file may be unsupported or invalid.',
                    )
                    return
                }

                if (latestJob?.status === 'succeeded') {
                    showError('The conversion succeeded, but its preview is unavailable.')
                    return
                }

                // Only an empty conversion history can trigger
                // automatic creation of a new job.
                if (jobs.length !== 0) {
                    showError('Unable to verify the existing preview conversion.')
                    return
                }

                try {
                    const createdJob = await createProjectFileConversionJob(
                        projectId,
                        projectFileId,
                        controller.signal,
                    )

                    if (!isCurrent()) {
                        return
                    }

                    await followJob(createdJob)
                } catch (error) {
                    if (!isCurrent()) {
                        return
                    }

                    if (handleAuthentication(error)) {
                        return
                    }

                    if (error instanceof ConversionJobRequestError && error.status === 409) {
                        // Another request already created a job.
                        await reconcileCreation(false)
                        return
                    }

                    if (error instanceof ConversionJobRequestError && error.status < 500) {
                        showError('Unable to start preview preparation.')
                        return
                    }

                    // A network error, HTTP 5xx, or unexpected
                    // response can occur after the server created
                    // a job. Inspect history once, never repost.
                    await reconcileCreation(true)
                }
            } catch (error) {
                if (!isCurrent()) {
                    return
                }

                if (handleAuthentication(error)) {
                    return
                }

                showError('The preview could not be loaded.')
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
            {{ notFoundMessage }}
        </p>

        <p v-else-if="state === 'preparing'" data-testid="preview-preparing" role="status">
            Preparing preview…
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
