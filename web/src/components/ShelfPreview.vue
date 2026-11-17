<script setup lang="ts">
import { computed, onMounted, ref, useId, useTemplateRef, watch } from 'vue'
import Button from 'primevue/button'
import { api } from '@/api/client'
import type { BookRow, Layout, ShelfBook, ShelfPlan } from '@/types'
import { count, day } from '@/app/format'
import { messageOf } from '@/app/problem'
import { pxPerMm } from '@/app/stacks'
import { spineColors } from '@/app/theme/spine'
import { useFeed } from '@/composables/useFeed'
import { useWidth } from '@/composables/useWidth'
import BookcaseFrame from './BookcaseFrame.vue'

// The owner's own bookcases as a layout fills them, drawn to scale: the layout in use, or a
// fresh suggestion by the areas' rules, to see before putting one to use on Shelves. A picture
// to look at, each shelf told in words: its books are pulled off the shelves of the Stacks.
const props = defineProps<{ books: ReadonlyMap<number, BookRow> }>()
const id = useId()

// Which layout is drawn; a fresh suggestion where none is in use.
const which = ref<'in-use' | 'suggested'>('in-use')
const inUse = ref(false)
const layout = ref<Layout | null>(null)
const failed = ref('')
async function load() {
  const { data, error } = await api.GET('/layout', {
    params: { query: { suggest: which.value === 'suggested' } },
  })
  if (error) {
    failed.value = messageOf(error)
    return
  }
  failed.value = ''
  if (which.value === 'in-use') {
    inUse.value = data.inUse
    if (!data.inUse) {
      which.value = 'suggested'
      return
    }
  }
  layout.value = data
}
watch(which, () => void load())
onMounted(load)
useFeed(['book'], load)

// One scale for every bookcase, the widest shelf filling the page, and never larger than the
// Stacks draw their books.
const width = useWidth(useTemplateRef<HTMLElement>('inside'))
const frame = 40 // the bookcase's sides and the room inside them, in pixels, near enough
const scale = computed(() => {
  const widest = Math.max(1, ...(layout.value?.shelves ?? []).map((s) => s.widthMm))
  return Math.min(pxPerMm, Math.max(0.1, (width.value - frame) / widest))
})
const px = (mm: number) => `${String(Math.round(mm * scale.value))}px`

// A shelf in words: where it is, what area, how many books and how full, from what to what.
function told(s: ShelfPlan): string {
  const books = s.books ?? []
  const parts = [`Shelf ${String(s.position)}`, s.area || 'in no area', count(books.length, 'book')]
  if (books.length) {
    parts.push(`${String(Math.round((100 * s.usedMm) / s.widthMm))}% full`)
    parts.push(s.first === s.last ? s.first : `${s.first} to ${s.last}`)
  }
  if (s.overMm > 0) {
    parts.push('over full')
  }
  return parts.join(', ')
}

// The bookcases in walking order, each with its shelves from the top.
const cases = computed(() => {
  const out: {
    id: number
    name: string
    shelves: (ShelfPlan & { key: number; edge: string })[]
  }[] = []
  for (const s of layout.value?.shelves ?? []) {
    const shelf = { ...s, key: s.id, edge: told(s) }
    const last = out.at(-1)
    if (last?.id === s.bookcaseId) {
      last.shelves.push(shelf)
    } else {
      out.push({ id: s.bookcaseId, name: s.bookcase, shelves: [shelf] })
    }
  }
  return out
})

function spine(b: ShelfBook) {
  const row = props.books.get(b.bookId)
  const c = spineColors(row?.cover.color ?? '', row?.category ?? '')
  return {
    '--spine-back': c.back,
    '--spine-ink': c.ink,
    '--spine-width': px(b.thicknessMm),
    '--spine-height': px(b.heightMm),
  }
}

// The books the layout has no shelf for, which Shelves lists.
const left = computed(() => {
  const l = layout.value
  return [
    l?.noArea?.length ? `${count(l.noArea.length, 'book')} in no area` : '',
    l?.unplaced?.length ? `${count(l.unplaced.length, 'book')} with no room` : '',
    l?.tooTall?.length ? `${count(l.tooTall.length, 'book')} too tall for their place` : '',
  ]
    .filter(Boolean)
    .join(', ')
})
</script>

<template>
  <div ref="inside">
    <div v-if="inUse" class="mb-4 flex flex-wrap gap-2" role="group" aria-label="Layout">
      <Button
        v-for="w in [
          { value: 'in-use', label: 'The layout in use' },
          { value: 'suggested', label: 'A fresh suggestion' },
        ] as const"
        :key="w.value"
        type="button"
        :severity="which === w.value ? undefined : 'secondary'"
        :text="which !== w.value"
        :aria-pressed="which === w.value"
        @click="which = w.value"
        >{{ w.label }}</Button
      >
    </div>
    <div v-if="failed" class="mb-4 flex flex-wrap items-center gap-3" role="alert">
      <p class="prose-text font-semibold text-status-danger">
        Couldn't load your bookcases: {{ failed }}
      </p>
      <Button type="button" severity="secondary" label="Try again" @click="load()" />
    </div>
    <p v-else-if="!layout" class="text-ink-muted" role="status">Laying out your bookcases…</p>
    <p v-else-if="!cases.length" class="prose-text">
      No bookcases yet. Measure yours on
      <RouterLink to="/shelves" class="underline">Shelves</RouterLink>, and they show here.
    </p>
    <template v-else>
      <p class="prose-text mb-4">
        <template v-if="layout.inUse">The layout put to use {{ day(layout.appliedAt) }}.</template>
        <template v-else-if="inUse">
          What your areas’ rules would do now. Nothing is put to use from here.
        </template>
        <template v-else>
          No layout is in use: this is what your areas’ rules would do now. Nothing is put to use
          from here.
        </template>
        <template v-if="left">
          {{ left }}: see them on <RouterLink to="/shelves" class="underline">Shelves</RouterLink>.
        </template>
      </p>
      <section
        v-for="c in cases"
        :key="c.id"
        :aria-labelledby="`${id}-${String(c.id)}`"
        class="mb-8"
      >
        <h2 :id="`${id}-${String(c.id)}`" class="mb-2 text-xl">{{ c.name }}</h2>
        <BookcaseFrame :shelves="c.shelves" fit>
          <template #shelf="{ shelf }">
            <div
              class="shelf"
              role="img"
              :aria-label="shelf.edge"
              :style="{ '--shelf-width': px(shelf.widthMm), '--shelf-height': px(shelf.heightMm) }"
            >
              <template v-if="shelf.braceHeightMm">
                <span
                  v-for="end in ['start', 'end']"
                  :key="end"
                  class="shelf__brace"
                  :class="`shelf__brace--${end}`"
                  :style="{
                    '--brace-width': px(shelf.braceWidthMm),
                    '--brace-height': px(shelf.braceHeightMm),
                  }"
                />
              </template>
              <span v-if="shelf.gapMm" class="shelf__gap" :style="{ '--gap': px(shelf.gapMm) }" />
              <span
                v-for="b in shelf.books ?? []"
                :key="b.copyId"
                class="shelf__book"
                :style="spine(b)"
              >
                <span v-if="b.thicknessMm * scale >= 20" class="shelf__title">{{ b.title }}</span>
              </span>
            </div>
          </template>
        </BookcaseFrame>
      </section>
    </template>
  </div>
</template>

<style scoped>
/* A shelf to scale: its books stand along it from the left, and what runs past its end is
   hidden by the bookcase's side, as its edge says. */
.shelf {
  position: relative;
  display: flex;
  align-items: flex-end;
  gap: 1px;
  width: var(--shelf-width);
  height: var(--shelf-height);
  margin-top: 0.5rem;
  overflow: hidden;
}

/* The corner braces under the shelf above, at both ends. */
.shelf__brace {
  position: absolute;
  top: 0;
  width: var(--brace-width);
  height: var(--brace-height);
  background: var(--color-wood);
}

.shelf__brace--start {
  left: 0;
}

.shelf__brace--end {
  right: 0;
}

/* Room left empty at the start, where a brace is too low for the first book. */
.shelf__gap {
  flex: none;
  width: var(--gap);
}

.shelf__book {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: var(--spine-width);
  height: var(--spine-height);
  overflow: hidden;
  border-radius: 1px 1px 0 0;
  background: var(--spine-back);
  color: var(--spine-ink);
  box-shadow: inset -1px 0 var(--color-spine-shade);
}

.shelf__title {
  max-height: 100%;
  overflow: hidden;
  font-size: 0.75rem;
  font-weight: 650;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
  writing-mode: vertical-rl;
}
</style>
