<script setup lang="ts">
    import { onMounted, onUnmounted, ref } from 'vue'
    import { useRoute, useRouter } from 'vue-router'

    import { listProjects, ProjectRequestError, type Project } from '../api/projects'
    import AppButton from '../components/ui/AppButton.vue'
    import { useSessionStore } from '../stores/session'

    const route = useRoute()
    const router = useRouter()
    const session = useSessionStore()

    const projects = ref<Project[]>([])
    const loading = ref(true)
    const errorMessage = ref('')

    const dateFormatter = new Intl.DateTimeFormat('en-CA', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
    })

    let activeController: AbortController | null = null

    function formatCreatedAt(value: string): string {
        const date = new Date(value)

        if (Number.isNaN(date.getTime())) {
            return 'Creation date unavailable'
        }

        return `Created ${dateFormatter.format(date)}`
    }

    async function loadProjects(): Promise<void> {
        activeController?.abort()

        const controller = new AbortController()
        activeController = controller

        loading.value = true
        errorMessage.value = ''

        try {
            const result = await listProjects(controller.signal)

            if (controller.signal.aborted || activeController !== controller) {
                return
            }

            projects.value = result
        } catch (error) {
            if (controller.signal.aborted || activeController !== controller) {
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

            errorMessage.value = 'Unable to load projects. Please try again.'
        } finally {
            if (activeController === controller) {
                activeController = null

                if (!controller.signal.aborted) {
                    loading.value = false
                }
            }
        }
    }

    onMounted(() => {
        void loadProjects()
    })

    onUnmounted(() => {
        activeController?.abort()
        activeController = null
    })
</script>

<template>
    <section class="my-projects" aria-labelledby="my-projects-heading">
        <header class="my-projects__header">
            <div>
                <h2 id="my-projects-heading">My Projects</h2>
                <p>Your engineering projects, in one place.</p>
            </div>

            <RouterLink class="my-projects__create-link" :to="{ name: 'create-project' }">
                Create project
            </RouterLink>
        </header>

        <p v-if="loading" class="my-projects__feedback" role="status">Loading projects...</p>

        <div v-else-if="errorMessage" class="my-projects__feedback">
            <p class="my-projects__error" role="alert">{{ errorMessage }}</p>

            <AppButton variant="secondary" @click="loadProjects()"> Try again </AppButton>
        </div>

        <div v-else-if="projects.length === 0" class="my-projects__feedback">
            <h3>No projects yet.</h3>
            <p>Create your first project to start organizing your engineering work.</p>

            <RouterLink class="my-projects__empty-link" :to="{ name: 'create-project' }">
                Create your first project
            </RouterLink>
        </div>

        <ul v-else class="my-projects__list" aria-label="Your projects">
            <li v-for="project in projects" :key="project.id" class="my-projects__item">
                <div class="my-projects__thumbnail" aria-hidden="true">
                    <span>3D</span>
                </div>

                <div class="my-projects__details">
                    <h3>{{ project.name }}</h3>

                    <p v-if="project.description" class="my-projects__description">
                        {{ project.description }}
                    </p>

                    <p class="my-projects__metadata">
                        <time :datetime="project.createdAt">
                            {{ formatCreatedAt(project.createdAt) }}
                        </time>
                    </p>
                </div>
            </li>
        </ul>
    </section>
</template>

<style scoped>
    .my-projects {
        max-width: 62rem;
        margin-inline: auto;
        padding: var(--space-6) var(--space-4);
    }

    .my-projects__header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        flex-wrap: wrap;
        gap: var(--space-4);
    }

    .my-projects__create-link {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-height: 2.5rem;
        padding: var(--space-2) var(--space-4);
        border-radius: var(--radius-sm);
        background: var(--color-action);
        color: var(--color-on-action);
        font-size: var(--font-size-sm);
        font-weight: var(--font-weight-semibold);
        text-decoration: none;
    }

    .my-projects__create-link:hover {
        background: var(--color-action-hover);
    }

    .my-projects__create-link:focus-visible,
    .my-projects__empty-link:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
    }

    .my-projects__header h2 {
        margin: 0;
    }

    .my-projects__header p {
        margin: var(--space-2) 0 0;
        color: var(--color-text-muted);
    }

    .my-projects__feedback {
        margin-top: var(--space-5);
        padding: var(--space-5);
        border-radius: var(--radius-md);
        background: var(--color-surface);
    }

    .my-projects__empty-link {
        display: inline-block;
        margin-top: var(--space-4);
        color: var(--color-action);
        font-weight: var(--font-weight-semibold);
        text-underline-offset: 0.2em;
    }

    .my-projects__empty-link:hover {
        color: var(--color-action-hover);
    }

    .my-projects__feedback h3 {
        margin: 0;
        font-size: var(--font-size-lg);
    }

    .my-projects__feedback p {
        margin: var(--space-2) 0 0;
        color: var(--color-text-muted);
    }

    .my-projects__feedback .my-projects__error {
        margin: 0 0 var(--space-3);
        color: var(--color-danger);
    }

    .my-projects__list {
        margin: var(--space-5) 0 0;
        padding: 0;
        list-style: none;
        border-top: 1px solid var(--color-border);
    }

    .my-projects__item {
        display: grid;
        grid-template-columns: 3.5rem minmax(0, 1fr);
        gap: var(--space-4);
        align-items: start;
        padding: var(--space-4) 0;
        border-bottom: 1px solid var(--color-border);
    }

    .my-projects__thumbnail {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 100%;
        aspect-ratio: 1;
        border: 1px solid var(--color-border);
        border-radius: var(--radius-sm);
        background: var(--color-surface);
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
        font-weight: var(--font-weight-semibold);
    }

    .my-projects__details {
        min-width: 0;
    }

    .my-projects__details h3 {
        margin: 0;
        font-size: var(--font-size-base);
        font-weight: var(--font-weight-semibold);
        overflow-wrap: anywhere;
    }

    .my-projects__description {
        margin: var(--space-1) 0 0;
        color: var(--color-text-muted);
        overflow-wrap: anywhere;
    }

    .my-projects__metadata {
        margin: var(--space-2) 0 0;
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }
</style>
