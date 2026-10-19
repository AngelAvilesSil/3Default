<script setup lang="ts">
    import { computed, onUnmounted, ref, watch } from 'vue'
    import { useRouter } from 'vue-router'

    import { ProjectFileRequestError, uploadProjectFile } from '../api/projectFiles'
    import AppButton from '../components/ui/AppButton.vue'
    import { useSessionStore } from '../stores/session'

    const props = defineProps<{
        projectId: string
    }>()

    const router = useRouter()

    const fileInput = ref<HTMLInputElement | null>(null)
    const selectedFiles = ref<File[]>([])
    const uploading = ref(false)

    type UploadStatus = 'selected' | 'uploading' | 'uploaded' | 'failed' | 'needs-review'

    const fileStatuses = ref<Record<string, UploadStatus>>({})

    let activeController: AbortController | null = null

    const hasPendingFiles = computed(() =>
        selectedFiles.value.some((file) => fileStatus(file) === 'selected'),
    )

    const hasFailedFiles = computed(() =>
        selectedFiles.value.some((file) => fileStatus(file) === 'failed'),
    )

    const hasUncertainFiles = computed(() =>
        selectedFiles.value.some((file) => fileStatus(file) === 'needs-review'),
    )

    const hasUploadResults = computed(() =>
        selectedFiles.value.some((file) => {
            const status = fileStatus(file)

            return status === 'uploaded' || status === 'failed' || status === 'needs-review'
        }),
    )

    function fileStatus(file: File): UploadStatus {
        return fileStatuses.value[fileKey(file)] ?? 'selected'
    }

    function setFileStatus(file: File, status: UploadStatus): void {
        fileStatuses.value[fileKey(file)] = status
    }

    function statusLabel(file: File): string {
        switch (fileStatus(file)) {
            case 'uploading':
                return 'Uploading'
            case 'uploaded':
                return 'Uploaded'
            case 'failed':
                return 'Failed'
            case 'needs-review':
                return 'Needs review'
            default:
                return 'Ready to upload'
        }
    }

    async function startUploads(retryFailed = false): Promise<void> {
        if (uploading.value) {
            return
        }

        const requiredStatus: UploadStatus = retryFailed ? 'failed' : 'selected'

        const candidates = selectedFiles.value.filter((file) => fileStatus(file) === requiredStatus)

        if (candidates.length === 0) {
            return
        }

        const projectId = props.projectId
        const controller = new AbortController()

        activeController = controller
        uploading.value = true

        try {
            for (const file of candidates) {
                if (
                    controller.signal.aborted ||
                    activeController !== controller ||
                    props.projectId !== projectId
                ) {
                    return
                }

                setFileStatus(file, 'uploading')

                try {
                    await uploadProjectFile(projectId, file, controller.signal)

                    if (
                        controller.signal.aborted ||
                        activeController !== controller ||
                        props.projectId !== projectId
                    ) {
                        return
                    }

                    setFileStatus(file, 'uploaded')
                } catch (error) {
                    if (
                        controller.signal.aborted ||
                        activeController !== controller ||
                        props.projectId !== projectId
                    ) {
                        return
                    }

                    if (error instanceof ProjectFileRequestError && error.status === 401) {
                        // The server confirmed that the session is invalid.
                        // Stop this batch and preserve the intended page.
                        useSessionStore().invalidate()

                        const returnPath = router.resolve({
                            name: 'project-file-upload',
                            params: { projectId },
                        }).fullPath

                        void router.push({
                            name: 'login',
                            query: { redirect: returnPath },
                        })

                        return
                    }

                    if (error instanceof ProjectFileRequestError && error.status === 400) {
                        // Confirmed rejection: safe to offer a retry.
                        setFileStatus(file, 'failed')
                        continue
                    }

                    // An unexpected response or network interruption
                    // does not prove that the server rejected the file.
                    // Stop this batch rather than producing more
                    // uncertain outcomes.
                    setFileStatus(file, 'needs-review')
                    return
                }
            }
        } finally {
            if (activeController === controller) {
                activeController = null
                uploading.value = false
            }
        }
    }

    // Selection identity is a convenience heuristic, not a content hash.
    function fileKey(file: File): string {
        return JSON.stringify([file.name, file.size, file.lastModified, file.type])
    }

    function addFiles(files: FileList | File[] | null): void {
        if (files === null || uploading.value) {
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
        if (uploading.value || (fileStatus(file) !== 'selected' && fileStatus(file) !== 'failed')) {
            return
        }

        const key = fileKey(file)

        selectedFiles.value = selectedFiles.value.filter((candidate) => fileKey(candidate) !== key)

        delete fileStatuses.value[key]
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
            activeController?.abort()
            activeController = null
            uploading.value = false

            selectedFiles.value = []
            fileStatuses.value = {}

            if (fileInput.value !== null) {
                fileInput.value.value = ''
            }
        },
    )

    onUnmounted(() => {
        activeController?.abort()
        activeController = null
    })
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

                <AppButton
                    variant="secondary"
                    aria-label="Choose files"
                    :disabled="uploading"
                    @click="openPicker"
                >
                    Choose files
                </AppButton>

                <input
                    ref="fileInput"
                    class="project-file-upload__file-input"
                    type="file"
                    multiple
                    :disabled="uploading"
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
                        <span
                            v-if="fileStatus(file) !== 'selected'"
                            class="project-file-upload__status"
                            :class="`project-file-upload__status--${fileStatus(file)}`"
                            aria-live="polite"
                        >
                            {{ statusLabel(file) }}
                        </span>
                    </div>

                    <AppButton
                        v-if="fileStatus(file) === 'selected' || fileStatus(file) === 'failed'"
                        variant="secondary"
                        :disabled="uploading"
                        :aria-label="`Remove ${file.name}`"
                        @click="removeFile(file)"
                    >
                        Remove
                    </AppButton>
                </li>
            </ul>

            <p v-if="hasUncertainFiles" class="project-file-upload__review-message" role="alert">
                Some uploads have an uncertain result. Check the project file directory before
                attempting those files again.
            </p>

            <div class="project-file-upload__actions">
                <RouterLink
                    v-if="!uploading && hasUploadResults"
                    class="project-file-upload__view-files"
                    :to="{
                        name: 'project-overview',
                        params: { projectId: props.projectId },
                    }"
                    aria-label="View project files"
                >
                    View project files
                </RouterLink>

                <AppButton
                    v-if="hasFailedFiles"
                    variant="secondary"
                    aria-label="Retry failed uploads"
                    :disabled="uploading || hasUncertainFiles"
                    @click="startUploads(true)"
                >
                    Retry failed uploads
                </AppButton>

                <AppButton
                    aria-label="Upload selected files"
                    :disabled="uploading || !hasPendingFiles || hasUncertainFiles"
                    :loading="uploading"
                    @click="startUploads()"
                >
                    {{ uploading ? 'Uploading files…' : 'Upload files' }}
                </AppButton>
            </div>

            <p class="project-file-upload__notice">
                Selecting files does not upload them. Uploading stores source files without creating
                revisions or generating previews.
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

    .project-file-upload__status {
        font-weight: var(--font-weight-medium);
    }

    .project-file-upload__status--uploaded {
        color: var(--color-text);
    }

    .project-file-upload__status--failed,
    .project-file-upload__status--needs-review {
        color: var(--color-danger);
    }

    .project-file-upload__review-message {
        margin-top: var(--space-4);
        color: var(--color-danger);
    }

    .project-file-upload__actions {
        display: flex;
        justify-content: flex-end;
        flex-wrap: wrap;
        gap: var(--space-3);
        margin-top: var(--space-5);
    }

    .project-file-upload__view-files {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-height: 2.5rem;
        padding: var(--space-2) var(--space-4);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-sm);
        color: var(--color-text);
        background: var(--color-surface);
        font-size: var(--font-size-sm);
        font-weight: var(--font-weight-semibold);
        line-height: var(--line-height-body);
        text-decoration: none;
    }

    .project-file-upload__view-files:hover {
        background: var(--color-page);
    }

    .project-file-upload__view-files:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
    }

    @media (max-width: 35rem) {
        .project-file-upload__item {
            align-items: flex-start;
            flex-wrap: wrap;
        }
    }
</style>
