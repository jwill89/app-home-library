<script setup lang="ts">
import { onMounted, onUnmounted, ref, useId } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import SelectButton from 'primevue/selectbutton'
import {
  faBarcode,
  faFileImport,
  faFileLines,
  faMagnifyingGlass,
} from '@fortawesome/pro-regular-svg-icons'
import {
  api,
  formData,
  type BookInput,
  type Candidate,
  type Choices,
  type CopyInput,
  type CoverChoice,
} from '@/api/client'
import { scanNote, type ScanNote } from '@/app/barcodes'
import { bookFromProposal, emptyBook, emptyCopy, prepare } from '@/app/book'
import { sourceLabel } from '@/app/format'
import { messageOf, useNotice } from '@/app/problem'
import { lastSectionKey, recall, remember } from '@/app/remember'
import BarcodeScanner from '@/components/BarcodeScanner.vue'
import BookCover from '@/components/BookCover.vue'
import BookForm from '@/components/BookForm.vue'
import CopyFields from '@/components/CopyFields.vue'
import CoverChoices from '@/components/CoverChoices.vue'
import CoverPhoto from '@/components/CoverPhoto.vue'
import FormField from '@/components/FormField.vue'
import PageHeader from '@/components/PageHeader.vue'
import PagePanel from '@/components/PagePanel.vue'

const router = useRouter()
const route = useRoute()
const notice = useNotice()
const id = useId()

type Mode = 'isbn' | 'search' | 'manual'
const modes: { value: Mode; label: string }[] = [
  { value: 'isbn', label: 'ISBN' },
  { value: 'search', label: 'Title or author' },
  { value: 'manual', label: 'By hand' },
]
const mode = ref<Mode>('isbn')
const isbn = ref('')
const query = ref('')
const finding = ref(false)
const problem = ref('')
const results = ref<Candidate[]>([])
const existing = ref(0)
const unknownISBN = ref('')
const answered = ref<string[]>([])
const unreachable = ref<string[]>([])

const choices = ref<Choices | null>(null)
onMounted(async () => {
  // Opened from a scan ("Add it", "Add by hand") with the ISBN already known.
  const given = route.query.isbn
  if (typeof given === 'string' && given) {
    isbn.value = given
    void lookUp(given)
  }
  const { data } = await api.GET('/choices')
  choices.value = data ?? null
})

// The camera, for an ISBN not typed: a store's code is explained and the camera stays on;
// a book already in the library says so; anything else is looked up.
const scanning = ref(false)
const scanResult = ref<ScanNote | null>(null)
async function scanned(code: string) {
  const { data, error } = await api.GET('/codes/{code}', { params: { path: { code } } })
  // A store's code, or a label for a copy since removed, is said and the camera stays on.
  if (error || data.kind === 'retail' || (data.kind === 'copy' && !data.bookId)) {
    scanResult.value = scanNote(error ? messageOf(error) : data.message, 'bad')
    notice.scanned(scanResult.value)
    return
  }
  scanning.value = false
  scanResult.value = null
  if (data.bookId) {
    existing.value = data.bookId
    return
  }
  isbn.value = data.isbn13
  await lookUp(data.isbn13)
}

// The review: the book as the lookup found it, and the form the owner corrects it in.
const reviewing = ref(false)
const start = ref<BookInput>(emptyBook())
const book = ref<BookInput>(emptyBook())
const copy = ref<CopyInput>(emptyCopy())
const covers = ref<CoverChoice[]>([])
const coverChoice = ref('') // a candidate's link, 'photo', or '' for none
const photo = ref<Blob | null>(null)
const photoURL = ref('')
const saving = ref(false)
// A book can go on the wishlist instead of the shelves: then it has no copy yet.
const wanted = ref(route.query.wishlist === '1')
const destinations = [
  { value: false, label: 'My library' },
  { value: true, label: 'My wishlist' },
]

function setPhoto(blob: Blob | null) {
  if (photoURL.value) URL.revokeObjectURL(photoURL.value)
  photo.value = blob
  photoURL.value = blob ? URL.createObjectURL(blob) : ''
}
onUnmounted(() => setPhoto(null))

function review(input: BookInput, candidates: CoverChoice[]) {
  start.value = structuredClone(input)
  book.value = structuredClone(input)
  copy.value = emptyCopy(recall(lastSectionKey))
  covers.value = candidates
  coverChoice.value = candidates[0]?.url ?? ''
  setPhoto(null)
  reviewing.value = true
}

async function lookUp(code = isbn.value) {
  problem.value = ''
  existing.value = 0
  unknownISBN.value = ''
  finding.value = true
  const { data, error } = await api.GET('/lookup/isbn/{isbn}', {
    params: { path: { isbn: code.trim() } },
  })
  finding.value = false
  if (error) {
    problem.value = messageOf(error)
    return
  }
  existing.value = data.existingId
  answered.value = data.answered ?? []
  unreachable.value = Object.keys(data.failed ?? {})
  if (data.existingId) return
  if (!data.answered?.length) {
    problem.value = 'No source knows this ISBN. Check the digits, or add the book by hand.'
    unknownISBN.value = data.proposal.isbn13
    return
  }
  review(bookFromProposal(data.proposal), data.proposal.covers ?? [])
}

async function find() {
  problem.value = ''
  finding.value = true
  const { data, error } = await api.GET('/lookup/search', { params: { query: { q: query.value } } })
  finding.value = false
  if (error) {
    problem.value = messageOf(error)
    return
  }
  results.value = data.candidates ?? []
  if (!results.value.length)
    problem.value = 'Nothing found. Try fewer words, or add the book by hand.'
}

async function pick(c: Candidate) {
  if (c.isbn13) {
    isbn.value = c.isbn13
    await lookUp(c.isbn13)
    return
  }
  const input = emptyBook()
  input.title = c.title
  input.subtitle = c.subtitle
  input.contributors = (c.authors ?? []).map((name) => ({ name, role: 'author' as const }))
  input.publisher = c.publisher
  input.published = c.published
  review(input, c.covers ?? [])
}

function manual() {
  const input = emptyBook()
  input.isbn = unknownISBN.value
  answered.value = []
  review(input, [])
}

function photographed(blob: Blob) {
  setPhoto(blob)
  coverChoice.value = 'photo'
}

async function save() {
  if (!book.value.title.trim()) {
    notice.failed(new Error('A book needs a title.'))
    return
  }
  saving.value = true
  const { data, error } = await api.POST('/books', {
    body: {
      book: prepare(start.value, book.value),
      copy: copy.value,
      coverUrl: coverChoice.value && coverChoice.value !== 'photo' ? coverChoice.value : '',
      wanted: wanted.value,
    },
  })
  if (error) {
    saving.value = false
    notice.failed(error)
    return
  }
  if (coverChoice.value === 'photo' && photo.value) {
    const res = await api.PUT('/books/{id}/cover', {
      params: { path: { id: data.id } },
      ...formData(photo.value),
    })
    if (res.error) notice.failed(res.error)
  }
  saving.value = false
  remember(lastSectionKey, copy.value.section)
  notice.done(wanted.value ? `${data.title} is on your wishlist` : `Added ${data.title}`)
  // The new book takes the form's place: Back goes where the owner was before adding it.
  void router.replace(`/books/${data.id}`)
}

function again() {
  reviewing.value = false
  isbn.value = ''
  results.value = []
  existing.value = 0
  unknownISBN.value = ''
  problem.value = ''
}
</script>

<template>
  <div class="mx-auto max-w-record">
    <PageHeader
      title="Add a book"
      :subtitle="reviewing ? 'Check what the sources found, then save.' : ''"
    >
      <Button
        v-if="reviewing"
        type="button"
        severity="secondary"
        label="Start over"
        @click="again"
      />
      <!-- Importing is adding many at once, and a paper is the other thing a library holds,
           so both start here rather than in the navigation. -->
      <template v-else>
        <RouterLink v-slot="{ navigate }" to="/papers/new" custom>
          <Button type="button" severity="secondary" @click="navigate"
            ><FontAwesomeIcon :icon="faFileLines" /> Add a paper</Button
          >
        </RouterLink>
        <RouterLink v-slot="{ navigate }" to="/import" custom>
          <Button type="button" severity="secondary" @click="navigate"
            ><FontAwesomeIcon :icon="faFileImport" /> Import from another app</Button
          >
        </RouterLink>
      </template>
    </PageHeader>

    <template v-if="!reviewing">
      <SelectButton
        v-model="mode"
        :options="modes"
        option-label="label"
        option-value="value"
        :allow-empty="false"
        class="mb-4"
        aria-label="How to find the book"
      />

      <form v-if="mode === 'isbn'" class="flex max-w-lg flex-col gap-2" @submit.prevent="lookUp()">
        <FormField
          label="ISBN"
          :for="`${id}-isbn`"
          hint="The 10 or 13 digits under the barcode on the back, or on the copyright page."
        >
          <div class="flex gap-2">
            <InputText
              :id="`${id}-isbn`"
              v-model="isbn"
              inputmode="numeric"
              autocomplete="off"
              placeholder="978…"
              class="min-w-0 flex-1 font-mono"
            />
            <Button type="submit" :loading="finding" :disabled="!isbn.trim()">
              <FontAwesomeIcon :icon="faMagnifyingGlass" /> Look up
            </Button>
          </div>
        </FormField>
        <div>
          <Button type="button" severity="secondary" @click="scanning = true">
            <FontAwesomeIcon :icon="faBarcode" /> Scan the barcode
          </Button>
        </div>
      </form>

      <form v-else-if="mode === 'search'" class="flex flex-col gap-3" @submit.prevent="find">
        <FormField label="Title or author" :for="`${id}-q`" class="max-w-lg">
          <div class="flex gap-2">
            <InputText
              :id="`${id}-q`"
              v-model="query"
              class="min-w-0 flex-1"
              placeholder="blood song ryan"
            />
            <Button type="submit" :loading="finding" :disabled="query.trim().length < 2">
              <FontAwesomeIcon :icon="faMagnifyingGlass" /> Search
            </Button>
          </div>
        </FormField>
        <ul v-if="results.length" class="flex flex-col gap-2" aria-label="Search results">
          <li v-for="c in results" :key="c.isbn13 || c.title + c.publisher">
            <button type="button" class="result" @click="pick(c)">
              <BookCover :url="c.thumbnail || undefined" :title="c.title" :width="48" />
              <span class="min-w-0 text-left">
                <span class="block font-semibold">{{ c.title }}</span>
                <span v-if="c.subtitle" class="block text-sm text-ink-muted">{{ c.subtitle }}</span>
                <span class="block text-sm">{{ (c.authors ?? []).join(', ') }}</span>
                <span class="block text-sm text-ink-muted">
                  {{ [c.publisher, c.published?.slice(0, 4)].filter(Boolean).join(', ') }}
                  <span v-if="c.isbn13" class="font-mono"> · {{ c.isbn13 }}</span>
                </span>
              </span>
            </button>
          </li>
        </ul>
      </form>

      <div v-else class="flex flex-col items-start gap-3">
        <p class="prose-text">
          For a book without an ISBN, or one no source knows: fill in what the book says.
        </p>
        <Button type="button" label="Start the form" @click="manual" />
      </div>

      <Dialog
        v-model:visible="scanning"
        modal
        header="Scan the barcode"
        class="w-full max-w-xl"
        :draggable="false"
        @hide="scanResult = null"
      >
        <BarcodeScanner :note="scanResult" @code="scanned" />
        <p class="sr-only" role="alert">{{ scanResult?.text }}</p>
      </Dialog>
      <p v-if="problem" class="mt-4 text-status-danger" role="alert">{{ problem }}</p>
      <Button
        v-if="unknownISBN"
        type="button"
        severity="secondary"
        class="mt-2"
        label="Add it by hand"
        @click="manual"
      />
      <PagePanel v-if="existing" class="mt-4" title="Already in your library">
        <p class="text-sm">Open the book to add another copy of it.</p>
        <RouterLink v-slot="{ navigate }" :to="`/books/${existing}`" custom>
          <Button type="button" class="mt-3" label="Open the book" @click="navigate" />
        </RouterLink>
      </PagePanel>
    </template>

    <form v-else class="flex flex-col gap-6" @submit.prevent="save">
      <p v-if="answered.length" class="text-sm text-ink-muted">
        Found by {{ answered.map(sourceLabel).join(', ') }}.
        <template v-if="unreachable.length"
          >Not reached: {{ unreachable.map(sourceLabel).join(', ') }}.</template
        >
      </p>

      <PagePanel title="Cover">
        <CoverChoices
          v-model="coverChoice"
          :choices="covers"
          :photo="photoURL"
          none
          :title="book.title"
        />
        <p v-if="!covers.length" class="mt-2 text-sm text-ink-muted">
          No source has a cover for this book. Take a photo of it.
        </p>
        <CoverPhoto class="mt-3" @picked="photographed" />
      </PagePanel>

      <BookForm v-model="book" :choices="choices" />

      <FormField label="Add it to" :for="`${id}-to`">
        <SelectButton
          :id="`${id}-to`"
          v-model="wanted"
          :aria-labelledby="`${id}-to-label`"
          :options="destinations"
          option-label="label"
          option-value="value"
          :allow-empty="false"
        />
      </FormField>

      <PagePanel v-if="!wanted" title="This copy">
        <CopyFields v-model="copy" :sections="choices?.sections" />
      </PagePanel>

      <div class="flex flex-wrap gap-2">
        <Button
          type="submit"
          :label="wanted ? 'Put it on the wishlist' : 'Save the book'"
          :loading="saving"
        />
        <Button type="button" severity="secondary" label="Start over" @click="again" />
      </div>
    </form>
  </div>
</template>

<style scoped>
.result {
  display: flex;
  gap: 0.75rem;
  width: 100%;
  padding: 0.6rem;
  border: 1px solid var(--color-outline);
  border-radius: var(--radius-control);
  background: var(--color-surface-panel);
  color: inherit;
  font: inherit;
  cursor: pointer;
}

.result:hover {
  border-color: var(--color-accent);
}
</style>
