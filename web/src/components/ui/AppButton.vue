<script setup lang="ts">
    type ButtonVariant = 'primary' | 'secondary'
    type ButtonType = 'button' | 'submit' | 'reset'

    const props = withDefaults(
        defineProps<{
            variant?: ButtonVariant
            type?: ButtonType
            disabled?: boolean
            loading?: boolean
        }>(),
        {
            variant: 'primary',
            type: 'button',
            disabled: false,
            loading: false,
        },
    )
</script>

<template>
    <button
        class="ui-button"
        :class="`ui-button--${props.variant}`"
        :type="props.type"
        :disabled="props.disabled || props.loading"
        :aria-busy="props.loading ? 'true' : undefined"
    >
        <slot />
    </button>
</template>

<style scoped>
    .ui-button {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        gap: var(--space-2);
        min-height: 2.5rem;
        padding: var(--space-2) var(--space-4);
        border: 1px solid transparent;
        border-radius: var(--radius-sm);
        font-size: var(--font-size-sm);
        font-weight: var(--font-weight-semibold);
        line-height: var(--line-height-body);
        cursor: pointer;
        transition: background-color 120ms ease;
    }

    .ui-button--primary {
        color: var(--color-on-action);
        background: var(--color-action);
    }

    .ui-button--primary:hover:not(:disabled) {
        background: var(--color-action-hover);
    }

    .ui-button--secondary {
        color: var(--color-text);
        background: var(--color-surface);
        border-color: var(--color-border);
    }

    .ui-button--secondary:hover:not(:disabled) {
        background: var(--color-page);
    }

    .ui-button:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
    }

    .ui-button:disabled {
        cursor: not-allowed;
        opacity: 0.6;
    }
</style>
