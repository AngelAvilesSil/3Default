<script setup lang="ts">
    import { onUnmounted, ref, watch } from 'vue'

    const props = withDefaults(
        defineProps<{
            id: string
            label?: string
        }>(),
        {
            label: 'revision',
        },
    )

    type CopyState = 'idle' | 'copying' | 'copied' | 'failed'

    const copyState = ref<CopyState>('idle')

    let activeAttempt = 0
    let disposed = false

    watch(
        () => props.id,
        () => {
            activeAttempt += 1
            copyState.value = 'idle'
        },
    )

    onUnmounted(() => {
        disposed = true
        activeAttempt += 1
    })

    async function copyFullId(): Promise<void> {
        if (copyState.value === 'copying') {
            return
        }

        const fullId = props.id
        const attempt = ++activeAttempt

        copyState.value = 'copying'

        try {
            if (
                typeof navigator === 'undefined' ||
                typeof navigator.clipboard?.writeText !== 'function'
            ) {
                throw new Error('Clipboard API unavailable')
            }

            await navigator.clipboard.writeText(fullId)

            if (!disposed && activeAttempt === attempt && props.id === fullId) {
                copyState.value = 'copied'
            }
        } catch {
            if (!disposed && activeAttempt === attempt && props.id === fullId) {
                copyState.value = 'failed'
            }
        }
    }
</script>

<template>
    <span class="revision-id">
        <code class="revision-id__short" :title="props.id">
            {{ props.id.slice(0, 8) }}
        </code>

        <button
            class="revision-id__copy"
            type="button"
            :disabled="copyState === 'copying'"
            :aria-label="`Copy full ${props.label} ID ${props.id.slice(0, 8)}`"
            @click="copyFullId"
        >
            {{ copyState === 'copying' ? 'Copying…' : 'Copy ID' }}
        </button>

        <span v-if="copyState === 'copied'" class="revision-id__status" role="status">
            Copied
        </span>

        <span v-else-if="copyState === 'failed'" class="revision-id__fallback" role="status">
            Clipboard unavailable. Select full ID:
            <code>{{ props.id }}</code>
        </span>
    </span>
</template>

<style scoped>
    .revision-id {
        display: inline-flex;
        align-items: center;
        flex-wrap: wrap;
        gap: var(--space-2);
        min-width: 0;
    }

    .revision-id__short,
    .revision-id__fallback code {
        font-family: var(--font-family-mono);
        overflow-wrap: anywhere;
    }

    .revision-id__copy {
        min-height: 1.75rem;
        padding: 0 var(--space-2);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-sm);
        color: var(--color-action);
        background: var(--color-surface);
        font-size: var(--font-size-sm);
        cursor: pointer;
    }

    .revision-id__copy:hover:not(:disabled) {
        background: var(--color-page);
    }

    .revision-id__copy:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
    }

    .revision-id__copy:disabled {
        cursor: not-allowed;
        opacity: 0.6;
    }

    .revision-id__status,
    .revision-id__fallback {
        font-size: var(--font-size-sm);
        color: var(--color-text-muted);
    }

    .revision-id__fallback {
        flex-basis: 100%;
        min-width: 0;
        overflow-wrap: anywhere;
    }
</style>
