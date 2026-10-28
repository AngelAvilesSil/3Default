<script setup lang="ts">
    import { onUnmounted, ref, watch } from 'vue'

    import type { ProjectFile } from '../api/projectFiles'
    import { listProjectRevisionFiles, RevisionRequestError } from '../api/revisions'
    import AppButton from './ui/AppButton.vue'

    const props = defineProps<{
        projectId: string
        revisionId: string
    }>()

    const emit = defineEmits<{
        unauthorized: []
    }>()

    type LoadState = 'loading' | 'ready' | 'error' | 'unavailable'

    const state = ref<LoadState>('loading')
    const files = ref<ProjectFile[]>([])

    let activeController: AbortController | null = null

    function isCurrent(
        controller: AbortController,
        projectId: string,
        revisionId: string,
    ): boolean {
        return (
            !controller.signal.aborted &&
            activeController === controller &&
            props.projectId === projectId &&
            props.revisionId === revisionId
        )
    }

    async function loadFiles(): Promise<void> {
        activeController?.abort()

        const controller = new AbortController()
        activeController = controller

        const projectId = props.projectId
        const revisionId = props.revisionId

        state.value = 'loading'
        files.value = []

        if (!projectId.trim() || !revisionId.trim()) {
            state.value = 'unavailable'
            activeController = null
            return
        }

        try {
            const result = await listProjectRevisionFiles(projectId, revisionId, controller.signal)

            if (!isCurrent(controller, projectId, revisionId)) {
                return
            }

            if (result.some((file) => file.projectId !== projectId)) {
                state.value = 'error'
                return
            }

            files.value = result
            state.value = 'ready'
        } catch (error) {
            if (!isCurrent(controller, projectId, revisionId)) {
                return
            }

            if (error instanceof RevisionRequestError && error.status === 401) {
                state.value = 'unavailable'
                emit('unauthorized')
                return
            }

            if (
                error instanceof RevisionRequestError &&
                (error.status === 400 || error.status === 403 || error.status === 404)
            ) {
                state.value = 'unavailable'
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
        () => [props.projectId, props.revisionId],
        () => {
            void loadFiles()
        },
        { immediate: true },
    )

    onUnmounted(() => {
        activeController?.abort()
        activeController = null
    })
</script>

<template>
    <section class="branch-head-files" aria-labelledby="branch-head-files-heading">
        <h4 id="branch-head-files-heading">Files in head revision</h4>

        <p class="branch-head-files__note">
            Files saved in the selected head revision when its information was loaded. This is
            separate from all files uploaded to the project.
        </p>

        <p v-if="state === 'loading'" role="status">Loading head revision files...</p>

        <div v-else-if="state === 'error'" class="branch-head-files__feedback">
            <p role="alert">Unable to load head revision files. Please try again.</p>

            <AppButton variant="secondary" @click="loadFiles()"> Try again </AppButton>
        </div>

        <p v-else-if="state === 'unavailable'" role="alert">
            This revision's files are unavailable or not accessible.
        </p>

        <p v-else-if="files.length === 0" class="branch-head-files__empty">
            This revision contains no source files.
        </p>

        <ul v-else class="branch-head-files__list" aria-label="Files in head revision">
            <li v-for="file in files" :key="file.id">
                {{ file.originalFilename }}
            </li>
        </ul>
    </section>
</template>

<style scoped>
    .branch-head-files {
        margin-top: var(--space-4);
        padding-top: var(--space-4);
        border-top: 1px solid var(--color-border);
    }

    .branch-head-files h4 {
        margin: 0;
    }

    .branch-head-files__note,
    .branch-head-files__empty {
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }

    .branch-head-files__list {
        margin: var(--space-3) 0 0;
        padding-left: var(--space-5);
    }

    .branch-head-files__list li {
        overflow-wrap: anywhere;
    }

    .branch-head-files__list li + li {
        margin-top: var(--space-2);
    }

    .branch-head-files__feedback p {
        color: var(--color-danger);
    }
</style>
