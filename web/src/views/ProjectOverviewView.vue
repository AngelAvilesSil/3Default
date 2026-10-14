<script setup lang="ts">
    import { onUnmounted, ref, watch } from 'vue'
    import { useRoute, useRouter } from 'vue-router'

    import { getProject, ProjectRequestError, type Project } from '../api/projects'
    import AppButton from '../components/ui/AppButton.vue'
    import { useSessionStore } from '../stores/session'

    const route = useRoute()
    const router = useRouter()
    const session = useSessionStore()

    const project = ref<Project | null>(null)
    const loading = ref(true)
    const unavailableMessage = ref('')
    const errorMessage = ref('')

    let activeController: AbortController | null = null

    async function loadProject(): Promise<void> {
        activeController?.abort()

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
        } catch (error) {
            if (controller.signal.aborted || activeController !== controller) {
                return
            }

            if (error instanceof ProjectRequestError) {
                if (error.status === 401) {
                    session.invalidate()

                    await router.replace({
                        name: 'login',
                        query: { redirect: route.fullPath },
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
</style>
