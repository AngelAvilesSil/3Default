<script setup lang="ts">
    import { computed, onUnmounted, ref, watch } from 'vue'
    import { useRoute, useRouter } from 'vue-router'

    import {
        createProjectBranch,
        getProject,
        ProjectRequestError,
        type Project,
    } from '../api/projects'
    import {
        getProjectRevision,
        RevisionRequestError,
        type ProjectRevision,
    } from '../api/revisions'
    import AppButton from '../components/ui/AppButton.vue'
    import AppTextField from '../components/ui/AppTextField.vue'
    import RevisionId from '../components/ui/RevisionId.vue'
    import { useSessionStore } from '../stores/session'

    const props = defineProps<{
        projectId: string
        revisionId: string
    }>()

    const route = useRoute()
    const router = useRouter()
    const session = useSessionStore()

    type LoadState = 'loading' | 'ready' | 'error' | 'unavailable'

    const state = ref<LoadState>('loading')
    const project = ref<Project | null>(null)
    const revision = ref<ProjectRevision | null>(null)

    const branchName = ref('')
    const nameError = ref('')
    const errorMessage = ref('')
    const unavailableMessage = ref('')
    const submissionError = ref('')
    const submitting = ref(false)
    const submissionBlocked = ref(false)

    let activeLoadController: AbortController | null = null
    let activeSubmitController: AbortController | null = null

    // This query parameter is navigation context, never branch ownership data.
    const sourceBranchId = computed(() => {
        const value = route.query.fromBranchId
        return typeof value === 'string' && value.trim() ? value : null
    })

    const revisionDestination = computed(() => ({
        name: 'project-revision-detail',
        params: {
            projectId: props.projectId,
            revisionId: props.revisionId,
        },
        query: sourceBranchId.value === null ? {} : { fromBranchId: sourceBranchId.value },
    }))

    function isCurrentLoad(
        controller: AbortController,
        projectId: string,
        revisionId: string,
    ): boolean {
        return (
            !controller.signal.aborted &&
            activeLoadController === controller &&
            props.projectId === projectId &&
            props.revisionId === revisionId
        )
    }

    async function redirectToLogin(): Promise<void> {
        const returnPath = route.fullPath
        session.invalidate()

        await router.replace({
            name: 'login',
            query: { redirect: returnPath },
        })
    }

    async function loadContext(): Promise<void> {
        activeLoadController?.abort()
        activeSubmitController?.abort()
        activeSubmitController = null

        const controller = new AbortController()
        activeLoadController = controller

        const projectId = props.projectId
        const revisionId = props.revisionId

        state.value = 'loading'
        project.value = null
        revision.value = null
        branchName.value = ''
        nameError.value = ''
        errorMessage.value = ''
        unavailableMessage.value = ''
        submissionError.value = ''
        submitting.value = false
        submissionBlocked.value = false

        if (!projectId.trim() || !revisionId.trim()) {
            state.value = 'unavailable'
            unavailableMessage.value = 'This starting revision link is invalid.'
            activeLoadController = null
            return
        }

        try {
            const loadedProject = await getProject(projectId, controller.signal)

            if (!isCurrentLoad(controller, projectId, revisionId)) {
                return
            }

            if (loadedProject.id !== projectId) {
                throw new Error('Unexpected project identity')
            }

            const loadedRevision = await getProjectRevision(
                projectId,
                revisionId,
                controller.signal,
            )

            if (!isCurrentLoad(controller, projectId, revisionId)) {
                return
            }

            if (loadedRevision.projectId !== projectId || loadedRevision.id !== revisionId) {
                throw new Error('Unexpected starting revision identity')
            }

            project.value = loadedProject
            revision.value = loadedRevision
            state.value = 'ready'
        } catch (error) {
            if (!isCurrentLoad(controller, projectId, revisionId)) {
                return
            }

            if (
                (error instanceof ProjectRequestError || error instanceof RevisionRequestError) &&
                error.status === 401
            ) {
                await redirectToLogin()
                return
            }

            if (
                (error instanceof ProjectRequestError || error instanceof RevisionRequestError) &&
                [400, 403, 404].includes(error.status)
            ) {
                state.value = 'unavailable'
                unavailableMessage.value =
                    'This project or starting revision was not found or is not accessible.'
                return
            }

            state.value = 'error'
            errorMessage.value = 'Unable to load the starting revision. Please try again.'
        } finally {
            if (activeLoadController === controller) {
                activeLoadController = null
            }
        }
    }

    function updateBranchName(value: string): void {
        if (submitting.value || submissionBlocked.value) {
            return
        }

        branchName.value = value
        nameError.value = ''
        submissionError.value = ''
    }

    async function handleSubmit(): Promise<void> {
        if (
            state.value !== 'ready' ||
            project.value === null ||
            revision.value === null ||
            project.value.id !== props.projectId ||
            revision.value.id !== props.revisionId ||
            revision.value.projectId !== props.projectId ||
            submitting.value ||
            submissionBlocked.value
        ) {
            return
        }

        const name = branchName.value.trim()
        nameError.value = name ? '' : 'Branch name is required.'
        submissionError.value = ''

        if (!name) {
            return
        }

        const projectId = props.projectId
        const revisionId = props.revisionId

        const controller = new AbortController()
        activeSubmitController = controller
        submitting.value = true

        let confirmedCreated = false

        try {
            const created = await createProjectBranch(
                projectId,
                {
                    name,
                    headRevisionId: revisionId,
                },
                controller.signal,
            )

            if (
                controller.signal.aborted ||
                activeSubmitController !== controller ||
                props.projectId !== projectId ||
                props.revisionId !== revisionId
            ) {
                return
            }

            if (created.name !== name) {
                submissionBlocked.value = true
                submissionError.value =
                    'The branch response was inconsistent. Check your project before trying again.'
                return
            }

            confirmedCreated = true
            submissionBlocked.value = true
        } catch (error) {
            if (
                controller.signal.aborted ||
                activeSubmitController !== controller ||
                props.projectId !== projectId ||
                props.revisionId !== revisionId
            ) {
                return
            }

            if (error instanceof ProjectRequestError && error.status === 401) {
                submissionBlocked.value = true
                await redirectToLogin()
                return
            }

            if (error instanceof ProjectRequestError && error.status === 409) {
                nameError.value = 'A branch with this name already exists.'
                return
            }

            if (error instanceof ProjectRequestError && error.status === 400) {
                submissionError.value = 'The branch name was not accepted. Review it and try again.'
                return
            }

            // Confirmed unavailable/forbidden and uncertain POST outcomes
            // must not encourage a second submission from this draft.
            submissionBlocked.value = true

            if (error instanceof ProjectRequestError && [403, 404].includes(error.status)) {
                submissionError.value =
                    'The project or starting revision is no longer available. Review the project before trying again.'
                return
            }

            submissionError.value =
                'Branch creation could not be confirmed. Check the project branch list before trying again.'
        } finally {
            if (activeSubmitController === controller) {
                activeSubmitController = null
                submitting.value = false
            }
        }

        if (confirmedCreated && props.projectId === projectId) {
            await router.push({
                name: 'project-overview',
                params: { projectId },
            })
        }
    }

    function cancel(): void {
        if (submitting.value) {
            return
        }

        if (submissionBlocked.value) {
            void router.push({
                name: 'project-overview',
                params: { projectId: props.projectId },
            })
            return
        }

        void router.push(revisionDestination.value)
    }

    watch(
        [() => props.projectId, () => props.revisionId],
        () => {
            void loadContext()
        },
        { immediate: true },
    )

    onUnmounted(() => {
        activeLoadController?.abort()
        activeLoadController = null
        activeSubmitController?.abort()
        activeSubmitController = null
    })
</script>

<template>
    <section class="create-branch" aria-labelledby="create-branch-heading">
        <RouterLink v-if="!submitting" class="create-branch__back" :to="revisionDestination">
            Back to revision
        </RouterLink>

        <header class="create-branch__header">
            <h2 id="create-branch-heading">Create branch</h2>
            <p>Start a new design direction from an exact, existing revision.</p>
        </header>

        <p v-if="state === 'loading'" role="status">Loading starting revision...</p>

        <p v-else-if="state === 'unavailable'" role="alert">
            {{ unavailableMessage }}
        </p>

        <div v-else-if="state === 'error'" class="create-branch__feedback">
            <p role="alert">{{ errorMessage }}</p>
            <AppButton variant="secondary" @click="loadContext">Try again</AppButton>
        </div>

        <form
            v-else-if="state === 'ready' && project && revision"
            class="create-branch__form"
            novalidate
            @submit.prevent="handleSubmit"
        >
            <section class="create-branch__panel" aria-labelledby="branch-context-heading">
                <h3 id="branch-context-heading">Starting revision</h3>

                <dl class="create-branch__metadata">
                    <div>
                        <dt>Project</dt>
                        <dd>{{ project.name }}</dd>
                    </div>
                    <div>
                        <dt>Revision</dt>
                        <dd>
                            <strong>{{ revision.message }}</strong>
                            <RevisionId :id="revision.id" label="starting revision" />
                        </dd>
                    </div>
                    <div>
                        <dt>Created</dt>
                        <dd>
                            <time :datetime="revision.createdAt">
                                {{ new Date(revision.createdAt).toLocaleString() }}
                            </time>
                        </dd>
                    </div>
                </dl>
            </section>

            <section class="create-branch__panel" aria-labelledby="branch-name-heading">
                <h3 id="branch-name-heading">New branch</h3>

                <AppTextField
                    id="branch-name"
                    :model-value="branchName"
                    label="Branch name *"
                    name="name"
                    autocomplete="off"
                    placeholder="e.g. lightweight-design"
                    :required="true"
                    :disabled="submitting || submissionBlocked"
                    :error="nameError"
                    @update:model-value="updateBranchName"
                />

                <p class="create-branch__note">
                    This branch points to the selected revision. It does not duplicate the
                    revision's source files or modify existing branches.
                </p>
            </section>

            <p v-if="submissionError" class="create-branch__error" role="alert">
                {{ submissionError }}
            </p>

            <div class="create-branch__actions">
                <AppButton variant="secondary" :disabled="submitting" @click="cancel">
                    {{ submissionBlocked ? 'Review project' : 'Cancel' }}
                </AppButton>

                <AppButton type="submit" :loading="submitting" :disabled="submissionBlocked">
                    {{ submitting ? 'Creating branch…' : 'Create branch' }}
                </AppButton>
            </div>
        </form>
    </section>
</template>

<style scoped>
    .create-branch {
        box-sizing: border-box;
        width: 100%;
        max-width: 62rem;
        margin-inline: auto;
        padding: var(--space-6) var(--space-4);
    }

    .create-branch__back {
        color: var(--color-action);
        font-weight: var(--font-weight-medium);
    }

    .create-branch__back:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
    }

    .create-branch__header {
        margin-top: var(--space-4);
    }

    .create-branch__header h2 {
        margin: 0;
    }

    .create-branch__header p,
    .create-branch__note {
        color: var(--color-text-muted);
    }

    .create-branch__form {
        margin-top: var(--space-5);
    }

    .create-branch__panel,
    .create-branch__feedback {
        margin-top: var(--space-4);
        padding: var(--space-4);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        background: var(--color-surface);
    }

    .create-branch__panel h3 {
        margin: 0 0 var(--space-4);
    }

    .create-branch__metadata {
        display: grid;
        gap: var(--space-3);
        margin: 0;
    }

    .create-branch__metadata > div {
        display: grid;
        grid-template-columns: minmax(0, 8rem) minmax(0, 1fr);
        gap: var(--space-2);
    }

    .create-branch__metadata dt {
        color: var(--color-text-muted);
    }

    .create-branch__metadata dd {
        display: flex;
        min-width: 0;
        flex-wrap: wrap;
        align-items: center;
        gap: var(--space-2);
        margin: 0;
        overflow-wrap: anywhere;
    }

    .create-branch__error,
    .create-branch__feedback [role='alert'] {
        color: var(--color-danger);
    }

    .create-branch__actions {
        display: flex;
        justify-content: flex-end;
        flex-wrap: wrap;
        gap: var(--space-3);
        margin-top: var(--space-5);
    }

    @media (max-width: 40rem) {
        .create-branch__metadata > div {
            grid-template-columns: minmax(0, 1fr);
        }
    }
</style>
