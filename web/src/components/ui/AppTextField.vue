<script setup lang="ts">
    type TextFieldType = 'text' | 'email' | 'password'

    const props = withDefaults(
        defineProps<{
            id: string
            label: string
            modelValue: string
            type?: TextFieldType
            name?: string
            autocomplete?: string
            placeholder?: string
            error?: string
            required?: boolean
            disabled?: boolean
        }>(),
        {
            type: 'text',
            required: false,
            disabled: false,
        },
    )

    const emit = defineEmits<{
        'update:modelValue': [value: string]
    }>()

    function handleInput(event: Event): void {
        const input = event.target as HTMLInputElement
        emit('update:modelValue', input.value)
    }
</script>

<template>
    <div class="ui-field">
        <label class="ui-field__label" :for="props.id">
            {{ props.label }}
        </label>

        <input
            :id="props.id"
            class="ui-field__input"
            :class="{ 'ui-field__input--invalid': props.error }"
            :name="props.name"
            :type="props.type"
            :value="props.modelValue"
            :autocomplete="props.autocomplete"
            :placeholder="props.placeholder"
            :required="props.required"
            :disabled="props.disabled"
            :aria-invalid="props.error ? 'true' : undefined"
            :aria-describedby="props.error ? `${props.id}-error` : undefined"
            @input="handleInput"
        />

        <p v-if="props.error" :id="`${props.id}-error`" class="ui-field__error">
            {{ props.error }}
        </p>
    </div>
</template>

<style scoped>
    .ui-field {
        display: flex;
        flex-direction: column;
        gap: var(--space-2);
    }

    .ui-field__label {
        font-size: var(--font-size-sm);
        font-weight: var(--font-weight-semibold);
        color: var(--color-text);
    }

    .ui-field__input {
        width: 100%;
        min-height: 2.5rem;
        padding: var(--space-2) var(--space-3);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-sm);
        color: var(--color-text);
        background: var(--color-page);
    }

    .ui-field__input:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 1px;
    }

    .ui-field__input:disabled {
        cursor: not-allowed;
        opacity: 0.6;
        background: var(--color-surface);
    }

    .ui-field__input--invalid {
        border-color: var(--color-danger);
    }

    .ui-field__error {
        margin: 0;
        color: var(--color-danger);
        font-size: var(--font-size-sm);
    }
</style>
