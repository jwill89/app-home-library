<script setup lang="ts">
import { computed, useId, useTemplateRef } from 'vue'
import type { BookRow } from '@/types'
import { type Arrangement, shelfRange, shelvesOf, spineGap, spineWidth } from '@/app/stacks'
import { count } from '@/app/format'
import { useWidth } from '@/composables/useWidth'
import BookcaseFrame from './BookcaseFrame.vue'
import BookSpine from './BookSpine.vue'

// One bookcase of the Stacks: its books in order along shelves as wide as the page, each shelf
// a list of spines named for what it holds, which its edge says as a shelf's label would.
const props = defineProps<{
  title: string
  books: BookRow[]
  by: Arrangement
  /** The book off the shelf, if one is: 0 for none. */
  pulled: number
  found: ReadonlySet<number>
}>()
defineEmits<{ pull: [book: BookRow] }>()

const id = useId()
// A shelf is as wide as the room inside the case, which every shelf has: the first one's.
const width = useWidth(
  useTemplateRef<HTMLElement>('inside'),
  (el) => el.querySelector('ul')?.clientWidth ?? 0,
)

const shelves = computed(() =>
  shelvesOf(props.books, width.value || Infinity, spineWidth, spineGap).map((books, i) => {
    const edge = shelfRange(books, props.by)
    return { key: i, books, edge, name: `Shelf ${String(i + 1)}: ${edge}` }
  }),
)
</script>

<template>
  <section :aria-labelledby="`${id}-title`" class="mb-8">
    <div class="mb-2 flex flex-wrap items-baseline gap-x-3">
      <h2 :id="`${id}-title`" class="text-xl">{{ title }}</h2>
      <p class="text-ink-muted">{{ count(books.length, 'book') }}</p>
    </div>
    <div ref="inside">
      <BookcaseFrame :shelves="shelves">
        <template #shelf="{ shelf }">
          <ul class="books" :aria-label="shelf.name">
            <li v-for="b in shelf.books" :key="b.id">
              <BookSpine
                :id="`spine-${String(b.id)}`"
                :book="b"
                :pulled="b.id === pulled"
                :found="found.has(b.id)"
                @pull="$emit('pull', b)"
              />
            </li>
          </ul>
        </template>
      </BookcaseFrame>
    </div>
  </section>
</template>

<style scoped>
/* Room above the tallest book, as a real shelf leaves, for a book to be drawn up. */
.books {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  margin: 0;
  padding: 1.5rem 0 0;
  list-style: none;
}
</style>
