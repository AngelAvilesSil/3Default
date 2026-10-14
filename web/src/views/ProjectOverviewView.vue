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
    import AppButton from '../components/ui/AppButton.vue'
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

    let activeController: AbortController | null = null
    let activeBranchController: AbortController | null = null

    function resetBranches(): void {
        activeBranchController?.abort()
        activeBranchController = null

        branches.value = []
        branchesLoading.value = false
        branchErrorMessage.value = ''
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
                const returnPath = route.fullPath

                session.invalidate()

                await router.replace({
                    name: 'login',
                    query: { redirect: returnPath },
                })

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
                    const returnPath = route.fullPath

                    session.invalidate()

                    await router.replace({
                        name: 'login',
                        query: { redirect: returnPath },
                    })

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

    onUnmounted(() => {
        activeController?.abort()
        activeController = null
        resetBranches()
    })
</script>

<template>
    <section class="project-overview" aria-labelledby="project-overview-heading">
        <RouterLink class="project-overview__back" :to="{ name: 'my-projects' }">
            ← My Projects
        </RouterLink>

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

            <ul v-else class="project-overview__branch-list" aria-label="Project branches">
                <li
                    v-for="branch in branches"
                    :key="branch.id"
                    class="project-overview__branch-item"
                >
                    <div class="project-overview__branch-row">
                        <strong>{{ branch.name }}</strong>

                        <span
                            v-if="branch.headRevisionId === null"
                            class="project-overview__branch-state"
                        >
                            No revisions yet
                        </span>

                        <span v-else class="project-overview__branch-state">
                            Head revision:
                            <code>{{ branch.headRevisionId }}</code>
                        </span>
                    </div>
                </li>
            </ul>
        </section>
    </section>
</template>

<style scoped>
    .project-overview {
        box-sizing: border-box;
        width: 100%;
        padding: var(--space-6) var(--space-4);
    }

    .project-overview__back {
        color: var(--color-action);
        font-weight: var(--font-weight-medium);
        text-decoration: none;
    }

    .project-overview__back:hover {
        text-decoration: underline;
    }

    .project-overview__back:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
    }

    .project-overview__header {
        margin-top: var(--space-5);
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
        margin-top: var(--space-6);
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
    }

    .project-overview__count,
    .project-overview__branch-state,
    .project-overview__empty {
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }

    .project-overview__branch-list {
        margin: var(--space-4) 0 0;
        padding: 0;
        list-style: none;
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
    }

    .project-overview__branch-item {
        padding: var(--space-4);
        overflow-wrap: anywhere;
    }

    .project-overview__branch-item + .project-overview__branch-item {
        border-top: 1px solid var(--color-border);
    }

    .project-overview__branch-row {
        display: flex;
        align-items: start;
        justify-content: space-between;
        flex-wrap: wrap;
        gap: var(--space-3);
    }

    .project-overview__branch-state code {
        font-family: var(--font-family-mono);
        overflow-wrap: anywhere;
    }

    .project-overview__branch-feedback,
    .project-overview__empty {
        margin-top: var(--space-4);
    }

    .project-overview__branch-feedback p {
        color: var(--color-danger);
    }
</style>
