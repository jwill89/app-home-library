<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { faBook } from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import type { Cover } from '@/api/client'
import { coverSrcset, coverURL } from '@/app/format'

// A stored cover (by id), or a source's link before one is stored, or the plain spine of a
// book with neither. Covers keep their own shape inside a 2:3 frame.
const props = defineProps<{
  cover?: Cover | null
  url?: string
  title: string
  /** The width it is shown at, in CSS pixels, which picks the rendition. */
  width: number
}>()

const emit = defineEmits<{
  /** The picture's own size in pixels, once it has loaded. */
  measured: [width: number, height: number]
  /** The picture could not be loaded. */
  failed: []
}>()

const stored = computed(() => (props.cover?.id ? props.cover : null))
const src = computed(() => props.url ?? (stored.value ? coverURL(stored.value, props.width) : ''))
const srcset = computed(() => (!props.url && stored.value ? coverSrcset(stored.value) : undefined))

// A picture that does not load shows as no cover, not as a broken image.
const broken = ref(false)
watch(src, () => (broken.value = false))

function loaded(event: Event) {
  const img = event.target as HTMLImageElement
  emit('measured', img.naturalWidth, img.naturalHeight)
}

function fail() {
  broken.value = true
  emit('failed')
}
</script>

<template>
  <div class="cover" :class="`cover--w${width}`">
    <img
      v-if="src && !broken"
      :src="src"
      :srcset="srcset"
      :sizes="`${width}px`"
      :alt="`Cover of ${title}`"
      loading="lazy"
      class="cover__img"
      @load="loaded"
      @error="fail"
    />
    <div v-else class="cover__blank" role="img" :aria-label="`No cover for ${title}`">
      <FontAwesomeIcon :icon="faBook" />
    </div>
  </div>
</template>

<style scoped>
.cover {
  flex: none;
  aspect-ratio: 2 / 3;
}

.cover--w48 {
  width: 3rem;
}

.cover--w96 {
  width: 6rem;
}

.cover--w240 {
  width: 15rem;
}

.cover__img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  object-position: bottom;
  border-radius: var(--radius-cover);
}

.cover__blank {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  border-radius: var(--radius-cover);
  background: var(--color-surface-page);
  border: 1px solid var(--color-hairline);
  color: var(--color-ink-muted);
}
</style>
