<script setup lang="ts">
    import { ref } from 'vue'
    import { useRoute, useRouter } from 'vue-router'

    import AppButton from '../components/ui/AppButton.vue'
    import { safeReturnPath } from '../router/authRedirect'
    import { useSessionStore } from '../stores/session'

    const route = useRoute()
    const router = useRouter()
    const session = useSessionStore()

    const retrying = ref(false)
    const errorMessage = ref('')

    async function retry(): Promise<void> {
        if (retrying.value) {
            return
        }

        retrying.value = true
        errorMessage.value = ''

        try {
            await session.restore()
            await router.replace(safeReturnPath(router, route.query.redirect))
        } catch {
            errorMessage.value = 'Your session could not be verified. Please try again.'
        } finally {
            retrying.value = false
        }
    }
</script>

<template>
    <section class="session-error">
        <h2>Session temporarily unavailable</h2>

        <p>We could not verify your session. This does not mean you have been signed out.</p>

        <AppButton :loading="retrying" @click="retry"> Try again </AppButton>

        <p v-if="errorMessage" role="alert">
            {{ errorMessage }}
        </p>
    </section>
</template>

<style scoped>
    .session-error {
        max-width: 36rem;
        margin-inline: auto;
        padding: var(--space-6) var(--space-4);
    }

    .session-error h2 {
        margin-top: 0;
    }

    .session-error p {
        color: var(--color-text-muted);
    }

    .session-error [role='alert'] {
        color: var(--color-danger);
    }
</style>
