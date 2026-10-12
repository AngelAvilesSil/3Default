<script setup lang="ts">
    import { onUnmounted, ref, watch } from 'vue'
    import { useRoute, useRouter } from 'vue-router'

    import { createProject, ProjectRequestError } from '../api/projects'
    import AppButton from '../components/ui/AppButton.vue'
    import AppTextArea from '../components/ui/AppTextArea.vue'
    import AppTextField from '../components/ui/AppTextField.vue'
    import { useSessionStore } from '../stores/session'

    const route = useRoute()
    const router = useRouter()
    const session = useSessionStore()

    const projectName = ref('')
    const description = ref('')
    const nameError = ref('')
    const errorMessage = ref('')
    const submitting = ref(false)

    let activeController: AbortController | null = null

    watch(projectName, () => {
        nameError.value = ''
    })

    async function handleSubmit(): Promise<void> {
        if (submitting.value) {
            return
        }

        const name = projectName.value.trim()
        const normalizedDescription = description.value.trim()

        nameError.value = ''
        errorMessage.value = ''

        if (!name) {
            nameError.value = 'Project name is required.'
            return
        }

        const controller = new AbortController()
        activeController = controller
        submitting.value = true

        try {
            await createProject(
                {
                    name,
                    ...(normalizedDescription ? { description: normalizedDescription } : {}),
                },
                controller.signal,
            )

            if (!controller.signal.aborted) {
                await router.push({ name: 'my-projects' })
            }
        } catch (error) {
            if (controller.signal.aborted) {
                return
            }

            if (error instanceof ProjectRequestError) {
                if (error.status === 409) {
                    nameError.value = 'A project with this name already exists.'
                    return
                }

                if (error.status === 401) {
                    session.invalidate()

                    await router.replace({
                        name: 'login',
                        query: { redirect: route.fullPath },
                    })
                    return
                }

                if (error.status === 400) {
                    errorMessage.value =
                        'The project details were not accepted. Review them and try again.'
                    return
                }
            }

            errorMessage.value = 'Unable to create project. Please try again.'
        } finally {
            if (activeController === controller) {
                activeController = null

                if (!controller.signal.aborted) {
                    submitting.value = false
                }
            }
        }
    }

    function cancel(): void {
        if (!submitting.value) {
            void router.push({ name: 'my-projects' })
        }
    }

    onUnmounted(() => {
        activeController?.abort()
        activeController = null
    })
</script>

<template>
    <section class="create-project" aria-labelledby="create-project-heading">
        <RouterLink class="create-project__back" :to="{ name: 'my-projects' }">
            ← My Projects
        </RouterLink>

        <header class="create-project__header">
            <h2 id="create-project-heading">Create a project</h2>
            <p>Set up a new project in your engineering workspace.</p>
        </header>

        <form class="create-project__form" novalidate @submit.prevent="handleSubmit">
            <section class="create-project__details" aria-labelledby="project-details-heading">
                <h3 id="project-details-heading">Project details</h3>

                <div class="create-project__fields">
                    <AppTextField
                        id="project-name"
                        v-model="projectName"
                        label="Project name *"
                        name="name"
                        placeholder="e.g. Robotic Gripper"
                        autocomplete="off"
                        :required="true"
                        :disabled="submitting"
                        :error="nameError"
                    />

                    <AppTextArea
                        id="project-description"
                        v-model="description"
                        label="Description (optional)"
                        name="description"
                        placeholder="Describe what you're working on..."
                        :disabled="submitting"
                    />
                </div>
            </section>

            <p v-if="errorMessage" class="create-project__error" role="alert">
                {{ errorMessage }}
            </p>

            <div class="create-project__actions">
                <AppButton variant="secondary" :disabled="submitting" @click="cancel">
                    Cancel
                </AppButton>

                <AppButton type="submit" :loading="submitting">
                    {{ submitting ? 'Creating project…' : 'Create project' }}
                </AppButton>
            </div>
        </form>
    </section>
</template>

<style scoped>
    .create-project {
        max-width: 62rem;
        margin-inline: auto;
        padding: var(--space-6) var(--space-4);
    }

    .create-project__back {
        display: inline-flex;
        padding: var(--space-2) 0;
        color: var(--color-action);
        font-weight: var(--font-weight-medium);
        text-decoration: none;
    }

    .create-project__back:hover {
        text-decoration: underline;
    }

    .create-project__back:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
        border-radius: var(--radius-sm);
    }

    .create-project__header {
        margin-top: var(--space-4);
    }

    .create-project__header h2 {
        margin: 0;
    }

    .create-project__header p {
        margin: var(--space-2) 0 0;
        color: var(--color-text-muted);
    }

    .create-project__form {
        margin-top: var(--space-5);
    }

    .create-project__details {
        padding: var(--space-5);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        background: var(--color-surface);
    }

    .create-project__details h3 {
        margin: 0 0 var(--space-5);
        font-size: var(--font-size-lg);
        font-weight: var(--font-weight-semibold);
    }

    .create-project__fields {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
    }

    .create-project__error {
        margin: var(--space-4) 0 0;
        color: var(--color-danger);
        font-size: var(--font-size-sm);
    }

    .create-project__actions {
        display: flex;
        justify-content: flex-end;
        flex-wrap: wrap;
        gap: var(--space-3);
        margin-top: var(--space-5);
    }
</style>
