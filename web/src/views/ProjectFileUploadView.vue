<script setup lang="ts">
    const props = defineProps<{
        projectId: string
    }>()
</script>

<template>
    <section class="project-file-upload" aria-labelledby="project-file-upload-heading">
        <RouterLink
            class="project-file-upload__back"
            :to="{
                name: 'project-overview',
                params: { projectId: props.projectId },
            }"
        >
            Back to project
        </RouterLink>

        <header class="project-file-upload__header">
            <h2 id="project-file-upload-heading">Upload files</h2>
            <p>
                Select source files for this project and review them before uploading. Files are not
                uploaded automatically.
            </p>
        </header>
    </section>
</template>

<style scoped>
    .project-file-upload {
        max-width: 62rem;
        margin-inline: auto;
        padding: var(--space-6) var(--space-4);
    }

    .project-file-upload__back {
        display: inline-flex;
        padding: var(--space-2) 0;
        color: var(--color-action);
        font-weight: var(--font-weight-medium);
        text-decoration: none;
    }

    .project-file-upload__back:hover {
        text-decoration: underline;
    }

    .project-file-upload__back:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
        border-radius: var(--radius-sm);
    }

    .project-file-upload__header {
        margin-top: var(--space-4);
    }

    .project-file-upload__header h2 {
        margin: 0;
    }

    .project-file-upload__header p {
        margin: var(--space-2) 0 0;
        color: var(--color-text-muted);
    }
</style>
