<script setup lang="ts">
    import { onMounted } from 'vue'
    import { RouterView } from 'vue-router'

    import SiteHeader from './components/SiteHeader.vue'
    import { useSessionStore } from './stores/session'

    const session = useSessionStore()

    onMounted(() => {
        if (session.status === 'unknown') {
            void session.restore().catch(() => {
                // Public pages remain usable if session verification fails.
            })
        }
    })
</script>

<template>
    <SiteHeader />
    <main id="main-content">
        <RouterView />
    </main>
</template>
