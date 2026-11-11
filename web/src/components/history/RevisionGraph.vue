<script setup lang="ts">
    import { computed } from 'vue'

    import {
        getRevisionAncestry,
        type RevisionGraph,
        type RevisionGraphRevision,
    } from '../../history/revisionGraph'
    import {
        layoutRevisionGraph,
        type RevisionGraphLayoutEdge,
        type RevisionGraphLayoutNode,
        type RevisionGraphLayoutUnresolvedEdge,
    } from '../../history/revisionGraphLayout'

    const props = withDefaults(
        defineProps<{
            graph: RevisionGraph
            selectedRevisionId?: string | null
        }>(),
        {
            selectedRevisionId: null,
        },
    )

    const emit = defineEmits<{
        'select-revision': [revisionId: string]
    }>()

    // Geometry belongs to the renderer, not the graph model.
    // Layout positions remain independent of pixels and CSS.
    const ROW_HEIGHT = 88
    const LANE_STEP = 32
    const NODE_START_X = 24

    type PositionedRevision = RevisionGraphLayoutNode & {
        readonly revision: RevisionGraphRevision
        readonly headBranchIds: readonly string[]
    }

    const layout = computed(() => layoutRevisionGraph(props.graph))

    const positionedRevisions = computed<PositionedRevision[]>(() => {
        const byId = new Map(props.graph.nodes.map((node) => [node.revision.id, node]))

        return layout.value.nodes.map((position) => {
            const graphNode = byId.get(position.revisionId)

            if (graphNode === undefined) {
                throw new Error('Revision graph renderer is missing a revision')
            }

            return {
                ...position,
                revision: graphNode.revision,
                headBranchIds: graphNode.headBranchIds,
            }
        })
    })

    const branchNames = computed(
        () => new Map(props.graph.branches.map((branch) => [branch.id, branch.name])),
    )

    const ancestryIds = computed(
        () => new Set(getRevisionAncestry(props.graph, props.selectedRevisionId)),
    )

    const diagramWidth = computed(() => 48 + layout.value.laneCount * LANE_STEP)

    const diagramHeight = computed(() => layout.value.nodes.length * ROW_HEIGHT)

    const plotStyles = computed(() => ({
        '--revision-graph-rail-width': `${diagramWidth.value}px`,
        '--revision-graph-row-height': `${ROW_HEIGHT}px`,
    }))

    const coverageLabel = computed(() =>
        props.graph.coverage.source === 'project-wide'
            ? 'Project-wide history'
            : 'Branch-head-reachable history',
    )

    function headNames(branchIds: readonly string[]): string {
        return branchIds.map((id) => branchNames.value.get(id) ?? id).join(', ')
    }

    function displayDate(createdAt: string): string {
        return new Date(createdAt).toISOString().slice(0, 10)
    }

    function nodeX(lane: number): number {
        return NODE_START_X + lane * LANE_STEP
    }

    function nodeY(row: number): number {
        return row * ROW_HEIGHT + ROW_HEIGHT / 2
    }

    function edgePath(edge: RevisionGraphLayoutEdge): string {
        const x1 = nodeX(edge.childLane)
        const y1 = nodeY(edge.childRow)
        const x2 = nodeX(edge.parentLane)
        const y2 = nodeY(edge.parentRow)

        if (x1 === x2) {
            return `M ${x1} ${y1} L ${x2} ${y2}`
        }

        const middleY = (y1 + y2) / 2

        return [`M ${x1} ${y1}`, `C ${x1} ${middleY},`, `${x2} ${middleY},`, `${x2} ${y2}`].join(
            ' ',
        )
    }

    function unresolvedEdgePath(edge: RevisionGraphLayoutUnresolvedEdge): string {
        const x = nodeX(edge.childLane)
        const y = nodeY(edge.childRow)

        // A short, open-ended mark indicates missing ancestry.
        // Never draw a fabricated parent revision.
        return `M ${x} ${y} L ${x} ${y + ROW_HEIGHT / 3}`
    }

    function isSelected(revisionId: string): boolean {
        return props.selectedRevisionId === revisionId
    }

    function isAncestor(revisionId: string): boolean {
        return ancestryIds.value.has(revisionId) && !isSelected(revisionId)
    }

    function isHighlightedEdge(edge: RevisionGraphLayoutEdge): boolean {
        return (
            ancestryIds.value.has(edge.childRevisionId) &&
            ancestryIds.value.has(edge.parentRevisionId)
        )
    }

    function selectRevision(revisionId: string): void {
        emit('select-revision', revisionId)
    }
</script>

<template>
    <section class="revision-graph" aria-label="Visual revision history">
        <p class="revision-graph__coverage">
            {{ coverageLabel }}
        </p>

        <p
            v-if="props.graph.coverage.status === 'partial'"
            class="revision-graph__partial"
            role="status"
        >
            History is incomplete. Some branch histories or revision ancestors could not be loaded.
        </p>

        <p v-if="positionedRevisions.length === 0" class="revision-graph__empty" role="status">
            No revisions are available in the loaded history.
        </p>

        <div v-else class="revision-graph__scroll">
            <div class="revision-graph__plot" :style="plotStyles">
                <!--
                    The SVG is decorative. All revision interaction
                    and metadata remain available through native
                    HTML buttons in the adjacent list.
                -->
                <svg
                    class="revision-graph__diagram"
                    xmlns="http://www.w3.org/2000/svg"
                    :viewBox="`0 0 ${diagramWidth} ${diagramHeight}`"
                    :width="diagramWidth"
                    :height="diagramHeight"
                    aria-hidden="true"
                    focusable="false"
                >
                    <path
                        v-for="edge in layout.edges"
                        :key="`${edge.childRevisionId}:${edge.kind}:${edge.parentRevisionId}`"
                        class="revision-graph__edge"
                        :class="{
                            'revision-graph__edge--merge': edge.kind === 'merge-parent',
                            'revision-graph__edge--highlight': isHighlightedEdge(edge),
                        }"
                        :data-kind="edge.kind"
                        :data-child-id="edge.childRevisionId"
                        :data-parent-id="edge.parentRevisionId"
                        :d="edgePath(edge)"
                    />

                    <path
                        v-for="edge in layout.unresolvedEdges"
                        :key="`unresolved:${edge.childRevisionId}:${edge.kind}:${edge.parentRevisionId}`"
                        class="revision-graph__edge revision-graph__edge--unresolved"
                        :data-kind="edge.kind"
                        :data-child-id="edge.childRevisionId"
                        :data-parent-id="edge.parentRevisionId"
                        data-unresolved="true"
                        :d="unresolvedEdgePath(edge)"
                    />

                    <circle
                        v-for="node in positionedRevisions"
                        :key="node.revisionId"
                        class="revision-graph__node"
                        :class="{
                            'revision-graph__node--selected': isSelected(node.revisionId),
                            'revision-graph__node--ancestor': isAncestor(node.revisionId),
                        }"
                        :data-revision-id="node.revisionId"
                        :cx="nodeX(node.lane)"
                        :cy="nodeY(node.row)"
                        r="6"
                    />
                </svg>

                <ol class="revision-graph__revision-list">
                    <li
                        v-for="node in positionedRevisions"
                        :key="node.revisionId"
                        class="revision-graph__revision-item"
                    >
                        <button
                            type="button"
                            class="revision-graph__revision"
                            :class="{
                                'revision-graph__revision--selected': isSelected(node.revisionId),
                                'revision-graph__revision--ancestor': isAncestor(node.revisionId),
                            }"
                            :data-revision-id="node.revisionId"
                            :data-lane="node.lane"
                            :aria-pressed="isSelected(node.revisionId)"
                            :aria-label="`Select revision ${node.revisionId}: ${node.revision.message}`"
                            @click="selectRevision(node.revisionId)"
                        >
                            <span class="revision-graph__message">
                                {{ node.revision.message }}
                            </span>

                            <span class="revision-graph__metadata">
                                <span>
                                    {{
                                        node.revision.authorDisplayName ??
                                        node.revision.authorUserId
                                    }}
                                </span>

                                <time :datetime="node.revision.createdAt">
                                    {{ displayDate(node.revision.createdAt) }}
                                </time>
                            </span>

                            <span v-if="node.headBranchIds.length > 0" class="revision-graph__head">
                                Head: {{ headNames(node.headBranchIds) }}
                            </span>
                        </button>
                    </li>
                </ol>
            </div>
        </div>
    </section>
</template>

<style scoped>
    .revision-graph {
        min-width: 0;
    }

    .revision-graph__coverage {
        margin: 0 0 var(--space-3);
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }

    .revision-graph__partial {
        margin: 0 0 var(--space-3);
        padding: var(--space-3);
        border-left: 3px solid var(--color-danger);
        background: var(--color-surface);
        color: var(--color-text);
        font-size: var(--font-size-sm);
    }

    .revision-graph__empty {
        margin: 0;
        padding: var(--space-4);
        border: 1px solid var(--color-border);
        border-radius: var(--radius-md);
        color: var(--color-text-muted);
    }

    .revision-graph__scroll {
        min-width: 0;
        overflow-x: auto;
    }

    .revision-graph__plot {
        position: relative;
        display: grid;
        grid-template-columns: var(--revision-graph-rail-width) minmax(0, 1fr);
        min-width: calc(var(--revision-graph-rail-width) + 14rem);
    }

    .revision-graph__diagram {
        position: absolute;
        top: 0;
        left: 0;
        pointer-events: none;
    }

    .revision-graph__edge {
        fill: none;
        stroke: var(--color-text-muted);
        stroke-width: 2;
        stroke-linecap: round;
    }

    .revision-graph__edge--merge {
        stroke-dasharray: 5 4;
    }

    .revision-graph__edge--unresolved {
        stroke: var(--color-danger);
        stroke-dasharray: 3 4;
    }

    .revision-graph__edge--highlight {
        stroke: var(--color-action);
    }

    .revision-graph__node {
        fill: var(--color-page);
        stroke: var(--color-text-muted);
        stroke-width: 2;
    }

    .revision-graph__node--ancestor {
        fill: var(--color-surface);
        stroke: var(--color-action);
    }

    .revision-graph__node--selected {
        fill: var(--color-action);
        stroke: var(--color-action);
    }

    .revision-graph__revision-list {
        display: grid;
        grid-column: 2;
        grid-row: 1;
        grid-auto-rows: var(--revision-graph-row-height);
        min-width: 0;
        margin: 0;
        padding: 0;
        list-style: none;
    }

    .revision-graph__revision-item {
        display: flex;
        align-items: center;
        min-width: 0;
        padding: var(--space-1) var(--space-2);
    }

    .revision-graph__revision {
        box-sizing: border-box;
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        justify-content: center;
        gap: var(--space-1);
        width: 100%;
        min-width: 0;
        height: calc(var(--revision-graph-row-height) - var(--space-2));
        padding: var(--space-2) var(--space-3);
        overflow: hidden;
        border: 1px solid var(--color-border);
        border-radius: var(--radius-sm);
        background: var(--color-page);
        color: var(--color-text);
        text-align: left;
        cursor: pointer;
    }

    .revision-graph__revision:hover {
        background: var(--color-surface);
    }

    .revision-graph__revision:focus-visible {
        outline: 3px solid var(--color-focus-ring);
        outline-offset: 2px;
    }

    .revision-graph__revision--selected {
        border-color: var(--color-action);
        background: var(--color-surface);
    }

    .revision-graph__revision--ancestor {
        border-left: 3px solid var(--color-action);
    }

    .revision-graph__message {
        display: block;
        width: 100%;
        overflow: hidden;
        font-size: var(--font-size-sm);
        font-weight: var(--font-weight-semibold);
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .revision-graph__metadata {
        display: flex;
        flex-wrap: wrap;
        gap: var(--space-2);
        color: var(--color-text-muted);
        font-size: var(--font-size-sm);
    }

    .revision-graph__head {
        max-width: 100%;
        overflow: hidden;
        color: var(--color-action);
        font-size: var(--font-size-sm);
        text-overflow: ellipsis;
        white-space: nowrap;
    }
</style>
