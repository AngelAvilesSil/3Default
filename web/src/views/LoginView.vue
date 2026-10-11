<script setup lang="ts">
    import { useRoute, useRouter } from 'vue-router'

    import LoginForm from '../components/auth/LoginForm.vue'
    import { safeReturnPath } from '../router/authRedirect'

    const route = useRoute()
    const router = useRouter()

    async function handleAuthenticated(): Promise<void> {
        await router.replace(safeReturnPath(router, route.query.redirect))
    }
</script>

<template>
    <div class="login-page">
        <LoginForm @authenticated="handleAuthenticated" />
    </div>
</template>

<style scoped>
    .login-page {
        display: flex;
        justify-content: center;
        padding: var(--space-6) var(--space-4);
        background: var(--color-surface);
    }
</style>
