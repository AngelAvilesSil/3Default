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
