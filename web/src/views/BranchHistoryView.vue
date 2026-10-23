<script setup lang="ts">
    import { onUnmounted, ref, watch } from 'vue'
    import { useRoute, useRouter } from 'vue-router'

    import {
        getProject,
        listProjectBranches,
        ProjectRequestError,
        type Project,
        type ProjectBranch,
    } from '../api/projects'
    import {
        listProjectBranchHistory,
        RevisionRequestError,
        type ProjectRevision,
    } from '../api/revisions'
    import AppButton from '../components/ui/AppButton.vue'
    import { useSessionStore } from '../stores/session'

    const props = defineProps<{
        projectId: string
        branchId: string
    }>()

    const route = useRoute()
    const router = useRouter()
    const session = useSessionStore()

    type LoadState = 'loading' | 'ready' | 'unavailable' | 'error'

    const state = ref<LoadState>('loading')
    const project = ref<Project | null>(null)
    const branch = ref<ProjectBranch | null>(null)
    const revisions = ref<ProjectRevision[]>([])
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

    async function redirectToLogin(): Promise<void> {
        const returnPath = route.fullPath
        session.invalidate()

        await router.replace({
            name: 'login',
            query: { redirect: returnPath },
        })
    }

    async function loadHistory(): Promise<void> {
        activeController?.abort()

        const controller = new AbortController()
        activeController = controller

        const projectId = props.projectId
        const branchId = props.branchId

        state.value = 'loading'
        project.value = null
        branch.value = null
        revisions.value = []
        unavailableMessage.value = ''
        errorMessage.value = ''

        if (!projectId.trim() || !branchId.trim()) {
            state.value = 'unavailable'
            unavailableMessage.value = 'This branch history link is invalid.'
            activeController = null
            return
        }

        try {
            const loadedProject = await getProject(projectId, controller.signal)

            if (!isCurrent(controller, projectId, branchId)) {
                return
            }

            if (loadedProject.id !== projectId) {
                throw new Error('Unexpected project identity')
            }

            const loadedBranches = await listProjectBranches(projectId, controller.signal)

            if (!isCurrent(controller, projectId, branchId)) {
                return
            }

            if (loadedBranches.some((item) => item.projectId !== projectId)) {
                throw new Error('Unexpected branch project identity')
            }

            const selectedBranch = loadedBranches.find((item) => item.id === branchId)

            if (!selectedBranch) {
                state.value = 'unavailable'
                unavailableMessage.value = 'This branch was not found or is not accessible.'
                return
            }

            const loadedHistory = await listProjectBranchHistory(
                projectId,
                branchId,
                controller.signal,
            )

            if (!isCurrent(controller, projectId, branchId)) {
                return
            }

            // These independent reads cannot guarantee an atomic snapshot.
            // Detect obvious mismatches, without treating a previously
            // observed head as a guaranteed live value.
            const head = selectedBranch.headRevisionId
            const inconsistent =
                (head === null && loadedHistory.length !== 0) ||
                (head !== null && !loadedHistory.some((item) => item.id === head))

            if (inconsistent) {
                state.value = 'error'
                errorMessage.value = 'Branch history changed while loading. Please try again.'
                return
            }

            project.value = loadedProject
            branch.value = selectedBranch
            revisions.value = loadedHistory
            state.value = 'ready'
        } catch (error) {
            if (!isCurrent(controller, projectId, branchId)) {
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
                (error.status === 400 || error.status === 404)
            ) {
                state.value = 'unavailable'
                unavailableMessage.value = 'This branch history was not found or is not accessible.'
                return
            }

            state.value = 'error'
            errorMessage.value = 'Unable to load branch history. Please try again.'
        } finally {
            if (activeController === controller) {
                activeController = null
            }
        }
    }

    watch(
        () => [props.projectId, props.branchId],
        () => {
            void loadHistory()
        },
        { immediate: true },
    )

    onUnmounted(() => {
        activeController?.abort()
        activeController = null
    })
</script>

<template>
    <section class="branch-history" aria-labelledby="branch-history-heading">
        <RouterLink
            class="branch-history__back"
            :to="{ name: 'project-overview', params: { projectId: props.projectId } }"
        >
            Back to project
        </RouterLink>

        <header class="branch-history__header">
            <h2 id="branch-history-heading">Revision history</h2>
            <p v-if="project && branch">{{ project.name }} / {{ branch.name }}</p>
        </header>

        <p v-if="state === 'loading'" role="status">Loading revision history...</p>

        <p v-else-if="state === 'unavailable'" role="alert">
            {{ unavailableMessage }}
        </p>

        <div v-else-if="state === 'error'" class="branch-history__feedback">
            <p role="alert">{{ errorMessage }}</p>
            <AppButton variant="secondary" @click="loadHistory()">Try again</AppButton>
        </div>

        <div v-else-if="state === 'ready' && branch">
            <p class="branch-history__context">
                Branch: <strong>{{ branch.name }}</strong>
                <span v-if="branch.headRevisionId === null"> — No revisions yet</span>
                <span v-else>
                    — Head when loaded:
                    <code>{{ branch.headRevisionId }}</code>
                </span>
            </p>

            <p class="branch-history__note">
                Revisions are displayed newest first. Parent and merge-parent references define
                their actual ancestry; display order is not necessarily a single development path.
            </p>

            <p v-if="revisions.length === 0" class="branch-history__empty">
                This branch has no revisions yet.
            </p>

            <ul v-else class="branch-history__list">
                <li v-for="revision in revisions" :key="revision.id" class="branch-history__item">
                    <div class="branch-history__item-heading">
                        <strong>{{ revision.message }}</strong>
                        <span
                            v-if="revision.id === branch.headRevisionId"
                            class="branch-history__head"
                        >
                            Head at load
                        </span>
                    </div>

                    <dl class="branch-history__metadata">
                        <div>
                            <dt>Revision</dt>
                            <dd>
                                <code>{{ revision.id }}</code>
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
                        <div>
                            <dt>Parent</dt>
                            <dd>
                                <code v-if="revision.parentRevisionId">
                                    {{ revision.parentRevisionId }}
                                </code>
                                <span v-else>None (root revision)</span>
                            </dd>
                        </div>
                        <div v-if="revision.mergeParentRevisionId">
                            <dt>Merge parent</dt>
                            <dd>
                                <code>{{ revision.mergeParentRevisionId }}</code>
                            </dd>
                        </div>
                    </dl>

                    <RouterLink
                        class="branch-history__inspect"
                        :to="{
                            name: 'project-revision-detail',
                            params: {
                                projectId: props.projectId,
                                revisionId: revision.id,
                            },
                            query: { fromBranchId: props.branchId },
                        }"
                    >
                        Inspect revision
                    </RouterLink>
                </li>
            </ul>
        </div>
    </section>
</template>

<style scoped>
    .branch-history {
        box-sizing: border-box;
        width: 100%;
        padding: var(--space-6) var(--space-4);
    }

    .branch-history__back {
        color: var(--color-action);
    }

    .branch-history__back:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
    }

    .branch-history__header {
        margin-top: var(--space-4);
    }

    .branch-history__header h2 {
        margin: 0;
    }

    .branch-history__header p,
    .branch-history__note,
    .branch-history__empty {
        color: var(--color-text-muted);
    }

    .branch-history__context,
    .branch-history__feedback {
        padding: var(--space-4);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        background: var(--color-surface);
    }

    .branch-history__context code,
    .branch-history__metadata code {
        font-family: var(--font-family-mono);
        overflow-wrap: anywhere;
    }

    .branch-history__list {
        padding: 0;
        margin: var(--space-4) 0 0;
        list-style: none;
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
    }

    .branch-history__item {
        padding: var(--space-4);
        overflow-wrap: anywhere;
    }

    .branch-history__item + .branch-history__item {
        border-top: 1px solid var(--color-border);
    }

    .branch-history__item-heading {
        display: flex;
        align-items: center;
        flex-wrap: wrap;
        gap: var(--space-2);
    }

    .branch-history__head {
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }

    .branch-history__inspect {
        display: inline-block;
        margin-top: var(--space-3);
        color: var(--color-action);
    }

    .branch-history__inspect:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
    }

    .branch-history__metadata {
        display: grid;
        gap: var(--space-2);
        margin: var(--space-3) 0 0;
    }

    .branch-history__metadata > div {
        display: grid;
        grid-template-columns: minmax(0, 7rem) minmax(0, 1fr);
        gap: var(--space-2);
    }

    .branch-history__metadata dt {
        color: var(--color-text-muted);
    }

    .branch-history__metadata dd {
        min-width: 0;
        margin: 0;
        overflow-wrap: anywhere;
    }

    @media (max-width: 40rem) {
        .branch-history__metadata > div {
            grid-template-columns: minmax(0, 1fr);
        }
    }
</style>
