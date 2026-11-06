<script setup lang="ts">
import { computed, onMounted, ref, useId, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { faArrowsRotate, faCheck, faPen } from '@fortawesome/pro-regular-svg-icons'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import { api, type Book, type BookRow, type Series, type Suggestions } from '@/api/client'
import { useConfirmAction } from '@/app/confirm'
import { count, numberRanges } from '@/app/format'
import { useLibrary } from '@/app/library'
import { goBack, type Crumb } from '@/app/navigation'
import { useNotice } from '@/app/problem'
import { useSession } from '@/app/session'
import BookCover from '@/components/BookCover.vue'
import FormField from '@/components/FormField.vue'
import PageHeader from '@/components/PageHeader.vue'
import ReadStatusChoice from '@/components/ReadStatusChoice.vue'
import SuggestedChanges from '@/components/SuggestedChanges.vue'

// One series: its books in order, had and wanted, and what is done to all of them at once
// (looked up again, marked read, renamed or merged, or said to be no series), or to one (taken
// out of it).
const props = defineProps<{ id: number }>()
const router = useRouter()
const notice = useNotice()
const ask = useConfirmAction()
const owner = useSession().isOwner
const library = useLibrary()
const { rows, loaded } = storeToRefs(library)
const formId = useId()

const series = ref<Series | null>(null)
async function load() {
  const { data, error } = await api.GET('/series/{id}', { params: { path: { id: props.id } } })
  if (error) return notice.failed(error)
  series.value = data
  document.title = `${data.name} · Home Library`
}
onMounted(() => {
  if (!loaded.value) void library.load()
})
watch(() => props.id, load, { immediate: true })

const trail: Crumb[] = [
  { label: 'Library', to: '/' },
  { label: 'Series', to: '/series' },
]

// In the series' order; a volume without a number after the numbered ones, by title.
const number = (v: string) => (v.trim() === '' || Number.isNaN(Number(v)) ? Infinity : Number(v))
const books = computed(() =>
  rows.value
    .filter((r) => r.seriesId === props.id)
    .sort(
      (a, b) => number(a.seriesNumber) - number(b.seriesNumber) || a.title.localeCompare(b.title),
    ),
)
const row = (d: unknown) => d as BookRow

const summary = computed(() => {
  const s = series.value
  if (!s) return ''
  return [
    count(s.books, 'book', 'books'),
    s.gaps?.length ? `missing ${numberRanges(s.gaps)}` : '',
    s.wanted?.length ? `wanted ${numberRanges(s.wanted)}` : '',
    s.next ? `next ${s.next}` : '',
  ]
    .filter(Boolean)
    .join(' · ')
})

async function changed() {
  await Promise.all([library.load(), load()])
}

// Changes taken from a lookup can move every book to another series ("Malazan book of the
// fallen -- bk. 9" into "Malazan Book of the Fallen"): then that series is the page to be on.
async function saved(moved: Book[]) {
  await library.load()
  if (books.value.length) return load()
  const into = new Set(moved.map((b) => b.seriesId).filter(Boolean))
  progress.value = ''
  if (into.size === 1) return router.replace(`/series/${[...into][0]}`)
  return goBack(router, '/series', `/series/${props.id}`)
}

// Looked up again, one book after another: blanks are filled as each answers, and what the
// sources give otherwise is gathered for the owner to take or leave at the end.
const lookable = computed(() => books.value.filter((b) => b.isbn13))
const looking = ref(false)
const stopping = ref(false)
const progress = ref('')
const suggestions = ref<Suggestions[]>([])
const suggesting = ref(false)
async function lookUpAll() {
  const list = lookable.value
  looking.value = true
  stopping.value = false
  const found: Suggestions[] = []
  const failed: string[] = []
  let asked = 0
  for (const b of list) {
    if (stopping.value) break
    progress.value = `Looking up ${asked + 1} of ${list.length}: ${b.title}`
    const { data, error } = await api.POST('/books/{id}/refresh', {
      params: { path: { id: b.id } },
    })
    asked++
    if (error) failed.push(b.title)
    else if (data.fields?.length) found.push(data)
  }
  looking.value = false
  await changed()
  const said = [
    `Looked up ${count(asked, 'book', 'books')}${asked < list.length ? ` of ${list.length}` : ''}.`,
    found.length
      ? `${count(found.length, 'has', 'have')} details the sources give otherwise.`
      : 'Blanks are filled where a source knew more; nothing else differs.',
    failed.length ? `No source answered for ${failed.join(', ')}: try again later.` : '',
  ]
  progress.value = said.filter(Boolean).join(' ')
  if (found.length) {
    suggestions.value = found
    suggesting.value = true
  }
}

// It records the past, so no book is given a day.
function markRead() {
  const s = series.value
  if (!s) return
  ask(
    {
      header: `Mark ${s.name} read?`,
      message:
        'Every book of the series in the library is marked read. Each keeps the days it has; none is given today’s.',
      accept: 'Mark them read',
      reject: 'Not now',
    },
    async () => {
      const { data, error } = await api.PUT('/series/{id}/read-status', {
        params: { path: { id: s.id } },
        body: { readStatus: 'read' },
      })
      if (error) return notice.failed(error)
      notice.done(
        data.changed === 0
          ? `Every book of ${s.name} was read already`
          : `Marked ${count(data.changed, 'book', 'books')} of ${s.name} read`,
      )
      await library.load()
    },
  )
}

function leave(b: BookRow) {
  const s = series.value
  if (!s) return
  ask(
    {
      header: `Take ${b.title} out of ${s.name}?`,
      message: `It leaves the series${b.seriesNumber ? ` and its number in it, ${b.seriesNumber}` : ''}, and keeps everything else. No lookup puts it in a series again; its Edit page can still give it one.`,
      accept: 'Take it out',
      reject: 'Leave it in',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/series/{id}/books/{bookId}', {
        params: { path: { id: s.id, bookId: b.id } },
      })
      if (error) return notice.failed(error)
      notice.done(`${b.title} is out of ${s.name}`)
      await library.load()
      if (!books.value.length) return goBack(router, '/series', `/series/${s.id}`)
      await load()
    },
  )
}

// A publisher's line a source gave as a series (Penguin classics): its books leave it, and the
// name is kept as no series, so no lookup or import makes it one again.
function dismiss() {
  const s = series.value
  if (!s) return
  ask(
    {
      header: `Is ${s.name} not a series?`,
      message: `${books.value.length === 1 ? 'Its book leaves' : `Its ${books.value.length} books leave`} it and keep everything else. No lookup or import puts a book in it again; typing its name on a book makes it a series again.`,
      accept: 'It is not a series',
      reject: 'Keep the series',
      danger: true,
    },
    async () => {
      const { data, error } = await api.DELETE('/series/{id}', { params: { path: { id: s.id } } })
      if (error) return notice.failed(error)
      notice.done(`${s.name} is no series now: ${count(data.changed, 'book', 'books')} left it`)
      await goBack(router, '/series', `/series/${s.id}`)
    },
  )
}

// Renamed, or given another series' name, merged into that one.
const renaming = ref(false)
const name = ref('')
const names = ref<string[]>([])
async function rename() {
  name.value = series.value?.name ?? ''
  renaming.value = true
  const { data } = await api.GET('/series')
  names.value = (data?.series ?? []).filter((s) => s.id !== props.id).map((s) => s.name)
}
async function saveName() {
  const s = series.value
  if (!s) return
  const { data, error } = await api.PUT('/series/{id}', {
    params: { path: { id: s.id } },
    body: { name: name.value },
  })
  if (error) return notice.failed(error)
  renaming.value = false
  if (data.id !== s.id) {
    notice.done(`${s.name} is part of ${name.value.trim()} now`)
    return router.replace(`/series/${data.id}`)
  }
  notice.done(`Renamed ${name.value.trim()}`)
  await changed()
}
</script>

<template>
  <div v-if="series" class="mx-auto max-w-page">
    <PageHeader :title="series.name" :subtitle="summary" :trail="trail">
      <template v-if="owner">
        <Button
          v-if="!looking && lookable.length"
          type="button"
          severity="secondary"
          @click="lookUpAll"
          ><FontAwesomeIcon :icon="faArrowsRotate" /> Look them up again</Button
        >
        <Button
          v-if="looking"
          type="button"
          severity="secondary"
          label="Stop looking up"
          :disabled="stopping"
          @click="stopping = true"
        />
        <Button type="button" severity="secondary" :disabled="looking" @click="markRead"
          ><FontAwesomeIcon :icon="faCheck" /> Mark read</Button
        >
        <Button type="button" severity="secondary" :disabled="looking" @click="rename"
          ><FontAwesomeIcon :icon="faPen" /> Rename</Button
        >
        <Button
          type="button"
          severity="danger"
          text
          label="Not a series"
          :disabled="looking"
          @click="dismiss"
        />
      </template>
    </PageHeader>

    <p v-if="progress" role="status" class="prose-text mb-4">{{ progress }}</p>

    <DataTable :value="books" data-key="id" :loading="!loaded">
      <template #empty>
        <p class="py-6 text-center text-ink-muted">No books in this series.</p>
      </template>
      <Column header="No." header-class="w-14" body-class="tabular">
        <template #body="{ data }">{{ row(data).seriesNumber }}</template>
      </Column>
      <Column
        header="Cover"
        header-class="column-unlabelled w-16 max-sm:hidden"
        body-class="max-sm:hidden"
      >
        <template #body="{ data }">
          <BookCover :cover="row(data).cover" :title="row(data).title" :width="48" />
        </template>
      </Column>
      <!-- One cell holds the book and what is done to it, laid out to fit a phone's width. -->
      <Column header="Book">
        <template #body="{ data }">
          <div class="volume">
            <div class="volume__title">
              <RouterLink :to="`/books/${row(data).id}`" class="row-link">{{
                row(data).title
              }}</RouterLink>
              <div v-if="row(data).authors" class="text-sm text-ink-muted">
                {{ row(data).authors }}
              </div>
              <div v-if="row(data).wantedOn" class="text-sm text-ink-muted">On the wishlist</div>
            </div>
            <ReadStatusChoice v-if="!row(data).wantedOn" :book="row(data)" />
            <Button
              v-if="owner"
              type="button"
              text
              label="Not in this series"
              :aria-label="`Not in this series: ${row(data).title}`"
              :disabled="looking"
              @click="leave(row(data))"
            />
          </div>
        </template>
      </Column>
    </DataTable>

    <SuggestedChanges
      v-if="owner"
      v-model:visible="suggesting"
      :suggestions="suggestions"
      @saved="saved"
    />

    <Dialog
      v-if="owner"
      v-model:visible="renaming"
      modal
      :header="`Rename ${series.name}`"
      class="w-full max-w-md"
    >
      <form :id="`${formId}-form`" @submit.prevent="saveName">
        <FormField
          label="Name"
          :for="`${formId}-name`"
          hint="Give it another series' name to make the two one."
          required
        >
          <InputText :id="`${formId}-name`" v-model="name" :list="`${formId}-names`" required />
          <datalist :id="`${formId}-names`">
            <option v-for="n in names" :key="n" :value="n" />
          </datalist>
        </FormField>
      </form>
      <template #footer>
        <Button type="button" severity="secondary" label="Cancel" @click="renaming = false" />
        <Button type="submit" :form="`${formId}-form`" label="Save" :disabled="!name.trim()" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.volume {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 1rem;
}

.volume__title {
  flex: 1 1 14rem;
  min-width: 0;
}
</style>
