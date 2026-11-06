<script setup lang="ts">
import { computed, nextTick, onMounted, ref, useId, watch } from 'vue'
import AutoComplete from 'primevue/autocomplete'
import Button from 'primevue/button'
import Select from 'primevue/select'
import { faBarcodeRead } from '@fortawesome/pro-regular-svg-icons'
import { api, type Book, type BookRow } from '@/api/client'
import { scanNote, type ScanNote } from '@/app/barcodes'
import { matching, useLibrary } from '@/app/library'
import { messageOf, useNotice } from '@/app/problem'
import { copyLabel } from '@/app/format'
import BarcodeScanner from './BarcodeScanner.vue'
import BookCover from './BookCover.vue'
import FormField from './FormField.vue'

// The one way to choose a book from the library, and then which of its copies: the search
// is the library list's own, and a scan of a copy's label or the book's ISBN chooses too.
// Choosing the book itself, as a share link does, says each one chosen and empties the box
// for the next.
const props = defineProps<{
  /** Only copies on the shelf can be chosen, as for a loan. */
  onShelf?: boolean
  /** The book itself is chosen, a wanted one among them, and no copy of it. */
  whole?: boolean
  label?: string
  hint?: string
}>()
const emit = defineEmits<{ chosen: [book: BookRow] }>()
const copyId = defineModel<number>({ default: 0 })
const library = useLibrary()
const notice = useNotice()
const id = useId()

onMounted(() => {
  if (!library.loaded) void library.load()
})

// The search box holds the words while they are typed, and the book once one is chosen.
const picked = ref<BookRow | string | null>(null)
const matches = ref<BookRow[]>([])
function search(query: string) {
  // Books in the house: a wanted one has no copy to lend or put on a course. A whole book may
  // be wanted too, but not one whose copies have all gone.
  const among = props.whole ? library.rows.filter((r) => r.copies > 0 || r.wantedOn) : library.books
  matches.value = among.filter(matching(query)).slice(0, 8)
}

const book = ref<Book | null>(null)
// The copies in the house: one that has gone can be neither lent nor put on a course.
const copies = computed(() =>
  (book.value?.copies ?? [])
    .filter((c) => !c.goneAs)
    .map((c) => ({
      value: c.id,
      label: copyLabel(c),
      disabled: !!props.onShelf && c.onLoan,
    })),
)

// The copy a scanned label named, chosen once its book has loaded.
const wanted = ref(0)

watch(picked, async (v) => {
  copyId.value = 0
  book.value = null
  if (!v || typeof v === 'string') return
  if (props.whole) {
    emit('chosen', v)
    // Emptied once the box has taken the choice: emptied at once, it never sees a change.
    void nextTick(() => (picked.value = null))
    return
  }
  const { data, error } = await api.GET('/books/{id}', { params: { path: { id: v.id } } })
  if (error) return notice.failed(error)
  book.value = data
  const free = copies.value.filter((c) => !c.disabled)
  const scannedCopy = free.find((c) => c.value === wanted.value)
  if (scannedCopy) copyId.value = scannedCopy.value
  else if (free.length === 1 && free[0]) copyId.value = free[0].value
})

const note = computed(() => {
  if (!book.value) return ''
  if (!copies.value.length) return 'This book has no copy on record. Add one on its page first.'
  if (copies.value.every((c) => c.disabled)) return 'Every copy of this book is out on loan.'
  const scannedCopy = copies.value.find((c) => c.value === wanted.value)
  if (scannedCopy?.disabled) return `The copy you scanned is out: ${scannedCopy.label}.`
  return ''
})

const scanning = ref(false)
const said = ref<ScanNote | null>(null)
async function scanned(code: string) {
  const { data, error } = await api.GET('/codes/{code}', { params: { path: { code } } })
  if (error) return (said.value = scanNote(messageOf(error), 'bad'))
  if (!data.bookId) {
    const why = data.message || 'That book is not in your library.'
    return (said.value = scanNote(why, 'bad'))
  }
  if (!library.loaded) await library.load()
  const row = library.rows.find((r) => r.id === data.bookId)
  if (!row) return (said.value = scanNote('That book is not in your library.', 'bad'))
  wanted.value = data.kind === 'copy' ? data.copyId : 0
  picked.value = row
  scanning.value = false
  said.value = null
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <FormField
      :label="label ?? 'Book'"
      :for="`${id}-book`"
      :hint="hint ?? 'Type a title, an author, a series, an ISBN, or a barcode.'"
      :required="!whole"
    >
      <AutoComplete
        v-model="picked"
        :input-id="`${id}-book`"
        :suggestions="matches"
        option-label="title"
        force-selection
        fluid
        @complete="search($event.query)"
        @update:model-value="wanted = 0"
      >
        <template #option="{ option }">
          <div class="flex items-center gap-3">
            <BookCover
              :cover="(option as BookRow).cover"
              :title="(option as BookRow).title"
              :width="48"
            />
            <div class="min-w-0">
              <div>{{ (option as BookRow).title }}</div>
              <div class="text-sm text-ink-muted">{{ (option as BookRow).authors }}</div>
            </div>
          </div>
        </template>
      </AutoComplete>
    </FormField>
    <div>
      <Button type="button" severity="secondary" @click="scanning = !scanning"
        ><FontAwesomeIcon :icon="faBarcodeRead" />
        {{ scanning ? 'Stop scanning' : 'Scan its label or ISBN' }}</Button
      >
    </div>
    <BarcodeScanner v-if="scanning" :note="said" @code="scanned" />
    <FormField v-if="copies.length > 1" label="Copy" :for="`${id}-copy`" required>
      <Select
        v-model="copyId"
        :input-id="`${id}-copy`"
        :aria-labelledby="`${id}-copy-label`"
        :options="copies"
        option-label="label"
        option-value="value"
        option-disabled="disabled"
        placeholder="Choose a copy"
      />
    </FormField>
    <p v-if="note" class="text-sm" role="status">{{ note }}</p>
  </div>
</template>
