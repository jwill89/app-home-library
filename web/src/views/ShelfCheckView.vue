<script setup lang="ts">
import { computed, onMounted, ref, useId } from 'vue'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import { api, type ShelfBook, type ShelfPlan } from '@/api/client'
import { scanNote, type ScanNote } from '@/app/barcodes'
import { callNumberText, placeLine, today } from '@/app/format'
import { messageOf, useNotice } from '@/app/problem'
import BarcodeScanner from '@/components/BarcodeScanner.vue'
import CopyGoneDialog from '@/components/CopyGoneDialog.vue'
import FormField from '@/components/FormField.vue'
import PageHeader from '@/components/PageHeader.vue'
import PagePanel from '@/components/PagePanel.vue'

// A shelf check: scan the books on one shelf, by their label or their ISBN, and see which the
// layout puts there that were not found, and which were found there but belong elsewhere.
// Every copy found is marked seen today.
const props = defineProps<{ id: number }>()
const notice = useNotice()
const fid = useId()

const shelf = ref<ShelfPlan | null>(null)
const loaded = ref(false)
async function load() {
  const { data, error } = await api.GET('/layout', { params: { query: { proposal: false } } })
  if (error) return notice.failed(error)
  shelf.value = data.shelves?.find((s) => s.id === props.id) ?? null
  loaded.value = true
}
onMounted(load)

const expected = computed(() => shelf.value?.books ?? [])
const found = ref(new Set<number>())
interface Misplaced {
  barcode: string
  title: string
  where: string
}
const elsewhere = ref<Misplaced[]>([])
const missing = computed(() => expected.value.filter((b) => !found.value.has(b.copyId)))
const title = computed(() =>
  shelf.value ? `Check ${shelf.value.bookcase}, shelf ${shelf.value.position}` : 'Shelf check',
)
const summary = computed(() =>
  loaded.value && shelf.value ? `${found.value.size} of ${expected.value.length} found` : '',
)

const note = ref<ScanNote | null>(null)
function tell(text: string, tone: ScanNote['tone']) {
  note.value = scanNote(text, tone)
  notice.scanned(note.value)
}

async function seen(copyId: number) {
  const { error } = await api.PUT('/copies/{id}/seen', {
    params: { path: { id: copyId } },
    body: { on: today() },
  })
  if (error) notice.failed(error)
}

async function heard(code: string) {
  const { data, error } = await api.GET('/codes/{code}', { params: { path: { code } } })
  if (error) return tell(messageOf(error), 'bad')
  if (data.kind === 'retail') return tell(data.message, 'bad')
  // A label names its copy; an ISBN, the next copy of its book this shelf still lacks.
  const here =
    data.kind === 'copy'
      ? expected.value.find((b) => b.copyId === data.copyId)
      : expected.value.find((b) => b.bookId === data.bookId && !found.value.has(b.copyId))
  if (here) {
    found.value = new Set(found.value).add(here.copyId)
    await seen(here.copyId)
    return tell(`Found: ${here.title}`, 'good')
  }
  if (!data.bookId) return tell('Not in your library', 'bad')
  if (data.kind === 'copy') {
    await seen(data.copyId)
    const place = await api.GET('/copies/{id}/place', { params: { path: { id: data.copyId } } })
    const where = place.data ? placeLine(place.data) : 'Somewhere else'
    elsewhere.value = [{ barcode: data.barcode, title: data.title, where }, ...elsewhere.value]
    return tell(`Belongs elsewhere: ${data.title}`, 'info')
  }
  tell(`Not on this shelf: ${data.title}`, 'info')
}

const typed = ref('')
function submitTyped() {
  const code = typed.value.replace(/[\s-]/g, '')
  typed.value = ''
  if (code) void heard(code)
}

// A book not found may be lost: the owner says so, through the one form for a copy gone.
const lostCopy = ref<ShelfBook | null>(null)
const lostOpen = computed({
  get: () => lostCopy.value !== null,
  set: (v) => {
    if (!v) lostCopy.value = null
  },
})
const row = (d: unknown) => d as ShelfBook
const misplaced = (d: unknown) => d as Misplaced
</script>

<template>
  <div class="mx-auto max-w-record">
    <PageHeader
      :title="title"
      :subtitle="summary"
      :trail="[
        { label: 'Library', to: '/' },
        { label: 'Shelves', to: '/shelves' },
      ]"
    />

    <p v-if="loaded && !shelf" class="prose-text">
      There is no such shelf in the layout. Choose a shelf to check from the Shelves page.
    </p>

    <template v-if="shelf">
      <PagePanel title="Scan" class="mb-4">
        <p class="prose-text mb-3 text-sm text-ink-muted">
          Scan each book's label inside the cover, or its ISBN. Every book found is marked seen
          today; one that belongs on another shelf says where.
        </p>
        <BarcodeScanner :note="note" @code="heard" />
        <form class="mt-4 flex max-w-lg flex-col gap-2" @submit.prevent="submitTyped">
          <FormField label="Or type a label or an ISBN" :for="`${fid}-typed`">
            <div class="flex gap-2">
              <InputText
                :id="`${fid}-typed`"
                v-model="typed"
                autocomplete="off"
                class="min-w-0 flex-1 font-mono"
              />
              <Button type="submit" label="Check it" :disabled="!typed.trim()" />
            </div>
          </FormField>
        </form>
        <p class="sr-only" role="status" aria-live="polite">{{ note?.text }}</p>
      </PagePanel>

      <PagePanel v-if="elsewhere.length" title="Found here, belongs elsewhere" class="mb-4">
        <DataTable :value="elsewhere" data-key="barcode">
          <Column header="Book">
            <template #body="{ data }">
              <div class="font-semibold">{{ misplaced(data).title }}</div>
              <div class="font-mono text-sm">{{ misplaced(data).barcode }}</div>
            </template>
          </Column>
          <Column header="Where it belongs">
            <template #body="{ data }">{{ misplaced(data).where }}</template>
          </Column>
        </DataTable>
      </PagePanel>

      <PagePanel :title="`On this shelf: ${missing.length} not found yet`">
        <DataTable :value="expected" data-key="copyId">
          <template #empty>
            <p class="text-ink-muted">The layout puts no books on this shelf.</p>
          </template>
          <Column header="Book">
            <template #body="{ data }">
              <div class="font-semibold">{{ row(data).title }}</div>
              <div class="font-mono text-sm">
                {{ row(data).barcode }} {{ callNumberText(row(data).callNumber) }}
              </div>
              <div v-if="row(data).onLoan" class="text-sm text-ink-muted">Out on loan</div>
            </template>
          </Column>
          <Column header="Found">
            <template #body="{ data }">
              <span v-if="found.has(row(data).copyId)" class="font-semibold text-status-success"
                >Found</span
              >
              <span v-else class="text-ink-muted">Not yet</span>
            </template>
          </Column>
          <Column header="Actions" header-class="column-unlabelled">
            <template #body="{ data }">
              <div class="flex justify-end">
                <Button
                  v-if="!found.has(row(data).copyId) && !row(data).onLoan"
                  type="button"
                  text
                  label="It's gone"
                  :aria-label="`${row(data).barcode} was not found: record it as gone`"
                  @click="lostCopy = row(data)"
                />
              </div>
            </template>
          </Column>
        </DataTable>
      </PagePanel>
    </template>

    <CopyGoneDialog
      v-model:visible="lostOpen"
      :copy="lostCopy ? { id: lostCopy.copyId, barcode: lostCopy.barcode } : null"
      as="lost"
      @saved="load"
    />
  </div>
</template>
