<script setup lang="ts">
    const props = defineProps<{
        projectId: string
        branchId: string
    }>()
</script>

<template>
    <section class="create-revision" aria-labelledby="create-revision-heading">
        <h2 id="create-revision-heading">Create revision</h2>

        <p>
            The revision editor is not available in this checkpoint. No revision has been created.
        </p>

        <p class="create-revision__context">
            Selected branch ID:
            <code>{{ props.branchId }}</code>
        </p>

        <RouterLink
            :to="{
                name: 'project-overview',
                params: { projectId: props.projectId },
            }"
        >
            Back to project
        </RouterLink>
    </section>
</template>

<style scoped>
    .create-revision {
        box-sizing: border-box;
        max-width: 48rem;
        margin: 0 auto;
        padding: var(--space-6) var(--space-4);
    }

    .create-revision h2 {
        margin: 0;
    }

    .create-revision p {
        color: var(--color-text-muted);
    }

    .create-revision__context {
        overflow-wrap: anywhere;
    }

    .create-revision__context code {
        font-family: var(--font-family-mono);
    }

    .create-revision a {
        color: var(--color-action);
    }

    .create-revision a:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
    }
</style>
