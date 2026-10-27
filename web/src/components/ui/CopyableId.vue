<script setup lang="ts">
    import { onUnmounted, ref, watch } from 'vue'

    const props = defineProps<{
        id: string
        label: string
        kind: 'revision' | 'user'
    }>()

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
    <span class="copyable-id" :class="props.kind === 'revision' ? 'revision-id' : 'user-id'">
        <code
            class="copyable-id__short"
            :class="props.kind === 'revision' ? 'revision-id__short' : 'user-id__short'"
            :title="props.id"
        >
            {{ props.id.slice(0, 8) }}
        </code>

        <button
            class="copyable-id__copy"
            :class="props.kind === 'revision' ? 'revision-id__copy' : 'user-id__copy'"
            type="button"
            :disabled="copyState === 'copying'"
            :aria-label="`Copy full ${props.label} ID ${props.id.slice(0, 8)}`"
            @click="copyFullId"
        >
            {{ copyState === 'copying' ? 'Copying…' : 'Copy ID' }}
        </button>

        <span
            v-if="copyState === 'copied'"
            class="copyable-id__status"
            :class="props.kind === 'revision' ? 'revision-id__status' : 'user-id__status'"
            role="status"
        >
            Copied
        </span>

        <span
            v-else-if="copyState === 'failed'"
            class="copyable-id__fallback"
            :class="props.kind === 'revision' ? 'revision-id__fallback' : 'user-id__fallback'"
            role="status"
        >
            Clipboard unavailable. Select full ID:
            <code>{{ props.id }}</code>
        </span>
    </span>
</template>

<style scoped>
    .copyable-id {
        display: inline-flex;
        align-items: center;
        flex-wrap: wrap;
        gap: var(--space-2);
        min-width: 0;
    }

    .copyable-id__short,
    .copyable-id__fallback code {
        font-family: var(--font-family-mono);
        overflow-wrap: anywhere;
    }

    .copyable-id__copy {
        min-height: 1.75rem;
        padding: 0 var(--space-2);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-sm);
        color: var(--color-action);
        background: var(--color-surface);
        font-size: var(--font-size-sm);
        cursor: pointer;
    }

    .copyable-id__copy:hover:not(:disabled) {
        background: var(--color-page);
    }

    .copyable-id__copy:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
    }

    .copyable-id__copy:disabled {
        cursor: not-allowed;
        opacity: 0.6;
    }

    .copyable-id__status,
    .copyable-id__fallback {
        font-size: var(--font-size-sm);
        color: var(--color-text-muted);
    }

    .copyable-id__fallback {
        flex-basis: 100%;
        min-width: 0;
        overflow-wrap: anywhere;
    }
</style>
