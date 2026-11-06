<script setup lang="ts">
import { reactive } from 'vue'
import { faCheck } from '@fortawesome/pro-regular-svg-icons'
import type { CoverChoice } from '@/api/client'
import { sourceLabel } from '@/app/format'
import BookCover from './BookCover.vue'

// The one way to choose among covers: the sources' covers, each named by its source and its
// size, the owner's photo when there is one, and, when allowed, none. The value is the chosen
// link, 'photo', or '' for none.
const props = defineProps<{
  choices: CoverChoice[]
  photo?: string
  none?: boolean
  title: string
}>()
const chosen = defineModel<string>({ required: true })

// Each picture's size, as the browser found it on loading; a source does not say.
const sizes = reactive(new Map<string, { width: number; height: number } | 'failed'>())

interface Tile {
  value: string
  url?: string
  name: string
}

const tiles = (): Tile[] => [
  ...props.choices.map((c) => ({ value: c.url, url: c.url, name: sourceLabel(c.source) })),
  ...(props.photo ? [{ value: 'photo', url: props.photo, name: 'Your photo' }] : []),
  ...(props.none ? [{ value: '', name: 'No cover' }] : []),
]

function sizeText(t: Tile): string {
  const size = t.url ? sizes.get(t.url) : undefined
  if (size === 'failed') return 'Not shown'
  return size ? `${size.width} × ${size.height}` : ''
}

function nameOf(t: Tile): string {
  const size = t.url ? sizes.get(t.url) : undefined
  if (size === 'failed') return `${t.name}, which could not be shown`
  return size ? `${t.name}, ${size.width} by ${size.height} pixels` : t.name
}
</script>

<template>
  <div class="flex flex-wrap gap-3" role="radiogroup" aria-label="Cover">
    <button
      v-for="t in tiles()"
      :key="t.value || 'none'"
      type="button"
      role="radio"
      :aria-checked="chosen === t.value"
      :aria-label="nameOf(t)"
      class="choice"
      :class="{ 'choice--on': chosen === t.value }"
      @click="chosen = t.value"
    >
      <BookCover
        :url="t.url"
        :title="title"
        :width="96"
        @measured="(width, height) => t.url && sizes.set(t.url, { width, height })"
        @failed="() => t.url && sizes.set(t.url, 'failed')"
      />
      <span class="choice__text text-sm">
        <span>{{ t.value ? t.name : 'None' }}</span>
        <span v-if="t.url" class="text-ink-muted">{{ sizeText(t) }}</span>
      </span>
      <FontAwesomeIcon v-if="chosen === t.value" :icon="faCheck" class="choice__mark" />
    </button>
  </div>
</template>

<style scoped>
.choice {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.25rem;
  width: 7.5rem;
  padding: 0.4rem;
  border: 2px solid var(--color-outline);
  border-radius: var(--radius-control);
  background: transparent;
  color: inherit;
  font: inherit;
  cursor: pointer;
}

.choice--on {
  border-color: var(--color-accent);
  background: var(--color-highlight);
}

/* Two lines kept for every tile, so a size arriving does not move the row. */
.choice__text {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-height: 2lh;
  text-align: center;
}

.choice__mark {
  position: absolute;
  top: 0.3rem;
  right: 0.3rem;
  padding: 0.25rem;
  border-radius: 999px;
  background: var(--color-accent);
  color: var(--color-accent-contrast);
}
</style>
