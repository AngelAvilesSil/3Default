<script setup lang="ts">
    import { computed, onUnmounted, ref, watch } from 'vue'

    import {
        listProjectFiles,
        ProjectFileRequestError,
        type ProjectFile,
    } from '../api/projectFiles'
    import { listProjectRevisionFiles, RevisionRequestError } from '../api/revisions'
    import AppButton from './ui/AppButton.vue'

    type FileScope = 'uploaded' | 'revision'

    const props = defineProps<{
        projectId: string
        selectedFileId?: string | null
        scope?: FileScope
        revisionId?: string | null
    }>()

    const emit = defineEmits<{
        unauthorized: []
        'select-file': [file: ProjectFile]
        'add-file': []
    }>()

    type LoadState = 'loading' | 'ready' | 'error' | 'unauthorized' | 'unavailable' | 'empty-branch'

    const state = ref<LoadState>('loading')
    const files = ref<ProjectFile[]>([])

    // Keep the same visible file order regardless of API response order.
    // Revision membership and the original response array are unchanged.
    const fileNameCollator = new Intl.Collator('en', {
        numeric: true,
        sensitivity: 'base',
    })

    const orderedFiles = computed(() =>
        [...files.value].sort(
            (first, second) =>
                fileNameCollator.compare(first.originalFilename, second.originalFilename) ||
                first.id.localeCompare(second.id),
        ),
    )

    const directoryHeading = computed(() => {
        if (props.scope === 'revision') {
            return 'Files in current revision'
        }

        if (props.scope === 'uploaded') {
            return 'All uploaded files'
        }

        return 'Project files'
    })

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

    async function loadFiles(
        projectId: string,
        scope: FileScope,
        revisionId: string | null | undefined,
    ): Promise<void> {
        activeController?.abort()

        const controller = new AbortController()
        activeController = controller

        files.value = []
        state.value = 'loading'

        const isCurrent = () =>
            !controller.signal.aborted &&
            activeController === controller &&
            props.projectId === projectId &&
            (props.scope ?? 'uploaded') === scope &&
            (scope !== 'revision' || props.revisionId === revisionId)

        try {
            let result: ProjectFile[]

            if (scope === 'revision') {
                if (!revisionId?.trim()) {
                    state.value = 'empty-branch'
                    return
                }

                result = await listProjectRevisionFiles(projectId, revisionId, controller.signal)
            } else {
                result = await listProjectFiles(projectId, controller.signal)
            }

            if (!isCurrent()) {
                return
            }

            // Never display files associated with another project.
            if (result.some((file) => file.projectId !== projectId)) {
                state.value = 'error'
                return
            }

            files.value = result
            state.value = 'ready'
        } catch (error) {
            if (!isCurrent()) {
                return
            }

            if (
                (error instanceof ProjectFileRequestError ||
                    error instanceof RevisionRequestError) &&
                error.status === 401
            ) {
                state.value = 'unauthorized'
                emit('unauthorized')
                return
            }

            if (
                scope === 'revision' &&
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

    function retryFiles(): void {
        void loadFiles(props.projectId, props.scope ?? 'uploaded', props.revisionId)
    }

    watch(
        () => [props.projectId, props.scope ?? 'uploaded', props.revisionId] as const,
        ([projectId, scope, revisionId]) => {
            void loadFiles(projectId, scope, revisionId)
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
            <h3 id="project-files-heading">{{ directoryHeading }}</h3>

            <div class="project-files-panel__header-actions">
                <span v-if="state === 'ready'" class="project-files-panel__count">
                    {{ files.length }} {{ files.length === 1 ? 'file' : 'files' }}
                </span>

                <AppButton variant="secondary" aria-label="Add file" @click="emit('add-file')">
                    Add file
                </AppButton>
            </div>
        </div>

        <p v-if="state === 'loading'" role="status">Loading files...</p>

        <div v-else-if="state === 'error'" class="project-files-panel__feedback">
            <p role="alert">Unable to load files. Please try again.</p>
            <AppButton variant="secondary" @click="retryFiles()">Try again</AppButton>
        </div>

        <p v-else-if="state === 'unauthorized'" role="status">
            Session expired. Please sign in again.
        </p>

        <p v-else-if="state === 'empty-branch'" class="project-files-panel__empty">
            This branch has no revisions yet.
        </p>

        <p v-else-if="state === 'unavailable'" role="alert">
            This revision's files are unavailable or not accessible.
        </p>

        <p
            v-else-if="files.length === 0 && props.scope === 'revision'"
            class="project-files-panel__empty"
        >
            This revision contains no source files.
        </p>

        <p v-else-if="files.length === 0" class="project-files-panel__empty">
            No files uploaded yet.
        </p>

        <div v-else class="project-files-panel__directory">
            <div class="project-files-panel__columns" aria-hidden="true">
                <span>Name</span>
                <span>Uploaded</span>
            </div>

            <ul class="project-files-panel__list" aria-label="Project files">
                <li v-for="file in orderedFiles" :key="file.id" class="project-files-panel__item">
                    <button
                        type="button"
                        class="project-files-panel__select"
                        :aria-pressed="props.selectedFileId === file.id"
                        @click="emit('select-file', file)"
                    >
                        <span class="project-files-panel__name">
                            <svg
                                class="project-files-panel__icon"
                                aria-hidden="true"
                                viewBox="0 0 16 16"
                                fill="none"
                                stroke="currentColor"
                                stroke-width="1.5"
                                stroke-linecap="round"
                                stroke-linejoin="round"
                            >
                                <path
                                    d="M4 1.75h5l3 3v8.5a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1v-10a1 1 0 0 1 1-1Z"
                                />
                                <path d="M9 1.75v3h3" />
                            </svg>
                            <span>{{ file.originalFilename }}</span>
                        </span>

                        <time class="project-files-panel__metadata" :datetime="file.createdAt">
                            {{ formatUploadedAt(file.createdAt) }}
                        </time>
                    </button>
                </li>
            </ul>
        </div>
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

    .project-files-panel__header-actions {
        display: flex;
        align-items: center;
        flex-wrap: wrap;
        gap: var(--space-3);
    }

    .project-files-panel__count,
    .project-files-panel__empty,
    .project-files-panel__metadata {
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }

    .project-files-panel__directory {
        margin-top: var(--space-4);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        overflow: hidden;
    }

    .project-files-panel__columns {
        display: grid;
        grid-template-columns: minmax(0, 1fr) auto;
        gap: var(--space-3);
        padding: var(--space-2) var(--space-3);
        border-bottom: 1px solid var(--color-border);
        background: var(--color-surface);
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
        font-weight: var(--font-weight-medium);
    }

    .project-files-panel__list {
        margin: 0;
        padding: 0;
        list-style: none;
    }

    .project-files-panel__item {
        min-width: 0;
    }

    .project-files-panel__select {
        display: grid;
        grid-template-columns: minmax(0, 1fr) auto;
        align-items: center;
        gap: var(--space-3);
        width: 100%;
        min-width: 0;
        padding: var(--space-2) var(--space-3);
        border: 0;
        color: inherit;
        background: transparent;
        font: inherit;
        text-align: left;
        cursor: pointer;
    }

    .project-files-panel__select:hover {
        background: var(--color-surface);
    }

    .project-files-panel__select[aria-pressed='true'] {
        background: var(--color-surface);
        box-shadow: inset var(--space-1) 0 0 var(--color-action);
    }

    .project-files-panel__select:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: -3px;
    }

    .project-files-panel__item + .project-files-panel__item {
        border-top: 1px solid var(--color-border);
    }

    .project-files-panel__name {
        display: flex;
        align-items: center;
        gap: var(--space-2);
        min-width: 0;
        font-weight: var(--font-weight-medium);
    }

    .project-files-panel__name > span {
        min-width: 0;
        overflow-wrap: anywhere;
    }

    .project-files-panel__icon {
        width: 1rem;
        height: 1rem;
        flex-shrink: 0;
        color: var(--color-text-muted);
    }

    .project-files-panel__metadata {
        white-space: nowrap;
        text-align: right;
    }

    .project-files-panel__feedback,
    .project-files-panel__empty {
        margin-top: var(--space-4);
    }

    .project-files-panel__feedback p {
        color: var(--color-danger);
    }

    @media (max-width: 52rem) {
        .project-files-panel__columns {
            grid-template-columns: minmax(0, 1fr);
        }

        .project-files-panel__columns span:last-child {
            display: none;
        }

        .project-files-panel__select {
            grid-template-columns: minmax(0, 1fr);
            gap: var(--space-1);
        }

        .project-files-panel__metadata {
            padding-left: calc(var(--space-4) + var(--space-2));
            text-align: left;
        }
    }
</style>
