<script setup lang="ts">
import type { Crumb } from '@/app/navigation'

// The one page title: the trail of pages above it, the page's h1, a line under it or richer
// details, and its actions. The trail names only the pages above: the title says where this is.
defineProps<{ title: string; subtitle?: string; trail?: Crumb[] }>()
</script>

<template>
  <nav v-if="trail?.length" aria-label="Breadcrumb" class="trail mb-1 text-sm">
    <ol class="flex flex-wrap items-center">
      <li v-for="c in trail" :key="c.label">
        <RouterLink :to="c.to" class="link-target trail__link">{{ c.label }}</RouterLink>
      </li>
    </ol>
  </nav>
  <div class="mb-5 flex flex-wrap items-end justify-between gap-3">
    <div class="min-w-0">
      <h1 class="text-2xl">{{ title }}</h1>
      <p v-if="subtitle" class="prose-text mt-1 text-ink-muted">{{ subtitle }}</p>
      <slot name="details" />
    </div>
    <div v-if="$slots.default" class="flex flex-wrap gap-2">
      <slot />
    </div>
  </div>
</template>

<style scoped>
/* The separator is drawn, not read: its alternative text is empty. */
.trail li + li::before {
  content: '›' / '';
  margin-inline: 0.5rem;
  color: var(--color-ink-muted);
}

/* A short name ("Library") still makes a target of the full size. */
.trail__link {
  min-inline-size: var(--spacing-target);
  text-align: center;
  overflow-wrap: anywhere;
}
</style>
