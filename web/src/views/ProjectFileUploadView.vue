<script setup lang="ts">
    import { ref, watch } from 'vue'

    import AppButton from '../components/ui/AppButton.vue'

    const props = defineProps<{
        projectId: string
    }>()

    const fileInput = ref<HTMLInputElement | null>(null)
    const selectedFiles = ref<File[]>([])

    // Selection identity is a convenience heuristic, not a content hash.
    function fileKey(file: File): string {
        return JSON.stringify([file.name, file.size, file.lastModified, file.type])
    }

    function addFiles(files: FileList | File[] | null): void {
        if (files === null) {
            return
        }

        const existing = new Set(selectedFiles.value.map(fileKey))
        const additions: File[] = []

        for (const file of Array.from(files)) {
            const key = fileKey(file)

            if (existing.has(key)) {
                continue
            }

            existing.add(key)
            additions.push(file)
        }

        if (additions.length > 0) {
            selectedFiles.value = [...selectedFiles.value, ...additions]
        }
    }

    function openPicker(): void {
        fileInput.value?.click()
    }

    function handlePickerChange(event: Event): void {
        const input = event.target

        if (!(input instanceof HTMLInputElement)) {
            return
        }

        addFiles(input.files)

        // Permit selecting the same file again after removing it.
        input.value = ''
    }

    function handleDrop(event: DragEvent): void {
        addFiles(event.dataTransfer?.files ?? null)
    }

    function removeFile(file: File): void {
        const key = fileKey(file)

        selectedFiles.value = selectedFiles.value.filter((candidate) => fileKey(candidate) !== key)
    }

    function formatFileSize(bytes: number): string {
        const units = ['B', 'KB', 'MB', 'GB', 'TB']
        let value = bytes
        let unitIndex = 0

        while (value >= 1024 && unitIndex < units.length - 1) {
            value /= 1024
            unitIndex++
        }

        const readable = Number.isInteger(value) ? String(value) : value.toFixed(1)

        return `${readable} ${units[unitIndex]}`
    }

    watch(
        () => props.projectId,
        () => {
            selectedFiles.value = []

            if (fileInput.value !== null) {
                fileInput.value.value = ''
            }
        },
    )
</script>

<template>
    <section class="project-file-upload" aria-labelledby="project-file-upload-heading">
        <RouterLink
            class="project-file-upload__back"
            :to="{
                name: 'project-overview',
                params: { projectId: props.projectId },
            }"
        >
            Back to project
        </RouterLink>

        <header class="project-file-upload__header">
            <h2 id="project-file-upload-heading">Upload files</h2>
            <p>
                Select source files for this project and review them before uploading. Files are not
                uploaded automatically.
            </p>
        </header>

        <div class="project-file-upload__selection">
            <div
                class="project-file-upload__drop-zone"
                data-testid="file-drop-zone"
                @dragover.prevent
                @drop.prevent="handleDrop"
            >
                <p class="project-file-upload__drop-title">Drag files here or choose files</p>

                <AppButton variant="secondary" aria-label="Choose files" @click="openPicker">
                    Choose files
                </AppButton>

                <input
                    ref="fileInput"
                    class="project-file-upload__file-input"
                    type="file"
                    multiple
                    tabindex="-1"
                    aria-label="Select files"
                    @change="handlePickerChange"
                />

                <p class="project-file-upload__drop-help">
                    Files stay on your computer until you explicitly upload them.
                </p>
            </div>

            <div class="project-file-upload__selected-header">
                <h3>Selected files</h3>

                <span class="project-file-upload__count" aria-live="polite">
                    {{ selectedFiles.length }}
                    {{ selectedFiles.length === 1 ? 'file' : 'files' }}
                </span>
            </div>

            <p v-if="selectedFiles.length === 0" class="project-file-upload__empty">
                No files selected yet.
            </p>

            <ul v-else class="project-file-upload__list" aria-label="Selected files">
                <li
                    v-for="file in selectedFiles"
                    :key="fileKey(file)"
                    class="project-file-upload__item"
                >
                    <div class="project-file-upload__file-details">
                        <strong>{{ file.name }}</strong>
                        <span>{{ formatFileSize(file.size) }}</span>
                    </div>

                    <AppButton
                        variant="secondary"
                        :aria-label="`Remove ${file.name}`"
                        @click="removeFile(file)"
                    >
                        Remove
                    </AppButton>
                </li>
            </ul>

            <p class="project-file-upload__notice">
                Selecting files does not upload them or create a project revision.
            </p>
        </div>
    </section>
</template>

<style scoped>
    .project-file-upload {
        max-width: 62rem;
        margin-inline: auto;
        padding: var(--space-6) var(--space-4);
    }

    .project-file-upload__back {
        display: inline-flex;
        padding: var(--space-2) 0;
        color: var(--color-action);
        font-weight: var(--font-weight-medium);
        text-decoration: none;
    }

    .project-file-upload__back:hover {
        text-decoration: underline;
    }

    .project-file-upload__back:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
        border-radius: var(--radius-sm);
    }

    .project-file-upload__header {
        margin-top: var(--space-4);
    }

    .project-file-upload__header h2 {
        margin: 0;
    }

    .project-file-upload__header p {
        margin: var(--space-2) 0 0;
        color: var(--color-text-muted);
    }

    .project-file-upload__selection {
        margin-top: var(--space-5);
    }

    .project-file-upload__drop-zone {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: var(--space-3);
        padding: var(--space-6);
        border: 2px dashed var(--color-border);
        border-radius: var(--radius-md);
        background: var(--color-surface);
        text-align: center;
    }

    .project-file-upload__drop-title {
        margin: 0;
        font-weight: var(--font-weight-semibold);
    }

    .project-file-upload__drop-help {
        margin: 0;
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }

    .project-file-upload__file-input {
        position: absolute;
        width: 1px;
        height: 1px;
        overflow: hidden;
        clip-path: inset(50%);
        white-space: nowrap;
    }

    .project-file-upload__selected-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        flex-wrap: wrap;
        gap: var(--space-3);
        margin-top: var(--space-5);
    }

    .project-file-upload__selected-header h3 {
        margin: 0;
        font-size: var(--font-size-base);
    }

    .project-file-upload__count,
    .project-file-upload__empty,
    .project-file-upload__notice {
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }

    .project-file-upload__list {
        margin: var(--space-3) 0 0;
        padding: 0;
        list-style: none;
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        overflow: hidden;
    }

    .project-file-upload__item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: var(--space-3);
        padding: var(--space-3);
    }

    .project-file-upload__item + .project-file-upload__item {
        border-top: 1px solid var(--color-border);
    }

    .project-file-upload__file-details {
        display: flex;
        flex-direction: column;
        min-width: 0;
        gap: var(--space-1);
    }

    .project-file-upload__file-details strong {
        overflow-wrap: anywhere;
    }

    .project-file-upload__file-details span {
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }

    .project-file-upload__notice {
        margin-top: var(--space-4);
    }

    @media (max-width: 35rem) {
        .project-file-upload__item {
            align-items: flex-start;
            flex-wrap: wrap;
        }
    }
</style>
