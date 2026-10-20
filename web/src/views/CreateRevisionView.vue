<script setup lang="ts">
    import { onUnmounted, ref, watch } from 'vue'
    import { useRoute, useRouter } from 'vue-router'

    import {
        listProjectFiles,
        ProjectFileRequestError,
        type ProjectFile,
    } from '../api/projectFiles'
    import {
        getProject,
        listProjectBranches,
        ProjectRequestError,
        type Project,
        type ProjectBranch,
    } from '../api/projects'
    import { listProjectRevisionFiles, RevisionRequestError } from '../api/revisions'
    import AppButton from '../components/ui/AppButton.vue'
    import AppTextArea from '../components/ui/AppTextArea.vue'
    import { useSessionStore } from '../stores/session'

    const props = defineProps<{
        projectId: string
        branchId: string
    }>()

    const route = useRoute()
    const router = useRouter()
    const session = useSessionStore()

    type LoadState = 'loading' | 'ready' | 'error' | 'unavailable'

    const state = ref<LoadState>('loading')
    const project = ref<Project | null>(null)
    const branch = ref<ProjectBranch | null>(null)
    const files = ref<ProjectFile[]>([])
    const selectedFileIds = ref<Set<string>>(new Set())

    const revisionMessage = ref('')
    const messageError = ref('')

    const unavailableMessage = ref('')
    const errorMessage = ref('')

    let activeController: AbortController | null = null

    function isCurrent(controller: AbortController, projectId: string, branchId: string): boolean {
        return (
            !controller.signal.aborted &&
            activeController === controller &&
            props.projectId === projectId &&
            props.branchId === branchId
        )
    }

    function resetDraft(): void {
        project.value = null
        branch.value = null
        files.value = []
        selectedFileIds.value = new Set()
        revisionMessage.value = ''
        messageError.value = ''
        unavailableMessage.value = ''
        errorMessage.value = ''
        state.value = 'loading'
    }

    async function loadDraft(): Promise<void> {
        activeController?.abort()

        const controller = new AbortController()
        activeController = controller

        const projectId = props.projectId
        const branchId = props.branchId

        resetDraft()

        if (!projectId.trim() || !branchId.trim()) {
            state.value = 'unavailable'
            unavailableMessage.value = 'This revision link is invalid.'
            activeController = null
            return
        }

        try {
            const loadedProject = await getProject(projectId, controller.signal)

            if (!isCurrent(controller, projectId, branchId)) {
                return
            }

            if (loadedProject.id !== projectId) {
                throw new Error('Project identity mismatch')
            }

            const [loadedBranches, loadedFiles] = await Promise.all([
                listProjectBranches(projectId, controller.signal),
                listProjectFiles(projectId, controller.signal),
            ])

            if (!isCurrent(controller, projectId, branchId)) {
                return
            }

            if (
                loadedBranches.some((candidate) => candidate.projectId !== projectId) ||
                loadedFiles.some((file) => file.projectId !== projectId)
            ) {
                throw new Error('Project resource mismatch')
            }

            const matchingBranches = loadedBranches.filter((candidate) => candidate.id === branchId)

            if (matchingBranches.length === 0) {
                state.value = 'unavailable'
                unavailableMessage.value = 'This branch was not found or is not available.'
                return
            }

            if (matchingBranches.length !== 1) {
                throw new Error('Duplicate branch identity')
            }

            const selectedBranch = matchingBranches[0]!

            const availableFileIds = new Set(loadedFiles.map((file) => file.id))

            if (availableFileIds.size !== loadedFiles.length) {
                throw new Error('Duplicate project file identity')
            }

            let initialFileIds: string[] = []

            if (selectedBranch.headRevisionId !== null) {
                const snapshot = await listProjectRevisionFiles(
                    projectId,
                    selectedBranch.headRevisionId,
                    controller.signal,
                )

                if (!isCurrent(controller, projectId, branchId)) {
                    return
                }

                initialFileIds = snapshot.map((file) => file.id)

                if (
                    snapshot.some((file) => file.projectId !== projectId) ||
                    new Set(initialFileIds).size !== initialFileIds.length ||
                    initialFileIds.some((id) => !availableFileIds.has(id))
                ) {
                    // Never silently discard an authoritative snapshot file.
                    throw new Error('Revision snapshot is inconsistent')
                }
            }

            if (!isCurrent(controller, projectId, branchId)) {
                return
            }

            project.value = loadedProject
            branch.value = selectedBranch
            files.value = loadedFiles
            selectedFileIds.value = new Set(initialFileIds)
            state.value = 'ready'
        } catch (error) {
            if (!isCurrent(controller, projectId, branchId)) {
                return
            }

            if (
                error instanceof ProjectRequestError ||
                error instanceof ProjectFileRequestError ||
                error instanceof RevisionRequestError
            ) {
                if (error.status === 401) {
                    session.invalidate()

                    await router.replace({
                        name: 'login',
                        query: { redirect: route.fullPath },
                    })
                    return
                }

                if (error.status === 400 || error.status === 404) {
                    state.value = 'unavailable'
                    unavailableMessage.value =
                        'The requested project, branch, or revision is unavailable.'
                    return
                }
            }

            state.value = 'error'
            errorMessage.value = 'Unable to prepare this revision draft. Please try again.'
        } finally {
            if (activeController === controller) {
                activeController = null
            }
        }
    }

    function updateRevisionMessage(value: string): void {
        revisionMessage.value = value
        messageError.value = value.trim() ? '' : 'Revision message is required.'
    }

    function toggleFile(fileId: string, event: Event): void {
        if (state.value !== 'ready') {
            return
        }

        const input = event.target

        if (!(input instanceof HTMLInputElement)) {
            return
        }

        if (!files.value.some((file) => file.id === fileId)) {
            return
        }

        const next = new Set(selectedFileIds.value)

        if (input.checked) {
            next.add(fileId)
        } else {
            next.delete(fileId)
        }

        selectedFileIds.value = next
    }

    watch(
        [() => props.projectId, () => props.branchId],
        () => {
            void loadDraft()
        },
        { immediate: true },
    )

    onUnmounted(() => {
        activeController?.abort()
        activeController = null
    })
</script>

<template>
    <section class="create-revision" aria-labelledby="create-revision-heading">
        <RouterLink
            class="create-revision__back"
            :to="{
                name: 'project-overview',
                params: { projectId: props.projectId },
            }"
        >
            Back to project
        </RouterLink>

        <header class="create-revision__header">
            <h2 id="create-revision-heading">Create revision</h2>
            <p>Choose which uploaded source files belong to this revision.</p>
        </header>

        <p v-if="state === 'loading'" role="status" class="create-revision__feedback">
            Loading revision draft...
        </p>

        <p v-else-if="state === 'unavailable'" role="alert" class="create-revision__feedback">
            {{ unavailableMessage }}
        </p>

        <div v-else-if="state === 'error'" class="create-revision__feedback">
            <p role="alert">{{ errorMessage }}</p>

            <AppButton variant="secondary" @click="loadDraft"> Try again </AppButton>
        </div>

        <div v-else-if="state === 'ready' && project && branch">
            <section class="create-revision__context" aria-labelledby="revision-context-heading">
                <h3 id="revision-context-heading">Revision context</h3>

                <dl>
                    <div>
                        <dt>Project</dt>
                        <dd>{{ project.name }}</dd>
                    </div>

                    <div>
                        <dt>Target branch</dt>
                        <dd>{{ branch.name }}</dd>
                    </div>

                    <div>
                        <dt>Current branch head</dt>
                        <dd v-if="branch.headRevisionId === null">No revisions yet</dd>
                        <dd v-else>
                            <code>{{ branch.headRevisionId }}</code>
                        </dd>
                    </div>
                </dl>
            </section>

            <section class="create-revision__message" aria-labelledby="revision-message-heading">
                <h3 id="revision-message-heading">Revision message</h3>

                <AppTextArea
                    id="revision-message"
                    :model-value="revisionMessage"
                    label="Revision message *"
                    name="message"
                    placeholder="Describe what changed in this revision..."
                    :required="true"
                    :error="messageError"
                    @update:model-value="updateRevisionMessage"
                />
            </section>

            <section class="create-revision__files" aria-labelledby="revision-files-heading">
                <div class="create-revision__files-header">
                    <h3 id="revision-files-heading">Files in this revision</h3>

                    <span class="create-revision__count" aria-live="polite">
                        {{ selectedFileIds.size }} of {{ files.length }} selected
                    </span>
                </div>

                <p class="create-revision__help">
                    Only selected files will belong to this revision. Files uploaded to the project
                    are not included automatically.
                </p>

                <p v-if="files.length === 0" class="create-revision__empty">
                    No source files have been uploaded to this project.
                </p>

                <ul v-else class="create-revision__file-list" aria-label="Revision files">
                    <li v-for="file in files" :key="file.id" class="create-revision__file-item">
                        <label class="create-revision__file-label">
                            <input
                                type="checkbox"
                                :value="file.id"
                                :checked="selectedFileIds.has(file.id)"
                                @change="toggleFile(file.id, $event)"
                            />
                            <span>{{ file.originalFilename }}</span>
                        </label>
                    </li>
                </ul>
            </section>

            <p class="create-revision__notice">This is a draft. No revision has been created.</p>

            <div class="create-revision__actions">
                <p>Revision submission is not yet enabled.</p>

                <AppButton disabled aria-label="Create revision unavailable">
                    Create revision
                </AppButton>
            </div>
        </div>
    </section>
</template>

<style scoped>
    .create-revision {
        box-sizing: border-box;
        max-width: 62rem;
        margin: 0 auto;
        padding: var(--space-6) var(--space-4);
    }

    .create-revision__back {
        display: inline-flex;
        padding: var(--space-2) 0;
        color: var(--color-action);
        font-weight: var(--font-weight-medium);
        text-decoration: none;
    }

    .create-revision__back:hover {
        text-decoration: underline;
    }

    .create-revision__back:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
        border-radius: var(--radius-sm);
    }

    .create-revision__header {
        margin-top: var(--space-4);
    }

    .create-revision__header h2 {
        margin: 0;
    }

    .create-revision__header p,
    .create-revision__help,
    .create-revision__count,
    .create-revision__empty,
    .create-revision__notice {
        color: var(--color-text-muted);
    }

    .create-revision__feedback,
    .create-revision__context,
    .create-revision__message,
    .create-revision__files {
        margin-top: var(--space-5);
        padding: var(--space-5);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        background: var(--color-surface);
    }

    .create-revision__feedback p {
        color: var(--color-danger);
    }

    .create-revision__context h3,
    .create-revision__message h3,
    .create-revision__files h3 {
        margin: 0;
        font-size: var(--font-size-lg);
    }

    .create-revision__message h3 {
        margin-bottom: var(--space-4);
    }

    .create-revision__context dl {
        display: grid;
        gap: var(--space-3);
        margin: var(--space-4) 0 0;
    }

    .create-revision__context dl > div {
        display: grid;
        grid-template-columns: minmax(0, 10rem) minmax(0, 1fr);
        gap: var(--space-3);
    }

    .create-revision__context dt {
        color: var(--color-text-muted);
    }

    .create-revision__context dd {
        min-width: 0;
        margin: 0;
        font-weight: var(--font-weight-medium);
        overflow-wrap: anywhere;
    }

    .create-revision__context code {
        font-family: var(--font-family-mono);
    }

    .create-revision__files-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        flex-wrap: wrap;
        gap: var(--space-3);
    }

    .create-revision__count {
        font-size: var(--font-size-sm);
    }

    .create-revision__help {
        margin: var(--space-3) 0;
        font-size: var(--font-size-sm);
    }

    .create-revision__file-list {
        margin: var(--space-4) 0 0;
        padding: 0;
        list-style: none;
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        background: var(--color-page);
    }

    .create-revision__file-item + .create-revision__file-item {
        border-top: 1px solid var(--color-border);
    }

    .create-revision__file-label {
        display: flex;
        align-items: center;
        gap: var(--space-3);
        padding: var(--space-3);
        cursor: pointer;
        overflow-wrap: anywhere;
    }

    .create-revision__file-label:hover {
        background: var(--color-surface);
    }

    .create-revision__file-label input {
        flex-shrink: 0;
        accent-color: var(--color-action);
    }

    .create-revision__file-label input:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
    }

    .create-revision__notice {
        margin-top: var(--space-5);
        font-size: var(--font-size-sm);
    }

    .create-revision__actions {
        display: flex;
        align-items: center;
        justify-content: flex-end;
        flex-wrap: wrap;
        gap: var(--space-3);
        margin-top: var(--space-5);
    }

    .create-revision__actions p {
        margin: 0;
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }

    @media (max-width: 40rem) {
        .create-revision__context dl > div {
            grid-template-columns: minmax(0, 1fr);
            gap: var(--space-1);
        }
    }
</style>
