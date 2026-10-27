<script setup lang="ts">
    import CopyableId from './CopyableId.vue'

    const props = withDefaults(
        defineProps<{
            id: string
            label?: string
        }>(),
        {
            label: 'revision',
        },
    )
</script>

<template>
    <CopyableId :id="props.id" :label="props.label" kind="revision" />
</template>
