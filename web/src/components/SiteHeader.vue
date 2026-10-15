<script setup lang="ts">
    import { ref } from 'vue'
    import { RouterLink, useRouter } from 'vue-router'

    import AppButton from './ui/AppButton.vue'
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
            await router.replace({ name: 'landing' })
        } catch {
            errorMessage.value = 'Sign out is temporarily unavailable. Please try again.'
        } finally {
            signingOut.value = false
        }
    }
</script>

<template>
    <header class="site-header">
        <nav class="site-header__nav" aria-label="Main navigation">
            <h1 class="site-header__title">
                <RouterLink :to="{ name: 'landing' }" class="site-header__brand">
                    3Default
                </RouterLink>
            </h1>

            <RouterLink
                v-if="session.status === 'authenticated'"
                :to="{ name: 'my-projects' }"
                class="site-header__link"
            >
                My Projects
            </RouterLink>

            <div class="site-header__account">
                <template v-if="session.status === 'authenticated'">
                    <span v-if="session.user" class="site-header__identity">
                        {{ session.user.displayName }}
                    </span>

                    <AppButton variant="secondary" :loading="signingOut" @click="handleSignOut">
                        Sign out
                    </AppButton>
                </template>

                <RouterLink
                    v-else-if="session.status === 'unauthenticated' || session.status === 'error'"
                    :to="{ name: 'login' }"
                    class="site-header__link"
                >
                    Sign in
                </RouterLink>
            </div>
        </nav>

        <p v-if="errorMessage" class="site-header__error" role="alert">
            {{ errorMessage }}
        </p>
    </header>
</template>

<style scoped>
    .site-header {
        border-bottom: 1px solid var(--color-border);
        background: var(--color-page);
    }

    .site-header__nav {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: var(--space-3) var(--space-5);
        padding: var(--space-3) var(--space-4);
    }

    .site-header__title {
        margin: 0;
        font-size: var(--font-size-lg);
        font-weight: var(--font-weight-semibold);
    }

    .site-header__brand {
        color: var(--color-text);
        text-decoration: none;
    }

    .site-header__link {
        color: var(--color-action);
        text-decoration: none;
    }

    .site-header__link:hover,
    .site-header__brand:hover {
        text-decoration: underline;
    }

    .site-header__account {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: var(--space-3);
        margin-left: auto;
    }

    .site-header__identity {
        color: var(--color-text-muted);
        overflow-wrap: anywhere;
    }

    .site-header a:focus-visible {
        outline: 2px solid var(--color-focus-ring);
        outline-offset: 3px;
        border-radius: var(--radius-sm);
    }

    .site-header__error {
        margin: 0;
        padding: var(--space-2) var(--space-4);
        color: var(--color-danger);
    }

    @media (max-width: 480px) {
        .site-header__account {
            margin-left: 0;
        }
    }
</style>
