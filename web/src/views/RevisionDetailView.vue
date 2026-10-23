<script setup lang="ts">
    import { computed, onUnmounted, ref, watch } from 'vue'
    import { useRoute, useRouter } from 'vue-router'

    import { type ProjectFile } from '../api/projectFiles'
    import { getProject, ProjectRequestError, type Project } from '../api/projects'
    import {
        getProjectRevision,
        listProjectRevisionFiles,
        RevisionRequestError,
        type ProjectRevision,
    } from '../api/revisions'
    import AppButton from '../components/ui/AppButton.vue'
    import { useSessionStore } from '../stores/session'

    const props = defineProps<{
        projectId: string
        revisionId: string
    }>()

    const route = useRoute()
    const router = useRouter()
    const session = useSessionStore()

    type LoadState = 'loading' | 'ready' | 'unavailable' | 'error'

    const state = ref<LoadState>('loading')
    const project = ref<Project | null>(null)
    const revision = ref<ProjectRevision | null>(null)
    const files = ref<ProjectFile[]>([])
    const unavailableMessage = ref('')
    const errorMessage = ref('')

    let activeController: AbortController | null = null

    // This is a navigation hint, not revision ownership data.
    const sourceBranchId = computed(() => {
        const value = route.query.fromBranchId
        return typeof value === 'string' && value.trim() ? value : null
    })

    const returnDestination = computed(() => {
        if (sourceBranchId.value !== null) {
            return {
                name: 'project-branch-history',
                params: {
                    projectId: props.projectId,
                    branchId: sourceBranchId.value,
                },
            }
        }

        return {
            name: 'project-overview',
            params: { projectId: props.projectId },
        }
    })

    function isCurrent(
        controller: AbortController,
        projectId: string,
        revisionId: string,
    ): boolean {
        return (
            !controller.signal.aborted &&
            activeController === controller &&
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

    async function loadRevision(): Promise<void> {
        activeController?.abort()

        const controller = new AbortController()
        activeController = controller

        const projectId = props.projectId
        const revisionId = props.revisionId

        state.value = 'loading'
        project.value = null
        revision.value = null
        files.value = []
        unavailableMessage.value = ''
        errorMessage.value = ''

        if (!projectId.trim() || !revisionId.trim()) {
            state.value = 'unavailable'
            unavailableMessage.value = 'This revision link is invalid.'
            activeController = null
            return
        }

        try {
            const loadedProject = await getProject(projectId, controller.signal)

            if (!isCurrent(controller, projectId, revisionId)) {
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

            if (!isCurrent(controller, projectId, revisionId)) {
                return
            }

            if (loadedRevision.id !== revisionId || loadedRevision.projectId !== projectId) {
                throw new Error('Unexpected revision identity')
            }

            const loadedFiles = await listProjectRevisionFiles(
                projectId,
                revisionId,
                controller.signal,
            )

            if (!isCurrent(controller, projectId, revisionId)) {
                return
            }

            if (loadedFiles.some((file) => file.projectId !== projectId)) {
                throw new Error('Unexpected revision file identity')
            }

            project.value = loadedProject
            revision.value = loadedRevision
            files.value = loadedFiles
            state.value = 'ready'
        } catch (error) {
            if (!isCurrent(controller, projectId, revisionId)) {
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
                unavailableMessage.value = 'This revision was not found or is not accessible.'
                return
            }

            state.value = 'error'
            errorMessage.value = 'Unable to load revision details. Please try again.'
        } finally {
            if (activeController === controller) {
                activeController = null
            }
        }
    }

    watch(
        () => [props.projectId, props.revisionId],
        () => {
            void loadRevision()
        },
        { immediate: true },
    )

    onUnmounted(() => {
        activeController?.abort()
        activeController = null
    })
</script>

<template>
    <section class="revision-detail" aria-labelledby="revision-detail-heading">
        <RouterLink class="revision-detail__back" :to="returnDestination">
            {{ sourceBranchId === null ? 'Back to project' : 'Back to branch history' }}
        </RouterLink>

        <header class="revision-detail__header">
            <h2 id="revision-detail-heading">Revision details</h2>
            <p v-if="project">{{ project.name }}</p>
        </header>

        <p v-if="state === 'loading'" role="status">Loading revision details...</p>

        <p v-else-if="state === 'unavailable'" role="alert">
            {{ unavailableMessage }}
        </p>

        <div v-else-if="state === 'error'" class="revision-detail__feedback">
            <p role="alert">{{ errorMessage }}</p>
            <AppButton variant="secondary" @click="loadRevision()">Try again</AppButton>
        </div>

        <div v-else-if="state === 'ready' && revision">
            <section class="revision-detail__section" aria-labelledby="revision-message-heading">
                <h3 id="revision-message-heading">{{ revision.message }}</h3>

                <dl class="revision-detail__metadata">
                    <div>
                        <dt>Revision ID</dt>
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
                        <dt>Author ID</dt>
                        <dd>
                            <code>{{ revision.authorUserId }}</code>
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
            </section>

            <section class="revision-detail__section" aria-labelledby="revision-files-heading">
                <h3 id="revision-files-heading">Files in this revision ({{ files.length }})</h3>

                <p class="revision-detail__note">
                    These are the exact project-file records selected for this revision, not every
                    file uploaded to the project. This view does not calculate CAD geometry
                    differences.
                </p>

                <p v-if="files.length === 0" class="revision-detail__empty">
                    This revision contains no source files.
                </p>

                <ul v-else class="revision-detail__files">
                    <li v-for="file in files" :key="file.id" class="revision-detail__file">
                        <strong>{{ file.originalFilename }}</strong>

                        <dl class="revision-detail__metadata">
                            <div>
                                <dt>File ID</dt>
                                <dd>
                                    <code>{{ file.id }}</code>
                                </dd>
                            </div>
                            <div>
                                <dt>SHA-256</dt>
                                <dd>
                                    <code>{{ file.contentSha256 }}</code>
                                </dd>
                            </div>
                            <div>
                                <dt>Uploaded</dt>
                                <dd>
                                    <time :datetime="file.createdAt">
                                        {{ new Date(file.createdAt).toLocaleString() }}
                                    </time>
                                </dd>
                            </div>
                        </dl>
                    </li>
                </ul>
            </section>
        </div>
    </section>
</template>

<style scoped>
    .revision-detail {
        box-sizing: border-box;
        width: 100%;
        padding: var(--space-6) var(--space-4);
    }

    .revision-detail__back {
        color: var(--color-action);
    }

    .revision-detail__back:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
    }

    .revision-detail__header {
        margin-top: var(--space-4);
    }

    .revision-detail__header h2 {
        margin: 0;
    }

    .revision-detail__header p,
    .revision-detail__note,
    .revision-detail__empty {
        color: var(--color-text-muted);
    }

    .revision-detail__section,
    .revision-detail__feedback {
        margin-top: var(--space-5);
        padding: var(--space-4);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        background: var(--color-surface);
    }

    .revision-detail__section h3 {
        margin: 0;
        overflow-wrap: anywhere;
    }

    .revision-detail__metadata {
        display: grid;
        gap: var(--space-2);
        margin: var(--space-4) 0 0;
    }

    .revision-detail__metadata > div {
        display: grid;
        grid-template-columns: minmax(0, 7rem) minmax(0, 1fr);
        gap: var(--space-2);
    }

    .revision-detail__metadata dt {
        color: var(--color-text-muted);
    }

    .revision-detail__metadata dd {
        min-width: 0;
        margin: 0;
        overflow-wrap: anywhere;
    }

    .revision-detail__metadata code {
        font-family: var(--font-family-mono);
        overflow-wrap: anywhere;
    }

    .revision-detail__files {
        padding: 0;
        margin: var(--space-4) 0 0;
        list-style: none;
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
    }

    .revision-detail__file {
        padding: var(--space-4);
        overflow-wrap: anywhere;
    }

    .revision-detail__file + .revision-detail__file {
        border-top: 1px solid var(--color-border);
    }

    .revision-detail__file .revision-detail__metadata {
        margin-top: var(--space-2);
    }

    @media (max-width: 40rem) {
        .revision-detail__metadata > div {
            grid-template-columns: minmax(0, 1fr);
        }
    }
</style>
