<script setup lang="ts">
import { useId } from 'vue'

// The one box. A section with a heading, or, as `fieldset`, a group of a form's fields with
// its legend, which a screen reader announces with each field in it.
defineProps<{ title?: string; fieldset?: boolean }>()

const headingId = useId()
</script>

<template>
  <fieldset v-if="fieldset" class="panel">
    <legend class="panel__legend">{{ title }}</legend>
    <slot />
  </fieldset>
  <section v-else class="panel" :aria-labelledby="title ? headingId : undefined">
    <div
      v-if="title || $slots.actions"
      class="mb-3 flex flex-wrap items-center justify-between gap-2"
    >
      <h2 v-if="title" :id="headingId" class="text-lg">{{ title }}</h2>
      <div v-if="$slots.actions" class="flex flex-wrap gap-2"><slot name="actions" /></div>
    </div>
    <slot />
  </section>
</template>

<!-- In the components layer, so a page's margin utilities on a panel win over its reset. -->
<style scoped>
@layer components {
  .panel {
    min-width: 0;
    margin: 0;
    padding: 1rem;
    border: 1px solid var(--color-hairline);
    border-radius: var(--radius-control);
    background: var(--color-surface-panel);
  }

  .panel__legend {
    padding: 0 0.35rem;
    font-weight: 650;
    font-size: 1.125rem;
  }
}
</style>
