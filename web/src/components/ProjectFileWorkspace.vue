<script setup lang="ts">
    import { defineAsyncComponent, ref, watch } from 'vue'

    import type { ProjectFile } from '../api/projectFiles'
    import ProjectFilesPanel from './ProjectFilesPanel.vue'
    import AppButton from './ui/AppButton.vue'

    const ProjectFilePreview = defineAsyncComponent(() => import('./ProjectFilePreview.vue'))

    type FileScope = 'revision' | 'uploaded'

    const props = defineProps<{
        projectId: string
        branchId?: string | null
        headRevisionId?: string | null
    }>()

    const emit = defineEmits<{
        unauthorized: []
        'add-file': []
    }>()

    const selectedFile = ref<ProjectFile | null>(null)
    const scope = ref<FileScope>(props.branchId ? 'revision' : 'uploaded')

    function setScope(nextScope: FileScope): void {
        if (scope.value === nextScope) {
            return
        }

        selectedFile.value = null
        scope.value = nextScope
    }

    function selectFile(file: ProjectFile): void {
        if (file.projectId !== props.projectId) {
            return
        }

        selectedFile.value = file
    }

    function closePreview(): void {
        selectedFile.value = null
    }

    watch(
        () => [props.projectId, props.branchId] as const,
        () => {
            selectedFile.value = null
            scope.value = props.branchId ? 'revision' : 'uploaded'
        },
    )

    watch(
        () => props.headRevisionId,
        () => {
            selectedFile.value = null
        },
    )
</script>

<template>
    <div
        class="project-file-workspace"
        :class="{ 'project-file-workspace--split': selectedFile !== null }"
    >
        <div class="project-file-workspace__files">
            <div
                v-if="props.branchId"
                class="project-file-workspace__scopes"
                role="group"
                aria-label="File scope"
            >
                <AppButton
                    :variant="scope === 'revision' ? 'primary' : 'secondary'"
                    aria-label="Current revision"
                    :aria-pressed="scope === 'revision'"
                    @click="setScope('revision')"
                >
                    Current revision
                </AppButton>

                <AppButton
                    :variant="scope === 'uploaded' ? 'primary' : 'secondary'"
                    aria-label="All uploaded files"
                    :aria-pressed="scope === 'uploaded'"
                    @click="setScope('uploaded')"
                >
                    All uploaded files
                </AppButton>
            </div>

            <ProjectFilesPanel
                :project-id="props.projectId"
                :scope="scope"
                :revision-id="scope === 'revision' ? (props.headRevisionId ?? null) : undefined"
                :selected-file-id="selectedFile?.id ?? null"
                @select-file="selectFile"
                @unauthorized="emit('unauthorized')"
                @add-file="emit('add-file')"
            />
        </div>

        <section
            v-if="selectedFile"
            class="project-file-workspace__preview"
            aria-labelledby="project-file-preview-heading"
        >
            <header class="project-file-workspace__preview-header">
                <h3 id="project-file-preview-heading">
                    Preview: {{ selectedFile.originalFilename }}
                </h3>
                <AppButton variant="secondary" aria-label="Close preview" @click="closePreview">
                    Close preview
                </AppButton>
            </header>

            <ProjectFilePreview
                :project-id="props.projectId"
                :project-file-id="selectedFile.id"
                :source-filename="selectedFile.originalFilename"
                @unauthorized="emit('unauthorized')"
            />
        </section>
    </div>
</template>

<style scoped>
    .project-file-workspace {
        min-width: 0;
    }

    .project-file-workspace--split {
        display: grid;
        grid-template-columns: minmax(0, 19fr) minmax(0, 31fr);
        align-items: start;
        gap: var(--space-4);
    }

    .project-file-workspace__files {
        min-width: 0;
    }

    .project-file-workspace__scopes {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-2);
        margin-top: var(--space-4);
    }

    .project-file-workspace__preview {
        min-width: 0;
        margin-top: var(--space-6);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        overflow: hidden;
    }

    .project-file-workspace__preview-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        flex-wrap: wrap;
        gap: var(--space-2);
        padding: var(--space-3) var(--space-4);
        border-bottom: 1px solid var(--color-border);
        background: var(--color-surface);
    }

    .project-file-workspace__preview-header h3 {
        margin: 0;
        font-size: var(--font-size-base);
        overflow-wrap: anywhere;
    }

    @media (max-width: 52rem) {
        .project-file-workspace--split {
            grid-template-columns: minmax(0, 1fr);
        }
    }
</style>
