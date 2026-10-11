<script setup lang="ts">
    import { RouterLink } from 'vue-router'
</script>

<template>
    <section class="landing">
        <h2>Engineering projects, with revision history.</h2>
        <p>
            3Default is being rebuilt around engineering project version control and browser-based
            3D visualization.
        </p>
        <RouterLink to="/login">Sign in</RouterLink>
    </section>
</template>

<style scoped>
    .landing {
        max-width: 42rem;
        margin-inline: auto;
        padding: var(--space-6) var(--space-4);
    }

    .landing h2 {
        margin-top: 0;
    }

    .landing p {
        color: var(--color-text-muted);
    }

    .landing a {
        color: var(--color-action);
    }
</style>
