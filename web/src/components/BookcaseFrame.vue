<script setup lang="ts" generic="S extends { key: string | number; edge: string }">
// The one bookcase drawn on the screen: wooden sides and top, a back behind the books, and
// each shelf's edge under what stands on it, saying what it holds as a shelf's label would.
// What stands on a shelf is the slot's: the Stacks' spines, or a measured shelf to scale.
defineProps<{
  shelves: S[]
  /** As wide as its widest shelf, rather than the page. */
  fit?: boolean
}>()
defineSlots<{ shelf: (props: { shelf: S }) => unknown }>()
</script>

<template>
  <div class="bookcase" :class="{ 'bookcase--fit': fit }">
    <div v-for="s in shelves" :key="s.key">
      <slot name="shelf" :shelf="s" />
      <p class="bookcase__edge" aria-hidden="true">{{ s.edge }}</p>
    </div>
  </div>
</template>

<style scoped>
.bookcase {
  padding: 0 1rem;
  border-top: 0.75rem solid var(--color-wood);
  border-radius: var(--radius-control) var(--radius-control) 0 0;
  background: var(--color-shelf-back);
  box-shadow:
    inset 0.75rem 0 var(--color-wood),
    inset -0.75rem 0 var(--color-wood);
}

.bookcase--fit {
  width: fit-content;
  max-width: 100%;
}

/* The shelf itself, reaching the sides, its words in the case's own ink. */
.bookcase__edge {
  min-height: 1.75rem;
  margin: 0 -0.25rem;
  padding: 0.25rem 0.5rem;
  background: var(--color-wood);
  color: var(--color-wood-ink);
  font-size: 0.875rem;
  overflow-wrap: anywhere;
}
</style>
