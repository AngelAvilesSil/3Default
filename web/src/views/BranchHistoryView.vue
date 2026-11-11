<script setup lang="ts">
    import { computed, onUnmounted, ref, shallowRef, watch } from 'vue'
    import { useRoute, useRouter } from 'vue-router'

    import {
        getProject,
        listProjectBranches,
        ProjectRequestError,
        type Project,
        type ProjectBranch,
    } from '../api/projects'
    import { listProjectBranchHistory, RevisionRequestError } from '../api/revisions'
    import RevisionGraphRenderer from '../components/history/RevisionGraph.vue'
    import AppButton from '../components/ui/AppButton.vue'
    import RevisionId from '../components/ui/RevisionId.vue'
    import { loadProjectHistoryGraph } from '../history/projectHistoryProvider'
    import { buildRevisionGraph, type RevisionGraph } from '../history/revisionGraph'
    import { useSessionStore } from '../stores/session'

    const props = defineProps<{
        projectId: string
        branchId: string
    }>()

    const route = useRoute()
    const router = useRouter()
    const session = useSessionStore()

    type LoadState = 'loading' | 'ready' | 'unavailable' | 'error'
    type HistoryScope = 'branch' | 'all'

    const state = ref<LoadState>('loading')
    const project = ref<Project | null>(null)
    const branch = ref<ProjectBranch | null>(null)
    const branches = ref<ProjectBranch[]>([])
    const branchGraph = shallowRef<RevisionGraph | null>(null)
    const allGraph = shallowRef<RevisionGraph | null>(null)

    const scope = ref<HistoryScope>('branch')
    const selectedRevisionId = ref<string | null>(null)

    const allLoading = ref(false)
    const allErrorMessage = ref('')
    const unavailableMessage = ref('')
    const errorMessage = ref('')

    let activeController: AbortController | null = null
    let allController: AbortController | null = null

    const activeGraph = computed(() => (scope.value === 'all' ? allGraph.value : branchGraph.value))

    const selectedRevision = computed(
        () =>
            activeGraph.value?.nodes.find((node) => node.revision.id === selectedRevisionId.value)
                ?.revision ?? null,
    )

    function isCurrent(controller: AbortController, projectId: string, branchId: string): boolean {
        return (
            !controller.signal.aborted &&
            activeController === controller &&
            props.projectId === projectId &&
            props.branchId === branchId
        )
    }

    function isCurrentAll(
        controller: AbortController,
        projectId: string,
        branchId: string,
    ): boolean {
        return (
            !controller.signal.aborted &&
            allController === controller &&
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
        allController?.abort()
        allController = null

        const controller = new AbortController()
        activeController = controller

        const projectId = props.projectId
        const branchId = props.branchId

        state.value = 'loading'
        project.value = null
        branch.value = null
        branches.value = []
        branchGraph.value = null
        allGraph.value = null
        scope.value = 'branch'
        selectedRevisionId.value = null
        allLoading.value = false
        allErrorMessage.value = ''
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

            // Branch enumeration and history are independent reads.
            // Do not present an obviously inconsistent observed head.
            const head = selectedBranch.headRevisionId
            const inconsistent =
                (head === null && loadedHistory.length !== 0) ||
                (head !== null && !loadedHistory.some((revision) => revision.id === head))

            if (inconsistent) {
                state.value = 'error'
                errorMessage.value = 'Branch history changed while loading. Please try again.'
                return
            }

            const graph = buildRevisionGraph({
                projectId,
                revisions: loadedHistory,
                branches: [selectedBranch],
                coverage: {
                    source: 'branch-head-reachable',
                    status: 'complete',
                    failedBranchIds: [],
                },
            })

            if (!isCurrent(controller, projectId, branchId)) {
                return
            }

            project.value = loadedProject
            branch.value = selectedBranch
            branches.value = loadedBranches
            branchGraph.value = graph
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

    async function loadAllBranches(): Promise<void> {
        allController?.abort()

        const controller = new AbortController()
        allController = controller

        const projectId = props.projectId
        const branchId = props.branchId
        const observedBranches = [...branches.value]

        allLoading.value = true
        allErrorMessage.value = ''

        try {
            const graph = await loadProjectHistoryGraph(
                projectId,
                observedBranches,
                controller.signal,
            )

            if (!isCurrentAll(controller, projectId, branchId)) {
                return
            }

            // No visible revisions with incomplete coverage means that
            // history loading failed, not that the project has no history.
            // Keep the graph uncached so the user can explicitly retry.
            if (graph.nodes.length === 0 && graph.coverage.status === 'partial') {
                allGraph.value = null
                allErrorMessage.value =
                    'Unable to load any branch revision history. Please try again.'
                return
            }

            allGraph.value = graph
        } catch (error) {
            if (!isCurrentAll(controller, projectId, branchId)) {
                return
            }

            if (error instanceof RevisionRequestError && error.status === 401) {
                await redirectToLogin()
                return
            }

            allErrorMessage.value = 'Unable to load combined branch history. Please try again.'
        } finally {
            if (allController === controller) {
                allController = null
                allLoading.value = false
            }
        }
    }

    function showAllBranches(): void {
        if (scope.value === 'all' || branches.value.length < 2) {
            return
        }

        scope.value = 'all'
        selectedRevisionId.value = null

        if (allGraph.value === null) {
            void loadAllBranches()
        }
    }

    function showSelectedBranch(): void {
        allController?.abort()
        allController = null
        allLoading.value = false
        allErrorMessage.value = ''
        scope.value = 'branch'

        const selected = selectedRevisionId.value

        if (
            selected !== null &&
            !branchGraph.value?.nodes.some((node) => node.revision.id === selected)
        ) {
            selectedRevisionId.value = null
        }
    }

    function selectRevision(revisionId: string): void {
        if (activeGraph.value?.nodes.some((node) => node.revision.id === revisionId)) {
            selectedRevisionId.value = revisionId
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
        allController?.abort()
        activeController = null
        allController = null
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
                    <RevisionId :id="branch.headRevisionId" label="branch head revision" />
                </span>
            </p>

            <div
                v-if="branches.length > 1"
                class="branch-history__scope-buttons"
                role="group"
                aria-label="History scope"
            >
                <AppButton
                    variant="secondary"
                    aria-label="Show selected branch"
                    :aria-pressed="scope === 'branch'"
                    @click="showSelectedBranch"
                >
                    Selected branch
                </AppButton>

                <AppButton
                    variant="secondary"
                    aria-label="Show all branches"
                    :aria-pressed="scope === 'all'"
                    :disabled="allLoading"
                    @click="showAllBranches"
                >
                    All branches
                </AppButton>
            </div>

            <p class="branch-history__note">
                Revisions follow their actual ancestry, not necessarily a single development path.
                Selecting a revision does not change the active project branch.
            </p>

            <p v-if="scope === 'all' && allLoading" role="status">
                Loading histories from all branches...
            </p>

            <div v-else-if="scope === 'all' && allErrorMessage" class="branch-history__feedback">
                <p role="alert">{{ allErrorMessage }}</p>
                <AppButton variant="secondary" @click="loadAllBranches()">Try again</AppButton>
            </div>

            <template v-else>
                <p
                    v-if="scope === 'branch' && branchGraph?.nodes.length === 0"
                    class="branch-history__empty"
                >
                    This branch has no revisions yet.
                </p>

                <RevisionGraphRenderer
                    v-if="activeGraph && activeGraph.nodes.length > 0"
                    :graph="activeGraph"
                    :selected-revision-id="selectedRevisionId"
                    @select-revision="selectRevision"
                />

                <p
                    v-else-if="scope === 'all' && activeGraph && activeGraph.nodes.length === 0"
                    class="branch-history__empty"
                >
                    No revisions were found in the loaded branch histories.
                </p>
            </template>

            <section
                v-if="selectedRevision"
                class="branch-history__selection"
                aria-labelledby="branch-history-selection-heading"
            >
                <h3 id="branch-history-selection-heading">Selected revision</h3>

                <p class="branch-history__selection-message">
                    {{ selectedRevision.message }}
                </p>

                <dl class="branch-history__selection-metadata">
                    <div>
                        <dt>Revision</dt>
                        <dd>
                            <RevisionId :id="selectedRevision.id" />
                        </dd>
                    </div>

                    <div>
                        <dt>Parent</dt>
                        <dd>
                            <RevisionId
                                v-if="selectedRevision.parentRevisionId"
                                :id="selectedRevision.parentRevisionId"
                                label="parent revision"
                            />
                            <span v-else>None (root revision)</span>
                        </dd>
                    </div>

                    <div v-if="selectedRevision.mergeParentRevisionId">
                        <dt>Merge parent</dt>
                        <dd>
                            <RevisionId
                                :id="selectedRevision.mergeParentRevisionId"
                                label="merge parent revision"
                            />
                        </dd>
                    </div>
                </dl>

                <RouterLink
                    class="branch-history__selected-inspect"
                    :to="{
                        name: 'project-revision-detail',
                        params: {
                            projectId: props.projectId,
                            revisionId: selectedRevision.id,
                        },
                        query: { fromBranchId: props.branchId },
                    }"
                >
                    Inspect revision
                </RouterLink>
            </section>
        </div>
    </section>
</template>

<style scoped>
    .branch-history {
        box-sizing: border-box;
        width: 100%;
        min-width: 0;
        padding: var(--space-6) var(--space-4);
    }

    .branch-history__back,
    .branch-history__selected-inspect {
        color: var(--color-action);
    }

    .branch-history__back:focus-visible,
    .branch-history__selected-inspect:focus-visible {
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
    .branch-history__feedback,
    .branch-history__selection {
        padding: var(--space-4);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        background: var(--color-surface);
    }

    .branch-history__context {
        overflow-wrap: anywhere;
    }

    .branch-history__feedback p {
        margin-top: 0;
        color: var(--color-danger);
    }

    .branch-history__scope-buttons {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-2);
        margin: var(--space-4) 0;
    }

    .branch-history__scope-buttons button[aria-pressed='true'] {
        border-color: var(--color-action);
    }

    .branch-history__selection {
        margin-top: var(--space-4);
    }

    .branch-history__selection h3 {
        margin-top: 0;
        font-size: var(--font-size-base);
    }

    .branch-history__selection-message {
        overflow-wrap: anywhere;
    }

    .branch-history__selection-metadata {
        display: grid;
        gap: var(--space-2);
    }

    .branch-history__selection-metadata > div {
        display: grid;
        grid-template-columns: minmax(0, 7rem) minmax(0, 1fr);
        gap: var(--space-2);
    }

    .branch-history__selection-metadata dt {
        color: var(--color-text-muted);
    }

    .branch-history__selection-metadata dd {
        min-width: 0;
        margin: 0;
        overflow-wrap: anywhere;
    }

    .branch-history__selected-inspect {
        display: inline-block;
        margin-top: var(--space-3);
    }

    @media (max-width: 40rem) {
        .branch-history__selection-metadata > div {
            grid-template-columns: minmax(0, 1fr);
        }
    }
</style>
