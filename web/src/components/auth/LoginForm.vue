<script setup lang="ts">
    import { ref } from 'vue'

    import { AuthenticationRequestError } from '../../api/auth'
    import { useSessionStore } from '../../stores/session'
    import AppButton from '../ui/AppButton.vue'
    import AppTextField from '../ui/AppTextField.vue'

    const emit = defineEmits<{
        authenticated: []
    }>()

    const session = useSessionStore()

    const email = ref('')
    const password = ref('')
    const submitting = ref(false)
    const errorMessage = ref('')

    async function handleSubmit(): Promise<void> {
        if (submitting.value) {
            return
        }

        const normalizedEmail = email.value.trim()

        errorMessage.value = ''

        if (!normalizedEmail || !password.value) {
            errorMessage.value = 'Enter your email and password.'
            return
        }

        submitting.value = true

        try {
            await session.login({
                email: normalizedEmail,
                password: password.value,
            })

            emit('authenticated')
        } catch (error) {
            if (error instanceof AuthenticationRequestError) {
                if (error.status === 401) {
                    errorMessage.value = 'Incorrect email or password.'
                } else if (error.status === 429) {
                    errorMessage.value = 'Too many sign-in attempts. Please try again later.'
                } else {
                    errorMessage.value = 'Sign in is temporarily unavailable. Please try again.'
                }
            } else {
                errorMessage.value = 'Sign in is temporarily unavailable. Please try again.'
            }
        } finally {
            submitting.value = false
        }
    }
</script>

<template>
    <section class="login-form" aria-labelledby="login-heading">
        <header class="login-form__header">
            <h2 id="login-heading" class="login-form__heading">Sign in to 3Default</h2>

            <p class="login-form__description">Access your engineering projects.</p>
        </header>

        <form class="login-form__fields" @submit.prevent="handleSubmit">
            <AppTextField
                id="login-email"
                v-model="email"
                label="Email address"
                name="email"
                type="email"
                autocomplete="username"
                :required="true"
                :disabled="submitting"
            />

            <AppTextField
                id="login-password"
                v-model="password"
                label="Password"
                name="password"
                type="password"
                autocomplete="current-password"
                :required="true"
                :disabled="submitting"
            />

            <p v-if="errorMessage" class="login-form__error" role="alert">
                {{ errorMessage }}
            </p>

            <AppButton type="submit" :loading="submitting" :disabled="submitting">
                {{ submitting ? 'Signing in…' : 'Sign in' }}
            </AppButton>
        </form>
    </section>
</template>

<style scoped>
    .login-form {
        width: 100%;
        max-width: 26rem;
        margin-inline: auto;
        padding: var(--space-5);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        background: var(--color-page);
    }

    .login-form__header {
        margin-bottom: var(--space-5);
        text-align: center;
    }

    .login-form__heading {
        margin: 0;
        color: var(--color-text);
        font-size: var(--font-size-lg);
        font-weight: var(--font-weight-semibold);
    }

    .login-form__description {
        margin: var(--space-2) 0 0;
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }

    .login-form__fields {
        display: flex;
        flex-direction: column;
        gap: var(--space-4);
    }

    .login-form__error {
        margin: 0;
        color: var(--color-danger);
        font-size: var(--font-size-sm);
    }

    .login-form__fields > :last-child {
        width: 100%;
    }
</style>
