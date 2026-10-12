<script setup lang="ts">
    const props = withDefaults(
        defineProps<{
            id: string
            label: string
            modelValue: string
            name?: string
            placeholder?: string
            error?: string
            rows?: number
            disabled?: boolean
        }>(),
        {
            rows: 4,
            disabled: false,
        },
    )

    const emit = defineEmits<{
        'update:modelValue': [value: string]
    }>()

    function handleInput(event: Event): void {
        const textarea = event.target as HTMLTextAreaElement
        emit('update:modelValue', textarea.value)
    }
</script>

<template>
    <div class="ui-field">
        <label class="ui-field__label" :for="props.id">
            {{ props.label }}
        </label>

        <textarea
            :id="props.id"
            class="ui-field__input"
            :class="{ 'ui-field__input--invalid': props.error }"
            :name="props.name"
            :value="props.modelValue"
            :placeholder="props.placeholder"
            :rows="props.rows"
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
    .ui-field__input {
        min-height: 6rem;
        resize: vertical;
    }
</style>
