<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import type { BookRow } from '@/types'
import {
  callNumberText,
  categoryLabel,
  formatLabel,
  lengthIn,
  readStatusLabel,
  seriesLine,
} from '@/app/format'
import { useUnits } from '@/stores/units'
import BookCover from './BookCover.vue'

// A book pulled off a shelf of the Stacks: its cover, what it is, and the way to its page.
// Putting it back closes this and stands the book on its shelf again.
const props = defineProps<{ book: BookRow | null }>()
defineEmits<{ 'put-back': [] }>()
const open = defineModel<boolean>('visible', { required: true })
const { unit } = storeToRefs(useUnits())

const published = computed(() =>
  [props.book?.publisher, props.book?.published.slice(0, 4)].filter(Boolean).join(', '),
)
const size = computed(() => {
  const b = props.book
  if (!b) {
    return ''
  }
  const said = `${lengthIn(b.heightMm, unit.value)} tall, ${lengthIn(b.thicknessMm, unit.value)} thick`
  return b.sizeEstimated ? `About ${said}, from its format and pages` : said
})
</script>

<template>
  <Dialog
    v-model:visible="open"
    modal
    :header="book?.title ?? ''"
    class="w-full max-w-2xl"
    @after-hide="$emit('put-back')"
  >
    <div v-if="book" class="grid gap-4 sm:grid-cols-[15rem_minmax(0,1fr)]">
      <BookCover :cover="book.cover" :title="book.title" :width="240" class="justify-self-center" />
      <div class="min-w-0">
        <p v-if="book.subtitle" class="prose-text mb-3">{{ book.subtitle }}</p>
        <dl class="detail-list">
          <dt>By</dt>
          <dd>{{ book.authors || 'No author recorded' }}</dd>
          <template v-if="book.series">
            <dt>Series</dt>
            <dd>{{ seriesLine(book.series, book.seriesNumber) }}</dd>
          </template>
          <template v-if="book.callNumber?.length">
            <dt>Call number</dt>
            <dd class="font-mono">{{ callNumberText(book.callNumber) }}</dd>
          </template>
          <dt>Category</dt>
          <dd>{{ categoryLabel(book.category) }}</dd>
          <dt>Reading</dt>
          <dd>{{ readStatusLabel(book.readStatus) }}</dd>
          <template v-if="published">
            <dt>Published</dt>
            <dd>{{ published }}</dd>
          </template>
          <template v-if="book.format">
            <dt>Format</dt>
            <dd>{{ formatLabel(book.format) }}</dd>
          </template>
          <dt>Size</dt>
          <dd>{{ size }}</dd>
          <template v-if="book.copies > 1">
            <dt>Copies</dt>
            <dd>{{ book.copies }}</dd>
          </template>
        </dl>
      </div>
    </div>
    <template #footer>
      <Button type="button" severity="secondary" label="Put it back" @click="open = false" />
      <RouterLink v-if="book" v-slot="{ navigate }" :to="`/books/${String(book.id)}`" custom>
        <Button type="button" label="Open its page" @click="navigate" />
      </RouterLink>
    </template>
  </Dialog>
</template>
