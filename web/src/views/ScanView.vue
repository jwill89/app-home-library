<script setup lang="ts">
import { computed, onMounted, ref, useId } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import { faPlus } from '@fortawesome/pro-regular-svg-icons'
import { api, isOffline, type CopyInput, type Scan } from '@/api/client'
import { emptyCopy } from '@/app/book'
import { useConfirmAction } from '@/app/confirm'
import { scanNote, type ScanNote } from '@/app/barcodes'
import { day, scanStatusLabel } from '@/app/format'
import { useLibrary } from '@/app/library'
import { useFeed } from '@/app/feed'
import { messageOf, useNotice } from '@/app/problem'
import { lastSectionKey, recall, remember } from '@/app/remember'
import BarcodeScanner from '@/components/BarcodeScanner.vue'
import CopyFields from '@/components/CopyFields.vue'
import FormField from '@/components/FormField.vue'
import PageHeader from '@/components/PageHeader.vue'
import PagePanel from '@/components/PagePanel.vue'
import ScanList from '@/components/ScanList.vue'
import { isbn13 } from '@/app/isbn'

const router = useRouter()
const notice = useNotice()
const ask = useConfirmAction()
const library = useLibrary()
const id = useId()

// What the added books' copies share, chosen when they are added.
const shared = ref<CopyInput>(emptyCopy(recall(lastSectionKey)))
const sections = ref<string[]>([])

const typed = ref('')
// The last result: across the top of the camera's picture, in a toast near it, and aloud.
// The phone is pointed at a book, so a result further down the page goes unseen.
const note = ref<ScanNote | null>(null)
function tell(text: string, tone: ScanNote['tone'], toast = true) {
  note.value = scanNote(text, tone)
  if (toast) notice.scanned(note.value)
}

// Every scan joins the list; the server looks each up while the next is scanned.
const scans = ref<Scan[]>([])
const pending = ref(0)
// The latest scan, whose outcome the banner follows once the background has looked it up.
let latest = 0
async function load() {
  const { data, error, response } = await api.GET('/scans')
  if (isOffline(response)) return
  // An error leaves the list as it was, rather than empty.
  if (error) return notice.failed(error)
  scans.value = data.scans ?? []
  pending.value = data.pending ?? 0
  const last = scans.value.find((s) => s.id === latest)
  if (last && last.status !== 'pending') {
    latest = 0
    const name = last.title || last.isbn13
    if (last.status === 'found') tell(`Not in your library: ${name}`, 'good', false)
    else if (last.status === 'owned') tell(`Already in your library: ${name}`, 'info', false)
    else tell(`${scanStatusLabel(last.status)}: ${name}`, 'bad', false)
  }
}
// A scan looked up, added, or let go, here or on another device, is told on the change feed.
useFeed(['scan'], load)

async function queue(code: string) {
  const { data, error } = await api.POST('/scans', { body: { code } })
  if (error) return tell(messageOf(error), 'bad')
  const name = data.title || data.isbn13
  if (data.status === 'pending' && data.id !== latest) {
    latest = data.id
    tell(`Scanned ${data.isbn13}. Looking it up…`, 'good')
  } else if (data.status === 'owned') {
    tell(`Already in your library: ${name}`, 'info')
  } else {
    tell(`Already on the list: ${name}`, 'info')
  }
  await load()
}

// The server says what a code is: an ISBN joins the list, one of the library's own labels
// opens the copy it is on, and anything else is explained.
async function heard(code: string) {
  const { data, error, response } = await api.GET('/codes/{code}', { params: { path: { code } } })
  if (isOffline(response)) return checkOffline(code)
  if (error) return tell(messageOf(error), 'bad')
  if (data.kind === 'isbn') return queue(code)
  if (data.kind === 'copy' && data.bookId) {
    tell(`Your copy: ${data.title}`, 'good')
    return router.push(`/books/${data.bookId}?copy=${data.copyId}`)
  }
  tell(data.message, 'bad')
}

// With no connection, the list this device kept answers "do I own this?". Nothing joins the
// list of scans until the server can be asked.
async function checkOffline(code: string) {
  if (!library.loaded) await library.load()
  const isbn = isbn13(code)
  if (!isbn) return tell('No connection, and that is not an ISBN to check offline.', 'bad')
  const row = library.rows.find((r) => r.isbn13 === isbn)
  const since = library.offlineSince ? ` (offline: as of ${day(library.offlineSince)})` : ''
  if (!row) return tell(`Not in your library${since}`, 'info')
  if (row.wantedOn) return tell(`On your wishlist: ${row.title}${since}`, 'good')
  tell(`In your library: ${row.title}${since}`, 'good')
}

function submitTyped() {
  const code = typed.value.replace(/[\s-]/g, '')
  if (!code) return
  typed.value = ''
  void heard(code)
}

const found = computed(() => scans.value.filter((s) => s.status === 'found'))
const counts = computed(() => {
  const n = scans.value.length
  if (!n) return ''
  const by = (status: string) => scans.value.filter((s) => s.status === status).length
  const parts = [
    found.value.length && `${found.value.length} to add`,
    by('pending') && `${by('pending')} being looked up`,
    by('adding') && `${by('adding')} being added`,
    by('added') && `${by('added')} added`,
    by('owned') && `${by('owned')} already yours`,
    by('not_found') + by('failed') && `${by('not_found') + by('failed')} not found`,
  ].filter(Boolean)
  return `${n} scanned: ${parts.join(', ')}`
})

async function add(list: Scan[], wanted = false) {
  const { data, error } = await api.POST('/scans/add', {
    body: {
      ids: list.map((s) => s.id),
      section: shared.value.section,
      acquiredAs: shared.value.acquiredAs,
      wanted,
    },
  })
  if (error) return notice.failed(error)
  remember(lastSectionKey, shared.value.section)
  const n = data.adding
  if (wanted) notice.done(n === 1 ? 'Putting it on the wishlist' : `Putting ${n} on the wishlist`)
  else notice.done(n === 1 ? 'Adding the book' : `Adding ${n} books`)
  await load()
}

async function done(s: Scan) {
  const { error } = await api.POST('/scans/{id}/done', { params: { path: { id: s.id } } })
  if (error) return notice.failed(error)
  await load()
}

async function remove(s: Scan) {
  const { error } = await api.DELETE('/scans/{id}', {
    params: { path: { id: s.id }, query: { removeBook: false } },
  })
  if (error) return notice.failed(error)
  await load()
}

async function clearAll() {
  const { error } = await api.POST('/scans/done')
  if (error) return notice.failed(error)
  await load()
}

function clear() {
  const left = found.value.length
  if (!left) return void clearAll()
  ask(
    {
      header: 'Clear the list?',
      message: `${left === 1 ? 'One book found is' : `${left} books found are`} not added yet and will be let go. Books already added stay in the library.`,
      accept: 'Clear the list',
      danger: true,
    },
    clearAll,
  )
}

function undo(s: Scan) {
  ask(
    {
      header: `Undo the scan of ${s.title || s.isbn13}?`,
      message: 'The book this scan added is removed again, with its copy and cover.',
      accept: 'Remove the book',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/scans/{id}', {
        params: { path: { id: s.id }, query: { removeBook: true } },
      })
      if (error) return notice.failed(error)
      await load()
    },
  )
}

async function addCopy(s: Scan) {
  const copy = { ...shared.value }
  const { error } = await api.POST('/books/{id}/copies', {
    params: { path: { id: s.bookId } },
    body: copy,
  })
  if (error) return notice.failed(error)
  notice.done(s.wantedOn ? `${s.title} is in your library now` : `Another copy of ${s.title} added`)
  await done(s)
}

function byHand(s: Scan) {
  void router.push(`/add?isbn=${s.isbn13}`)
}

onMounted(async () => {
  const [{ data }] = await Promise.all([api.GET('/choices'), load()])
  sections.value = data?.sections ?? []
})
</script>

<template>
  <div class="mx-auto max-w-record">
    <PageHeader
      title="Scan"
      subtitle="Scan one book after another. Each joins the list below and is looked up while you scan the next; then add the new ones together."
    />

    <PagePanel title="Camera" class="mb-4">
      <BarcodeScanner :note="note" @code="heard" />
      <p v-if="counts" class="mt-3 text-sm font-semibold">{{ counts }}</p>
      <form class="mt-4 flex max-w-lg flex-col gap-2" @submit.prevent="submitTyped">
        <FormField label="Or type the ISBN" :for="`${id}-typed`">
          <div class="flex gap-2">
            <InputText
              :id="`${id}-typed`"
              v-model="typed"
              inputmode="numeric"
              autocomplete="off"
              class="min-w-0 flex-1 font-mono"
            />
            <Button type="submit" label="Add to the list" :disabled="!typed.trim()" />
          </div>
        </FormField>
      </form>
      <p class="sr-only" role="status" aria-live="polite">{{ note?.text }}</p>
    </PagePanel>

    <PagePanel v-if="scans.length" title="Scanned">
      <template #actions>
        <Button
          type="button"
          severity="secondary"
          label="Clear the list"
          :disabled="scans.length === pending"
          @click="clear"
        />
      </template>
      <form
        v-if="found.length"
        class="mb-4 flex flex-col gap-4 border-b border-hairline pb-4"
        @submit.prevent="add(found)"
      >
        <CopyFields v-model="shared" :sections="sections" brief />
        <div>
          <Button type="submit">
            <FontAwesomeIcon :icon="faPlus" />
            {{ found.length === 1 ? 'Add the book' : `Add all ${found.length} books` }}
          </Button>
        </div>
      </form>
      <ScanList
        :scans="scans"
        @add="(s) => add([s])"
        @want="(s) => add([s], true)"
        @remove="remove"
        @done="done"
        @undo="undo"
        @copy="addCopy"
        @by-hand="byHand"
      />
    </PagePanel>
  </div>
</template>
