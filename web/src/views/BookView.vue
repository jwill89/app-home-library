<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import SelectButton from 'primevue/selectbutton'
import {
  faArrowsRotate,
  faArrowUpRightFromSquare,
  faBookArrowRight,
  faGraduationCap,
  faImages,
  faPen,
  faPlus,
  faTrash,
} from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import {
  api,
  formData,
  type Book,
  type Copy,
  type Reading,
  type CopyInput,
  type CourseUse,
  type CoverChoice,
  type Loan,
  type ShelfPlace,
  type ReadStatus,
  type Suggestions,
} from '@/api/client'
import { bookCite, bookTrail, emptyCopy } from '@/app/book'
import { useConfirmAction } from '@/app/confirm'
import {
  acquiredLabel,
  calendarDate,
  callNumberText,
  conditionLabel,
  courseName,
  detailLabel,
  detailText,
  goneLine,
  labelsWaiting,
  money,
  names,
  placeLine,
  readStatuses,
  readStatusLabel,
  roleLabel,
  roles,
  seriesLine,
  sourceLabel,
  termLabel,
  today,
} from '@/app/format'
import { useFeed } from '@/app/feed'
import { useLibrary } from '@/app/library'
import { loanLine, overdueBy, useReturnLoan } from '@/app/loans'
import { goBack } from '@/app/navigation'
import { useNotice } from '@/app/problem'
import { useSession } from '@/app/session'
import { useUnits } from '@/app/units'
import BookCover from '@/components/BookCover.vue'
import CitationBox from '@/components/CitationBox.vue'
import CopyFields from '@/components/CopyFields.vue'
import CourseCopyDialog from '@/components/CourseCopyDialog.vue'
import CoverChoices from '@/components/CoverChoices.vue'
import CoverPhoto from '@/components/CoverPhoto.vue'
import LateNote from '@/components/LateNote.vue'
import LoanDialog from '@/components/LoanDialog.vue'
import PagePanel from '@/components/PagePanel.vue'
import PageHeader from '@/components/PageHeader.vue'
import CopyGoneDialog from '@/components/CopyGoneDialog.vue'
import SubjectList from '@/components/SubjectList.vue'
import SuggestedChanges from '@/components/SuggestedChanges.vue'

const props = defineProps<{ id: number }>()
const router = useRouter()
const ask = useConfirmAction()
const notice = useNotice()
const library = useLibrary()
// A visitor by a share link reads the page; every control that changes the book is the owner's.
const { isOwner: owner } = useSession()
const { unit } = storeToRefs(useUnits())

const book = ref<Book | null>(null)
const sections = ref<string[]>([])
const busy = ref('')

async function load() {
  const { data, error } = await api.GET('/books/{id}', { params: { path: { id: props.id } } })
  if (error) {
    notice.failed(error)
    return
  }
  book.value = data
  document.title = `${data.title} · Home Library`
  if (owner) {
    await findPlaces(data)
  }
}

// Where each copy in the house stands on the shelves, which is the owner's alone to know: asked
// for all at once, and not for a copy that has gone.
const places = ref<Record<number, ShelfPlace>>({})
async function findPlaces(b: Book) {
  const found: Record<number, ShelfPlace> = {}
  await Promise.all(
    (b.copies ?? [])
      .filter((c) => !c.goneAs)
      .map(async (c) => {
        const { data } = await api.GET('/copies/{id}/place', { params: { path: { id: c.id } } })
        if (data) {
          found[c.id] = data
        }
      }),
  )
  places.value = found
}

// A scanned label opens its book at the copy it is on: that copy is marked and brought into
// view, and takes the focus so a screen reader starts there.
const route = useRoute()
const scanned = computed(() => Number(route.query.copy) || 0)
async function showScanned() {
  if (!scanned.value) {
    return
  }
  await nextTick()
  const el = document.getElementById(`copy-${scanned.value}`)
  el?.scrollIntoView({ block: 'center' })
  el?.focus({ preventScroll: true })
}

// This book changed, here, on another device, or in the background lookup; or a label of one
// of its copies was queued or printed.
useFeed(['book'], (refs, all) => (all || refs.has(props.id)) && load())
useFeed(
  ['label'],
  (refs, all) => (all || (book.value?.copies ?? []).some((c) => refs.has(c.id))) && load(),
)

onMounted(async () => {
  await load()
  await showScanned()
  if (!owner) {
    return
  }
  const { data } = await api.GET('/choices')
  sections.value = data?.sections ?? []
})
watch(() => props.id, load)

// The list shows covers and status too, so it loads again when it next opens.
function changed(b: Book) {
  book.value = b
}

const path = computed(() => ({ params: { path: { id: props.id } } }))

const status = computed({
  get: () => book.value?.readStatus ?? 'unread',
  set: (v: string) => void setStatus(v),
})

async function setStatus(v: string) {
  if (!book.value || v === book.value.readStatus) {
    return
  }
  const { data, error } = await library.setStatus(book.value.id, v as ReadStatus)
  if (error) {
    notice.failed(error)
    return
  }
  book.value = data
}

async function uploadCover(blob: Blob) {
  busy.value = 'cover'
  const { data, error } = await api.PUT('/books/{id}/cover', { ...path.value, ...formData(blob) })
  busy.value = ''
  if (error) {
    notice.failed(error)
    return
  }
  changed(data)
  notice.done('Cover saved')
}

// Other covers the sources have for this ISBN, to choose from.
const candidates = ref<CoverChoice[]>([])
const candidate = ref('')
const choosing = ref(false)
async function findCovers() {
  if (!book.value?.isbn13) {
    return
  }
  busy.value = 'find'
  const { data, error } = await api.GET('/lookup/isbn/{isbn}', {
    params: { path: { isbn: book.value.isbn13 } },
  })
  busy.value = ''
  if (error) {
    notice.failed(error)
    return
  }
  candidates.value = data.proposal.covers ?? []
  if (!candidates.value.length) {
    notice.failed(new Error('No source has a cover for this ISBN. Take a photo of it instead.'))
    return
  }
  candidate.value = candidates.value[0]?.url ?? ''
  choosing.value = true
}

async function useCandidate() {
  choosing.value = false
  busy.value = 'cover'
  const { data, error } = await api.POST('/books/{id}/cover', {
    ...path.value,
    body: { url: candidate.value },
  })
  busy.value = ''
  if (error) {
    notice.failed(error)
    return
  }
  changed(data)
}

function removeCover() {
  ask(
    {
      header: 'Remove the cover?',
      message: 'The book keeps everything else.',
      accept: 'Remove the cover',
      danger: true,
    },
    async () => {
      const { data, error } = await api.DELETE('/books/{id}/cover', path.value)
      if (error) {
        notice.failed(error)
        return
      }
      changed(data)
    },
  )
}

// Looking up again fills blanks at once; what the sources give otherwise waits for the owner.
const suggestions = ref<Suggestions[]>([])
const suggesting = ref(false)
async function lookUpAgain() {
  busy.value = 'refresh'
  const { data, error } = await api.POST('/books/{id}/refresh', path.value)
  busy.value = ''
  if (error) {
    notice.failed(error)
    return
  }
  changed(data.book)
  if (data.fields?.length) {
    suggestions.value = [data]
    suggesting.value = true
    return
  }
  notice.done('Looked up again. Blanks are filled where a source knew more; nothing else to offer.')
}

function removeBook() {
  ask(
    {
      header: 'Remove this book?',
      message: 'The book, its copies, and its cover are deleted. This cannot be undone.',
      accept: 'Remove the book',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/books/{id}', path.value)
      if (error) {
        notice.failed(error)
        return
      }
      notice.done('Book removed')
      await goBack(router, '/', `/books/${props.id}`)
    },
  )
}

// One dialog adds a copy or edits one.
const editing = ref<{ id: number; input: CopyInput } | null>(null)
const copyOpen = ref(false)
function editCopy(c?: Copy) {
  editing.value = c
    ? { id: c.id, input: { ...c } }
    : { id: 0, input: emptyCopy(book.value?.copies?.[0]?.section ?? '') }
  copyOpen.value = true
}

async function saveCopy() {
  if (!editing.value) {
    return
  }
  const { id, input } = editing.value
  const { error } = id
    ? await api.PUT('/copies/{id}', { params: { path: { id } }, body: input })
    : await api.POST('/books/{id}/copies', { ...path.value, body: input })
  if (error) {
    notice.failed(error)
    return
  }
  copyOpen.value = false
  await load()
}

function removeCopy(c: Copy) {
  ask(
    {
      header: `Remove copy ${c.barcode}?`,
      message: 'The book stays in the library with its other copies.',
      accept: 'Remove the copy',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/copies/{id}', { params: { path: { id: c.id } } })
      if (error) {
        notice.failed(error)
        return
      }
      await load()
    },
  )
}

// One dialog lends a copy or changes the loan it is out on.
const loanOpen = ref(false)
const lending = ref<{ id: number; barcode: string } | null>(null)
const loan = ref<Loan | null>(null)
function lend(c: Copy) {
  lending.value = { id: c.id, barcode: c.barcode }
  loan.value = null
  loanOpen.value = true
}
function editLoan(l: Loan) {
  lending.value = null
  loan.value = l
  loanOpen.value = true
}

// A copy that has left the library, and one that comes back.
const goneCopy = ref<Copy | null>(null)
const goneOpen = computed({
  get: () => goneCopy.value !== null,
  set: (v) => {
    if (!v) {
      goneCopy.value = null
    }
  },
})
async function bringBack(c: Copy) {
  const { error } = await api.DELETE('/copies/{id}/gone', { params: { path: { id: c.id } } })
  if (error) {
    notice.failed(error)
    return
  }
  notice.done(`${c.barcode} is back in the library`)
  await load()
}

// Read again: the read it had is kept among the earlier ones.
async function readAgain() {
  if (!book.value) {
    return
  }
  const { data, error } = await api.POST('/books/{id}/read-again', {
    params: { path: { id: book.value.id } },
    body: { on: today() },
  })
  if (error) {
    notice.failed(error)
    return
  }
  book.value = data
  notice.done('Begun again today')
}
function removeReading(r: Reading) {
  ask(
    {
      header: 'Remove this earlier read?',
      message: `The read ${r.finishedOn ? `finished ${calendarDate(r.finishedOn)}` : 'not finished'} leaves the book's history. The book stays.`,
      accept: 'Remove the read',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/readings/{id}', { params: { path: { id: r.id } } })
      if (error) {
        notice.failed(error)
        return
      }
      await load()
    },
  )
}

const returnLoan = useReturnLoan()
async function returned(l: Loan) {
  if (await returnLoan(l)) {
    await load()
  }
}

const courseOpen = ref(false)
function takeOff(u: CourseUse) {
  const name = courseName(u.course)
  ask(
    {
      header: `Take this book off ${name}?`,
      message: 'The course stays, with its other books, and so does this book.',
      accept: 'Take it off',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/courses/{id}/copies/{copyId}', {
        params: { path: { id: u.course.id, copyId: u.copyId } },
      })
      if (error) {
        notice.failed(error)
        return
      }
      await load()
    },
  )
}
// Labels: what a copy has waiting, and putting both back in the queue for one worn or torn.
async function reprint(c: Copy) {
  for (const kind of ['barcode', 'spine'] as const) {
    const { error } = await api.PUT('/labels/{kind}/{copyId}', {
      params: { path: { kind, copyId: c.id } },
    })
    if (error) {
      notice.failed(error)
      return
    }
  }
  notice.done(`${c.barcode}'s labels wait with the others to print`)
  await load()
}
const barcodeOf = (copyId: number) => book.value?.copies?.find((c) => c.id === copyId)?.barcode

const people = computed(() =>
  roles
    .map((r) => ({
      role: r.value,
      label: roleLabel(r.value),
      names: names(book.value?.contributors ?? null, r.value),
    }))
    .filter((r) => r.names),
)

// The details the page lists, in order; the numbers among them set in the code face.
const shownDetails = [
  'category',
  'isbn13',
  'isbn10',
  'ddc',
  'lcc',
  'lccn',
  'format',
  'publisher',
  'publishPlace',
  'published',
  'editionStatement',
  'pages',
  'heightMm',
  'widthMm',
  'thicknessMm',
  'language',
  'genres',
]
const numbers = new Set(['isbn13', 'isbn10', 'ddc', 'lcc', 'lccn'])
const details = computed(() => {
  const b = book.value
  if (!b) {
    return []
  }
  return shownDetails
    .map((field) => ({
      label: detailLabel(field),
      value: detailText(b, field, unit.value),
      mono: numbers.has(field),
      source: sourceLabel(b.sources[field]),
    }))
    .filter((r) => r.value)
})

function copyLine(c: Copy): string {
  const parts = [c.section, c.location].filter(Boolean)
  const bought = [
    c.acquiredAs !== 'unknown' ? `Bought ${acquiredLabel(c.acquiredAs).toLowerCase()}` : '',
    c.acquiredFrom ? `from ${c.acquiredFrom}` : '',
    c.acquiredOn ? `on ${calendarDate(c.acquiredOn)}` : '',
    money(c.priceCents) ? `for ${money(c.priceCents)}` : '',
  ]
    .filter(Boolean)
    .join(' ')
  if (bought) {
    parts.push(bought)
  }
  if (c.condition !== 'unknown') {
    parts.push(conditionLabel(c.condition))
  }
  if (c.dustJacket === 'yes') {
    parts.push('Dust jacket')
  }
  if (c.signed) {
    parts.push('Signed')
  }
  return parts.join(' · ')
}
</script>

<template>
  <div v-if="book" class="mx-auto max-w-record">
    <div class="flex flex-col gap-6 md:flex-row">
      <div class="flex flex-col items-center gap-3 md:items-start">
        <BookCover :cover="book.cover" :title="book.title" :width="240" />
        <CoverPhoto v-if="owner" @picked="uploadCover" />
        <div v-if="owner" class="flex flex-wrap gap-2">
          <Button
            v-if="book.isbn13"
            type="button"
            text
            :loading="busy === 'find'"
            @click="findCovers"
          >
            <FontAwesomeIcon :icon="faImages" /> Other covers
          </Button>
          <Button v-if="book.cover.id" type="button" text severity="danger" @click="removeCover"
            >Remove cover</Button
          >
        </div>
        <p v-if="book.sources?.cover" class="text-sm text-ink-muted">
          Cover from {{ sourceLabel(book.sources.cover) }}
        </p>
      </div>

      <div class="min-w-0 flex-1">
        <PageHeader :title="book.title" :subtitle="book.subtitle" :trail="bookTrail(book)">
          <template #details>
            <p v-for="p in people" :key="p.role" class="mt-2">
              <span v-if="p.role !== 'author'" class="text-ink-muted">{{ p.label }}: </span
              >{{ p.names }}
            </p>
            <p v-if="book.series" class="mt-2 text-ink-muted">
              {{ seriesLine(book.series, book.seriesNumber) }}
            </p>
          </template>
        </PageHeader>

        <div>
          <SelectButton
            v-if="owner"
            v-model="status"
            :options="readStatuses"
            option-label="label"
            option-value="value"
            :allow-empty="false"
            aria-label="Reading status"
          />
          <p v-else>
            <span class="text-ink-muted">Status: </span>{{ readStatusLabel(book.readStatus) }}
          </p>
          <p
            v-if="book.readStatus === 'reading' && book.currentPage"
            class="mt-1 text-sm text-ink-muted"
          >
            On page {{ book.currentPage
            }}<template v-if="book.pages"> of {{ book.pages }}</template>
          </p>
          <Button
            v-if="owner && book.readStatus === 'read'"
            type="button"
            text
            class="mt-1"
            label="Read it again"
            @click="readAgain"
          />
        </div>
        <div v-if="book.readings?.length" class="mt-4">
          <h2 class="mb-2 text-lg">Earlier reads</h2>
          <DataTable :value="book.readings" data-key="id">
            <Column header="Finished">
              <template #body="{ data: r }: { data: Reading }">{{
                r.finishedOn ? calendarDate(r.finishedOn) : 'Not finished'
              }}</template>
            </Column>
            <Column header="Begun">
              <template #body="{ data: r }: { data: Reading }">{{
                r.startedOn ? calendarDate(r.startedOn) : ''
              }}</template>
            </Column>
            <Column v-if="owner" header="Actions" header-class="column-unlabelled">
              <template #body="{ data: r }: { data: Reading }">
                <div class="flex justify-end">
                  <Button
                    type="button"
                    text
                    severity="danger"
                    label="Remove"
                    :aria-label="`Remove the read finished ${calendarDate(r.finishedOn) || 'not at all'}`"
                    @click="removeReading(r)"
                  />
                </div>
              </template>
            </Column>
          </DataTable>
        </div>

        <p v-if="book.wantedOn" class="mt-4 font-semibold">
          On the wishlist since {{ calendarDate(book.wantedOn) }}.
        </p>
        <div v-if="owner" class="mt-4 flex flex-wrap gap-2">
          <Button v-if="book.wantedOn" type="button" @click="editCopy()"
            ><FontAwesomeIcon :icon="faPlus" /> I got it</Button
          >
          <RouterLink v-slot="{ navigate }" :to="`/books/${book.id}/edit`" custom>
            <Button type="button" severity="secondary" @click="navigate"
              ><FontAwesomeIcon :icon="faPen" /> Edit</Button
            >
          </RouterLink>
          <Button
            v-if="book.isbn13"
            type="button"
            severity="secondary"
            :loading="busy === 'refresh'"
            @click="lookUpAgain"
          >
            <FontAwesomeIcon :icon="faArrowsRotate" /> Look up again
          </Button>
          <Button type="button" severity="danger" text @click="removeBook">
            <FontAwesomeIcon :icon="faTrash" /> Remove the book
          </Button>
        </div>
        <p v-if="book.enrichment === 'pending'" class="mt-2 text-sm text-ink-muted">
          This book is waiting for the background lookup.
        </p>
        <p v-else-if="book.enrichment === 'failed'" class="mt-2 text-sm text-ink-muted">
          The background lookup could not reach the sources. Look up again to retry.
        </p>

        <dl class="detail-list mt-6">
          <template v-for="d in details" :key="d.label">
            <dt>{{ d.label }}</dt>
            <dd>
              <span :class="{ 'font-mono tabular': d.mono }">{{ d.value }}</span>
              <span v-if="d.source" class="details__source">from {{ d.source }}</span>
            </dd>
          </template>
          <template v-if="book.links?.length">
            <dt>Elsewhere</dt>
            <dd class="flex flex-wrap gap-x-4">
              <a
                v-for="l in book.links"
                :key="l.source"
                :href="l.url"
                target="_blank"
                rel="noopener"
                >{{ sourceLabel(l.source) }}<span class="sr-only"> (opens in a new tab)</span>
                <FontAwesomeIcon
                  :icon="faArrowUpRightFromSquare"
                  class="ml-1 text-sm"
                  aria-hidden="true"
              /></a>
            </dd>
          </template>
        </dl>

        <section v-if="book.description" class="mt-6">
          <h2 class="mb-2 text-lg">Description</h2>
          <p class="prose-text">{{ book.description }}</p>
          <p v-if="owner && book.descriptionCut" class="mt-2 text-sm text-ink-muted">
            The Bookshelf export cut this description off, and no source has it whole. Paste the
            rest on the Edit page if you have it.
          </p>
        </section>
        <SubjectList :subjects="book.subjects ?? []" class="mt-6" />
        <section v-if="book.notes" class="mt-6">
          <h2 class="mb-2 text-lg">Notes</h2>
          <p class="prose-text">{{ book.notes }}</p>
        </section>
      </div>
    </div>

    <PagePanel title="Copies" class="mt-8">
      <template v-if="owner" #actions>
        <Button type="button" severity="secondary" @click="editCopy()"
          ><FontAwesomeIcon :icon="faPlus" /> Add a copy</Button
        >
      </template>
      <DataTable
        :value="book.copies ?? []"
        data-key="id"
        :row-class="(c: Copy) => (c.id === scanned ? 'copies__scanned-row' : '')"
      >
        <template #empty>
          <p class="text-ink-muted">No copy is on the shelves. The record stays for the history.</p>
        </template>
        <Column header="Copy">
          <template #body="{ data: c }: { data: Copy }">
            <div :id="`copy-${c.id}`" :tabindex="c.id === scanned ? -1 : undefined">
              <p v-if="c.id === scanned" class="copies__scanned">The copy you scanned</p>
              <div class="flex flex-wrap items-center justify-between gap-2">
                <span class="flex flex-wrap items-baseline gap-x-3">
                  <span class="font-mono text-lg tabular">{{ c.barcode }}</span>
                  <span v-if="c.callNumber?.length" class="font-mono">
                    <span class="sr-only">Call number </span>{{ callNumberText(c.callNumber) }}
                  </span>
                </span>
                <span v-if="owner && c.goneAs" class="flex flex-wrap gap-1">
                  <Button
                    type="button"
                    severity="secondary"
                    label="Bring it back"
                    :aria-label="`${c.barcode} is back in the library`"
                    @click="bringBack(c)"
                  />
                  <Button
                    type="button"
                    text
                    severity="danger"
                    :aria-label="`Remove copy ${c.barcode}`"
                    label="Remove"
                    @click="removeCopy(c)"
                  />
                </span>
                <span v-else-if="owner" class="flex flex-wrap gap-1">
                  <Button
                    v-if="c.loan"
                    type="button"
                    severity="secondary"
                    label="Returned"
                    :aria-label="`${c.barcode} is back: mark it returned today`"
                    @click="returned(c.loan)"
                  />
                  <Button
                    v-else
                    type="button"
                    severity="secondary"
                    :aria-label="`Lend copy ${c.barcode}`"
                    @click="lend(c)"
                    ><FontAwesomeIcon :icon="faBookArrowRight" /> Lend</Button
                  >
                  <Button
                    type="button"
                    text
                    :aria-label="`Edit copy ${c.barcode}`"
                    label="Edit"
                    @click="editCopy(c)"
                  />
                  <Button
                    v-if="!c.loan"
                    type="button"
                    text
                    :aria-label="`Copy ${c.barcode} has left the library`"
                    label="It's gone"
                    @click="goneCopy = c"
                  />
                  <Button
                    type="button"
                    text
                    severity="danger"
                    :aria-label="`Remove copy ${c.barcode}`"
                    label="Remove"
                    @click="removeCopy(c)"
                  />
                </span>
              </div>
              <p v-if="c.goneAs" class="text-sm font-semibold">{{ goneLine(c) }}</p>
              <div v-if="c.loan" class="mb-1 flex flex-wrap items-center gap-x-3">
                <span class="text-sm font-semibold">{{ loanLine(c.loan) }}</span>
                <LateNote v-if="overdueBy(c.loan)" :days="overdueBy(c.loan)" />
                <Button
                  type="button"
                  text
                  label="Change the loan"
                  :aria-label="`Change the loan of ${c.barcode}`"
                  @click="editLoan(c.loan)"
                />
              </div>
              <p v-else-if="!owner" class="text-sm font-semibold">
                {{ c.onLoan ? 'Out on loan' : 'On the shelf' }}
              </p>
              <p v-if="copyLine(c)" class="text-sm">{{ copyLine(c) }}</p>
              <p v-if="c.notes" class="prose-text mt-1 text-sm text-ink-muted">{{ c.notes }}</p>
              <p v-if="owner && c.seenOn && !c.goneAs" class="text-sm text-ink-muted">
                Last found on a shelf check {{ calendarDate(c.seenOn) }}
              </p>
              <p v-if="owner && !c.goneAs && places[c.id]?.inUse" class="mt-1 text-sm">
                {{ placeLine(places[c.id]!) }}
                <RouterLink to="/shelves" class="text-accent">Shelves</RouterLink>
              </p>
              <div
                v-if="owner && !c.goneAs"
                class="mt-1 flex flex-wrap items-center gap-x-3 text-sm"
              >
                <template v-if="c.labels?.length">
                  <span class="text-ink-muted">{{ labelsWaiting(c.labels) }}</span>
                  <RouterLink to="/labels" class="link-target text-accent"
                    >Open the Labels page</RouterLink
                  >
                </template>
                <Button
                  v-else
                  type="button"
                  text
                  label="Print its labels again"
                  :aria-label="`Print the labels for ${c.barcode} again`"
                  @click="reprint(c)"
                />
              </div>
            </div>
          </template>
        </Column>
      </DataTable>
    </PagePanel>

    <PagePanel v-if="owner" title="Courses" class="mt-6">
      <template #actions>
        <Button
          v-if="book.copies?.length"
          type="button"
          severity="secondary"
          @click="courseOpen = true"
          ><FontAwesomeIcon :icon="faGraduationCap" /> Add to a course</Button
        >
      </template>
      <DataTable
        :value="book.courses ?? []"
        :data-key="(u: CourseUse) => `${u.course.id}-${u.copyId}`"
      >
        <template #empty>
          <p class="text-ink-muted">
            {{
              book.copies?.length
                ? 'Not on any course.'
                : 'A course lists copies. Add a copy of this book first.'
            }}
          </p>
        </template>
        <Column header="Course">
          <template #body="{ data }">
            <RouterLink :to="`/courses/${(data as CourseUse).course.id}`" class="row-link">{{
              courseName((data as CourseUse).course)
            }}</RouterLink>
            <div class="text-sm text-ink-muted">
              {{
                [
                  termLabel((data as CourseUse).course),
                  (data as CourseUse).course.school,
                  (book.copies?.length ?? 0) > 1 ? barcodeOf((data as CourseUse).copyId) : '',
                ]
                  .filter(Boolean)
                  .join(' · ')
              }}
            </div>
            <div class="text-sm sm:hidden">
              {{ (data as CourseUse).required ? 'Required' : 'Not required' }}
            </div>
          </template>
        </Column>
        <Column header="Required" header-class="max-sm:hidden" body-class="max-sm:hidden">
          <template #body="{ data }">{{
            (data as CourseUse).required ? 'Required' : 'Not required'
          }}</template>
        </Column>
        <Column header="Take off" header-class="column-unlabelled">
          <template #body="{ data }">
            <Button
              type="button"
              text
              severity="danger"
              class="whitespace-nowrap"
              label="Take off"
              :aria-label="`Take this book off ${courseName((data as CourseUse).course)}`"
              @click="takeOff(data as CourseUse)"
            />
          </template>
        </Column>
      </DataTable>
    </PagePanel>

    <PagePanel title="Cite it" class="mt-6">
      <CitationBox :work="bookCite(book)" />
    </PagePanel>

    <LoanDialog
      v-if="owner"
      v-model:visible="loanOpen"
      :loan="loan"
      :copy="lending"
      @saved="load"
    />
    <CourseCopyDialog v-if="owner" v-model:visible="courseOpen" :book="book" @saved="load" />

    <Dialog
      v-if="owner"
      v-model:visible="copyOpen"
      modal
      :header="editing?.id ? 'Edit the copy' : 'Add a copy'"
      class="w-full max-w-2xl"
    >
      <CopyFields v-if="editing" v-model="editing.input" :sections="sections" />
      <template #footer>
        <Button type="button" severity="secondary" label="Cancel" @click="copyOpen = false" />
        <Button type="button" label="Save the copy" @click="saveCopy" />
      </template>
    </Dialog>

    <Dialog
      v-if="owner"
      v-model:visible="choosing"
      modal
      header="Choose a cover"
      class="w-full max-w-2xl"
    >
      <CoverChoices v-model="candidate" :choices="candidates" :title="book.title" />
      <template #footer>
        <Button type="button" severity="secondary" label="Cancel" @click="choosing = false" />
        <Button type="button" label="Use this cover" :disabled="!candidate" @click="useCandidate" />
      </template>
    </Dialog>
    <SuggestedChanges
      v-if="owner"
      v-model:visible="suggesting"
      :suggestions="suggestions"
      @saved="([b]) => b && changed(b)"
    />
    <CopyGoneDialog v-if="owner" v-model:visible="goneOpen" :copy="goneCopy" @saved="load()" />
  </div>
</template>

<style scoped>
.details__source {
  margin-left: 0.5rem;
  font-size: 0.875rem;
  color: var(--color-ink-muted);
}

/* A bar beside the scanned copy marks it; its colors stay the panel's, whose pairs are tested. */
:deep(.copies__scanned-row) > td:first-child {
  border-left: 4px solid var(--color-accent);
}

.copies__scanned {
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--color-accent);
}
</style>
