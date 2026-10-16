<script setup lang="ts">
    import { onUnmounted, ref, watch } from 'vue'

    import {
        listProjectFiles,
        ProjectFileRequestError,
        type ProjectFile,
    } from '../api/projectFiles'
    import AppButton from './ui/AppButton.vue'

    const props = defineProps<{
        projectId: string
    }>()

    const emit = defineEmits<{
        unauthorized: []
    }>()

    type LoadState = 'loading' | 'ready' | 'error' | 'unauthorized'

    const state = ref<LoadState>('loading')
    const files = ref<ProjectFile[]>([])

    const dateFormatter = new Intl.DateTimeFormat('en-CA', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
    })

    let activeController: AbortController | null = null

    function formatUploadedAt(value: string): string {
        const date = new Date(value)

        if (Number.isNaN(date.getTime())) {
            return 'Upload date unavailable'
        }

        return `Uploaded ${dateFormatter.format(date)}`
    }

    async function loadFiles(projectId: string): Promise<void> {
        activeController?.abort()

        const controller = new AbortController()
        activeController = controller

        files.value = []
        state.value = 'loading'

        try {
            const result = await listProjectFiles(projectId, controller.signal)

            if (controller.signal.aborted || activeController !== controller) {
                return
            }

            // Do not display files belonging to a different project.
            if (result.some((file) => file.projectId !== projectId)) {
                state.value = 'error'
                return
            }

            files.value = result
            state.value = 'ready'
        } catch (error) {
            if (controller.signal.aborted || activeController !== controller) {
                return
            }

            if (error instanceof ProjectFileRequestError && error.status === 401) {
                state.value = 'unauthorized'
                emit('unauthorized')
                return
            }

            state.value = 'error'
        } finally {
            if (activeController === controller) {
                activeController = null
            }
        }
    }

    watch(
        () => props.projectId,
        (projectId) => {
            void loadFiles(projectId)
        },
        { immediate: true },
    )

    onUnmounted(() => {
        activeController?.abort()
        activeController = null
    })
</script>

<template>
    <section class="project-files-panel" aria-labelledby="project-files-heading">
        <div class="project-files-panel__header">
            <h3 id="project-files-heading">Project files</h3>

            <span v-if="state === 'ready'" class="project-files-panel__count">
                {{ files.length }} {{ files.length === 1 ? 'file' : 'files' }}
            </span>
        </div>

        <p v-if="state === 'loading'" role="status">Loading files...</p>

        <div v-else-if="state === 'error'" class="project-files-panel__feedback">
            <p role="alert">Unable to load files. Please try again.</p>
            <AppButton variant="secondary" @click="loadFiles(props.projectId)">
                Try again
            </AppButton>
        </div>

        <p v-else-if="state === 'unauthorized'" role="status">
            Session expired. Please sign in again.
        </p>

        <p v-else-if="files.length === 0" class="project-files-panel__empty">
            No files uploaded yet.
        </p>

        <ul v-else class="project-files-panel__list" aria-label="Project files">
            <li v-for="file in files" :key="file.id" class="project-files-panel__item">
                <div class="project-files-panel__name">
                    {{ file.originalFilename }}
                </div>

                <div class="project-files-panel__metadata">
                    <span>{{ file.mediaType ?? 'File type unavailable' }}</span>
                    <time :datetime="file.createdAt">
                        {{ formatUploadedAt(file.createdAt) }}
                    </time>
                </div>
            </li>
        </ul>
    </section>
</template>

<style scoped>
    .project-files-panel {
        min-width: 0;
        margin-top: var(--space-6);
    }

    .project-files-panel__header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        flex-wrap: wrap;
        gap: var(--space-3);
    }

    .project-files-panel__header h3 {
        margin: 0;
    }

    .project-files-panel__count,
    .project-files-panel__empty,
    .project-files-panel__metadata {
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }

    .project-files-panel__list {
        margin: var(--space-4) 0 0;
        padding: 0;
        list-style: none;
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
    }

    .project-files-panel__item {
        display: flex;
        align-items: start;
        justify-content: space-between;
        flex-wrap: wrap;
        gap: var(--space-2) var(--space-4);
        padding: var(--space-4);
        min-width: 0;
    }

    .project-files-panel__item + .project-files-panel__item {
        border-top: 1px solid var(--color-border);
    }

    .project-files-panel__name {
        font-weight: var(--font-weight-medium);
        overflow-wrap: anywhere;
        min-width: 0;
    }

    .project-files-panel__metadata {
        display: flex;
        align-items: start;
        justify-content: space-between;
        flex-wrap: wrap;
        gap: var(--space-2) var(--space-4);
        overflow-wrap: anywhere;
    }

    .project-files-panel__feedback,
    .project-files-panel__empty {
        margin-top: var(--space-4);
    }

    .project-files-panel__feedback p {
        color: var(--color-danger);
    }
</style>
