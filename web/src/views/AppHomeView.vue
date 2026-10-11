<script setup lang="ts">
    import { ref } from 'vue'
    import { RouterLink, useRouter } from 'vue-router'

    import AppButton from '../components/ui/AppButton.vue'
    import { useSessionStore } from '../stores/session'

    const session = useSessionStore()
    const router = useRouter()

    const signingOut = ref(false)
    const errorMessage = ref('')

    async function handleSignOut(): Promise<void> {
        if (signingOut.value) {
            return
        }

        signingOut.value = true
        errorMessage.value = ''

        try {
            await session.logout()
            await router.replace({ name: 'login' })
        } catch {
            errorMessage.value = 'Sign out is temporarily unavailable. Please try again.'
        } finally {
            signingOut.value = false
        }
    }
</script>

<template>
    <section class="app-home">
        <h2>My 3Default</h2>

        <p v-if="session.user">Signed in as {{ session.user.displayName }}.</p>

        <p class="app-home__description">
            Project management will be available here in a later development milestone.
        </p>

        <div class="app-home__actions">
            <RouterLink to="/">Visit public home</RouterLink>

            <AppButton variant="secondary" :loading="signingOut" @click="handleSignOut">
                Sign out
            </AppButton>
        </div>

        <p v-if="errorMessage" role="alert">
            {{ errorMessage }}
        </p>
    </section>
</template>

<style scoped>
    .app-home {
        max-width: 44rem;
        margin-inline: auto;
        padding: var(--space-6) var(--space-4);
    }

    .app-home h2 {
        margin-top: 0;
    }

    .app-home__description {
        color: var(--color-text-muted);
    }

    .app-home__actions {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: var(--space-4);
        margin-top: var(--space-5);
    }

    .app-home__actions a {
        color: var(--color-action);
    }

    .app-home [role='alert'] {
        color: var(--color-danger);
    }
</style>
