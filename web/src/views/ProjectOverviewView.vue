<script setup lang="ts">
    import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
    import { useRoute, useRouter } from 'vue-router'

    import {
        getProject,
        listProjectBranches,
        ProjectRequestError,
        type Project,
        type ProjectBranch,
    } from '../api/projects'
    import ProjectFileWorkspace from '../components/ProjectFileWorkspace.vue'
    import AppButton from '../components/ui/AppButton.vue'
    import RevisionId from '../components/ui/RevisionId.vue'
    import { useSessionStore } from '../stores/session'

    const route = useRoute()
    const router = useRouter()
    const session = useSessionStore()

    const project = ref<Project | null>(null)
    const loading = ref(true)
    const unavailableMessage = ref('')
    const errorMessage = ref('')

    const branches = ref<ProjectBranch[]>([])
    const branchesLoading = ref(false)
    const branchErrorMessage = ref('')
    const branchPickerOpen = ref(false)
    const branchSearch = ref('')
    const branchPickerTrigger = ref<HTMLButtonElement | null>(null)
    const branchSearchInput = ref<HTMLInputElement | null>(null)

    const selectedBranch = computed<ProjectBranch | null>(() => {
        if (branches.value.length === 0) {
            return null
        }

        const requestedId = route.query.branchId

        if (requestedId !== undefined) {
            if (typeof requestedId !== 'string' || !requestedId) {
                return null
            }

            return branches.value.find((branch) => branch.id === requestedId) ?? null
        }

        return branches.value.find((branch) => branch.name === 'main') ?? branches.value[0] ?? null
    })

    const filteredBranches = computed(() => {
        const query = branchSearch.value.trim().toLowerCase()

        if (!query) {
            return branches.value
        }

        return branches.value.filter((branch) => branch.name.toLowerCase().includes(query))
    })

    let activeController: AbortController | null = null
    let activeBranchController: AbortController | null = null

    async function redirectToLogin(): Promise<void> {
        const returnPath = route.fullPath

        session.invalidate()

        await router.replace({
            name: 'login',
            query: { redirect: returnPath },
        })
    }

    function resetBranches(): void {
        activeBranchController?.abort()
        activeBranchController = null

        branches.value = []
        branchesLoading.value = false
        branchErrorMessage.value = ''
        branchPickerOpen.value = false
        branchSearch.value = ''
    }

    async function loadBranches(projectId: string): Promise<void> {
        activeBranchController?.abort()

        const controller = new AbortController()
        activeBranchController = controller

        branches.value = []
        branchesLoading.value = true
        branchErrorMessage.value = ''

        try {
            const result = await listProjectBranches(projectId, controller.signal)

            if (controller.signal.aborted || activeBranchController !== controller) {
                return
            }

            // Never render branch records belonging to another project.
            if (result.some((branch) => branch.projectId !== projectId)) {
                branchErrorMessage.value = 'Unable to load branches. Please try again.'
                return
            }

            branches.value = result
        } catch (error) {
            if (controller.signal.aborted || activeBranchController !== controller) {
                return
            }

            if (error instanceof ProjectRequestError && error.status === 401) {
                await redirectToLogin()
                return
            }

            branchErrorMessage.value = 'Unable to load branches. Please try again.'
        } finally {
            if (activeBranchController === controller) {
                activeBranchController = null

                if (!controller.signal.aborted) {
                    branchesLoading.value = false
                }
            }
        }
    }

    async function loadProject(): Promise<void> {
        activeController?.abort()
        resetBranches()

        const controller = new AbortController()
        activeController = controller

        const projectId = route.params.projectId

        project.value = null
        loading.value = true
        unavailableMessage.value = ''
        errorMessage.value = ''

        if (typeof projectId !== 'string' || !projectId) {
            unavailableMessage.value = 'This project link is invalid.'
            loading.value = false
            activeController = null
            return
        }

        try {
            const result = await getProject(projectId, controller.signal)

            if (controller.signal.aborted || activeController !== controller) {
                return
            }

            project.value = result

            // Branch failures must not discard successfully loaded metadata.
            void loadBranches(projectId)
        } catch (error) {
            if (controller.signal.aborted || activeController !== controller) {
                return
            }

            if (error instanceof ProjectRequestError) {
                if (error.status === 401) {
                    await redirectToLogin()
                    return
                }

                if (error.status === 400) {
                    unavailableMessage.value = 'This project link is invalid.'
                    return
                }

                if (error.status === 404) {
                    unavailableMessage.value = 'This project was not found or is not accessible.'
                    return
                }
            }

            errorMessage.value = 'Unable to load the project. Please try again.'
        } finally {
            if (activeController === controller) {
                activeController = null

                if (!controller.signal.aborted) {
                    loading.value = false
                }
            }
        }
    }

    function openFileUpload(): void {
        if (project.value === null) {
            return
        }

        void router.push({
            name: 'project-file-upload',
            params: { projectId: project.value.id },
        })
    }

    async function toggleBranchPicker(event: MouseEvent): Promise<void> {
        const trigger = event.currentTarget

        if (!(trigger instanceof HTMLButtonElement)) {
            return
        }

        branchPickerTrigger.value = trigger
        branchPickerOpen.value = !branchPickerOpen.value
        branchSearch.value = ''

        if (branchPickerOpen.value) {
            await nextTick()

            if (branchPickerOpen.value) {
                branchSearchInput.value?.focus()
            }
        }
    }

    function closeBranchPickerOnEscape(): void {
        branchPickerOpen.value = false
        branchSearch.value = ''
        branchPickerTrigger.value?.focus()
    }

    function chooseBranch(branch: ProjectBranch): void {
        if (
            project.value === null ||
            branch.projectId !== project.value.id ||
            !branches.value.some((candidate) => candidate.id === branch.id)
        ) {
            return
        }

        branchPickerOpen.value = false
        branchSearch.value = ''

        if (route.query.branchId === branch.id) {
            return
        }

        void router.push({
            name: 'project-overview',
            params: { projectId: project.value.id },
            query: { ...route.query, branchId: branch.id },
        })
    }

    function openBranchHistory(branch: ProjectBranch): void {
        if (
            project.value === null ||
            branch.projectId !== project.value.id ||
            !branches.value.some((candidate) => candidate.id === branch.id)
        ) {
            return
        }

        void router.push({
            name: 'project-branch-history',
            params: {
                projectId: project.value.id,
                branchId: branch.id,
            },
        })
    }

    function openCreateRevision(branch: ProjectBranch): void {
        if (
            project.value === null ||
            branch.projectId !== project.value.id ||
            !branches.value.some((candidate) => candidate.id === branch.id)
        ) {
            return
        }

        void router.push({
            name: 'create-project-revision',
            params: {
                projectId: project.value.id,
                branchId: branch.id,
            },
        })
    }

    function retryBranches(): void {
        if (project.value !== null) {
            void loadBranches(project.value.id)
        }
    }

    watch(
        () => route.params.projectId,
        () => {
            void loadProject()
        },
        { immediate: true },
    )

    watch(
        () => route.query.branchId,
        () => {
            branchPickerOpen.value = false
            branchSearch.value = ''
        },
    )

    onUnmounted(() => {
        activeController?.abort()
        activeController = null
        resetBranches()
    })
</script>

<template>
    <section class="project-overview" aria-labelledby="project-overview-heading">
        <header class="project-overview__header">
            <h2 id="project-overview-heading">
                {{ project?.name ?? 'Project overview' }}
            </h2>

            <p v-if="project?.description">
                {{ project.description }}
            </p>
        </header>

        <p v-if="loading" class="project-overview__feedback" role="status">Loading project...</p>

        <p v-else-if="unavailableMessage" class="project-overview__feedback" role="alert">
            {{ unavailableMessage }}
        </p>

        <div v-else-if="errorMessage" class="project-overview__feedback">
            <p role="alert">{{ errorMessage }}</p>
            <AppButton variant="secondary" @click="loadProject()">Try again</AppButton>
        </div>

        <section
            v-if="project && !loading"
            class="project-overview__branches"
            aria-labelledby="project-branches-heading"
        >
            <div class="project-overview__branches-header">
                <h3 id="project-branches-heading">Branches</h3>

                <span
                    v-if="!branchesLoading && !branchErrorMessage"
                    class="project-overview__count"
                >
                    {{ branches.length }} {{ branches.length === 1 ? 'branch' : 'branches' }}
                </span>
            </div>

            <p v-if="branchesLoading" role="status">Loading branches...</p>

            <div v-else-if="branchErrorMessage" class="project-overview__branch-feedback">
                <p role="alert">{{ branchErrorMessage }}</p>
                <AppButton variant="secondary" @click="retryBranches()"> Try again </AppButton>
            </div>

            <p v-else-if="branches.length === 0" class="project-overview__empty">
                No branches are available for this project.
            </p>

            <div v-else class="project-overview__branch-workspace">
                <div class="project-overview__branch-toolbar">
                    <div class="project-overview__selection">
                        <div class="project-overview__branch-picker">
                            <AppButton
                                variant="secondary"
                                aria-label="Choose design branch"
                                :aria-expanded="branchPickerOpen"
                                aria-controls="project-branch-choices"
                                @click="toggleBranchPicker($event)"
                            >
                                <span class="project-overview__selector-name">
                                    {{ selectedBranch?.name ?? 'Choose branch' }}
                                </span>
                                <span aria-hidden="true">
                                    {{ branchPickerOpen ? '▲' : '▼' }}
                                </span>
                            </AppButton>

                            <div
                                v-if="branchPickerOpen"
                                id="project-branch-choices"
                                class="project-overview__branch-options"
                                role="group"
                                aria-label="Available design branches"
                                @keydown.esc.stop.prevent="closeBranchPickerOnEscape"
                            >
                                <input
                                    ref="branchSearchInput"
                                    v-model="branchSearch"
                                    type="search"
                                    class="ui-field__input project-overview__branch-search"
                                    aria-label="Search branches"
                                    placeholder="Search branches"
                                    autocomplete="off"
                                />

                                <p
                                    v-if="filteredBranches.length === 0"
                                    class="project-overview__no-matches"
                                    role="status"
                                >
                                    No matching branches
                                </p>

                                <AppButton
                                    v-for="branch in filteredBranches"
                                    :key="branch.id"
                                    variant="secondary"
                                    :aria-pressed="selectedBranch?.id === branch.id"
                                    @click="chooseBranch(branch)"
                                >
                                    {{ branch.name }}
                                </AppButton>
                            </div>
                        </div>

                        <p
                            v-if="selectedBranch && selectedBranch.headRevisionId === null"
                            class="project-overview__branch-state"
                        >
                            No revisions yet
                        </p>

                        <p
                            v-else-if="selectedBranch && selectedBranch.headRevisionId"
                            class="project-overview__branch-state"
                        >
                            Head revision:
                            <RevisionId
                                :id="selectedBranch.headRevisionId"
                                label="branch head revision"
                            />
                        </p>

                        <div v-if="selectedBranch" class="project-overview__branch-actions">
                            <AppButton
                                variant="secondary"
                                :aria-label="`View history of ${selectedBranch.name}`"
                                @click="openBranchHistory(selectedBranch)"
                            >
                                View history
                            </AppButton>

                            <AppButton
                                variant="secondary"
                                :aria-label="`Create revision on ${selectedBranch.name}`"
                                @click="openCreateRevision(selectedBranch)"
                            >
                                Create revision
                            </AppButton>
                        </div>
                    </div>
                </div>

                <p v-if="!selectedBranch" class="project-overview__branch-feedback" role="alert">
                    The branch in this link is unavailable. Choose another branch.
                </p>
            </div>
        </section>

        <ProjectFileWorkspace
            v-if="project && !loading && selectedBranch"
            :key="project.id"
            :project-id="project.id"
            :branch-id="selectedBranch.id"
            :head-revision-id="selectedBranch.headRevisionId"
            @unauthorized="redirectToLogin"
            @add-file="openFileUpload"
        />
    </section>
</template>

<style scoped>
    .project-overview {
        box-sizing: border-box;
        width: 100%;
        padding: var(--space-6) var(--space-4);
    }

    .project-overview__header h2 {
        margin: 0;
        overflow-wrap: anywhere;
    }

    .project-overview__header p {
        color: var(--color-text-muted);
        overflow-wrap: anywhere;
    }

    .project-overview__feedback {
        margin-top: var(--space-5);
        padding: var(--space-4);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        background: var(--color-surface);
    }

    .project-overview__feedback p {
        margin-top: 0;
        color: var(--color-danger);
    }

    .project-overview__branches {
        margin-top: var(--space-5);
    }

    .project-overview__branches-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        flex-wrap: wrap;
        gap: var(--space-3);
    }

    .project-overview__branches-header h3 {
        margin: 0;
        font-size: var(--font-size-base);
    }

    .project-overview__count,
    .project-overview__branch-state,
    .project-overview__empty {
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }

    .project-overview__branch-workspace {
        margin-top: var(--space-3);
    }

    .project-overview__branch-toolbar {
        padding: var(--space-3);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        background: var(--color-surface);
    }

    .project-overview__selection {
        display: flex;
        align-items: center;
        flex-wrap: wrap;
        gap: var(--space-3);
        min-width: 0;
    }

    .project-overview__branch-picker {
        position: relative;
        min-width: 0;
        max-width: 100%;
    }

    .project-overview__selector-name {
        overflow-wrap: anywhere;
    }

    .project-overview__branch-options {
        position: absolute;
        top: 100%;
        left: 0;
        z-index: 2;
        display: flex;
        flex-direction: column;
        align-items: stretch;
        gap: var(--space-2);
        width: min(20rem, calc(100vw - var(--space-6) - var(--space-6)));
        max-height: 60vh;
        overflow-y: auto;
        margin-top: var(--space-2);
        padding: var(--space-3);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        background: var(--color-surface);
    }

    .project-overview__branch-search {
        box-sizing: border-box;
        width: 100%;
        min-width: 0;
    }

    .project-overview__no-matches {
        margin: 0;
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }

    .project-overview__branch-state {
        margin: 0;
        overflow-wrap: anywhere;
    }

    .project-overview__branch-actions {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-2);
        margin-left: auto;
    }

    @media (max-width: 52rem) {
        .project-overview__branch-actions {
            width: 100%;
            margin-left: 0;
        }
    }

    .project-overview__branch-feedback,
    .project-overview__empty {
        margin-top: var(--space-4);
    }

    .project-overview__branch-feedback p {
        color: var(--color-danger);
    }
</style>
