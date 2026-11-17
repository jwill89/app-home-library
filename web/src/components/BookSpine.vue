<script setup lang="ts">
import { computed } from 'vue'
import type { BookRow } from '@/types'
import { seriesLine } from '@/app/format'
import { spineHeight, spineWidth, surname } from '@/app/stacks'
import { spineColors } from '@/app/theme/spine'

// The one spine: a book standing on a shelf of the Stacks, as tall and as thick as it is (never
// narrower than the target size), in its cover's colour, with its title and author along it as
// a real spine has them. It is the button that pulls the book off the shelf.
const props = defineProps<{
  book: BookRow
  /** Off the shelf, its details open. */
  pulled?: boolean
  /** One that a search on the shelves found. */
  found?: boolean
}>()
defineEmits<{ pull: [] }>()

const colors = computed(() => spineColors(props.book.cover.color, props.book.category))
const look = computed(() => ({
  '--spine-back': colors.value.back,
  '--spine-ink': colors.value.ink,
  '--spine-width': `${String(spineWidth(props.book))}px`,
  '--spine-height': `${String(spineHeight(props.book))}px`,
}))
const author = computed(() => surname(props.book))
// Its name says all the spine shows: the title, the series and its number, and the authors in
// full.
const name = computed(() => {
  const b = props.book
  return [
    b.title,
    b.seriesNumber && seriesLine(b.series, b.seriesNumber),
    b.authors && `by ${b.authors}`,
  ]
    .filter(Boolean)
    .join(', ')
})
</script>

<template>
  <button
    type="button"
    class="spine"
    :class="{ 'spine--pulled': pulled, 'spine--found': found }"
    :style="look"
    :aria-label="name"
    aria-haspopup="dialog"
    @click="$emit('pull')"
  >
    <span class="spine__words">
      <span class="spine__title">{{ book.title }}</span>
      <span v-if="author" class="spine__author">{{ author }}</span>
    </span>
    <span v-if="book.seriesNumber" class="spine__volume">{{ book.seriesNumber }}</span>
  </button>
</template>

<style scoped>
.spine {
  position: relative;
  display: block;
  width: var(--spine-width);
  height: var(--spine-height);
  padding: 0.375rem 0.125rem;
  border: 0;
  border-radius: 2px 2px 0 0;
  background: var(--spine-back);
  color: var(--spine-ink);

  /* The edges of a rounded spine: light on one side, shadow on the other. */
  box-shadow:
    inset 2px 0 var(--color-spine-lit),
    inset -2px 0 var(--color-spine-shade);
  cursor: pointer;
  font: inherit;
  transition: transform 0.2s ease-out;
}

/* Reaching for a book draws it up a little; pulling it off draws it most of the way out. */
.spine:hover,
.spine:focus-visible {
  transform: translateY(-0.5rem);
}

.spine--pulled,
.spine--pulled:hover,
.spine--pulled:focus-visible {
  z-index: 1;
  transform: translateY(-40%);
}

/* Where the device asks for less motion, a book is up or down at once. */
@media (prefers-reduced-motion: reduce) {
  .spine {
    transition: none;
  }
}

/* A book a search found has a mark above it, on the back of the bookcase. */
.spine--found::before {
  position: absolute;
  inset: -0.75rem 0.25rem auto;
  height: 0.375rem;
  border-radius: 2px;
  background: var(--color-accent);
  content: '';
}

/* Words along the spine, read from the top as an English spine is. */
.spine__words {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.125rem;
  width: 100%;
  height: 100%;
  overflow: hidden;
  writing-mode: vertical-rl;
  line-height: 1.25;
}

/* A volume's number across the foot of the spine, as a series' spines have it, to read
   along a run of them whose titles are alike. */
.spine:has(.spine__volume) {
  padding-bottom: 1.75rem;
}

.spine__volume {
  position: absolute;
  inset: auto 0.125rem 0.375rem;
  overflow: hidden;
  font-size: 0.75rem;
  font-weight: 650;
  line-height: 1.25;
  text-align: center;
  white-space: nowrap;
}

.spine__title,
.spine__author {
  max-height: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.spine__title {
  font-size: 0.875rem;
  font-weight: 650;
}

.spine__author {
  font-size: 0.75rem;
}
</style>
