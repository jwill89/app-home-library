<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import ToggleSwitch from 'primevue/toggleswitch'
import {
  api,
  type Book,
  type BookInput,
  type Document,
  type DocumentInput,
  type DocumentSuggestions,
  type Suggestions,
} from '@/api/client'
import { bookToInput } from '@/app/book'
import { documentToInput } from '@/app/documents'
import {
  count,
  detailLabel,
  detailText,
  paperDetailLabel,
  paperDetailText,
  sourceLabel,
} from '@/app/format'
import { useNotice } from '@/app/problem'
import { useUnits } from '@/app/units'

// What a lookup the owner asked for would change, for one book or paper or many: each detail
// the sources now give otherwise, beside what it has, taken or left one by one. Nothing
// changes until the owner says. Books come as `suggestions`, papers as `papers`.
const props = defineProps<{ suggestions?: Suggestions[]; papers?: DocumentSuggestions[] }>()
const emit = defineEmits<{ saved: [books: Book[]]; savedPapers: [papers: Document[]] }>()
const open = defineModel<boolean>('visible', { required: true })
const notice = useNotice()
const { unit } = storeToRefs(useUnits())

interface Row {
  key: string
  recordId: number
  record: string
  field: string
  label: string
  now: string
  nowFrom: string
  then: string
  thenFrom: string
}

const books = computed(() => props.suggestions ?? [])
const papers = computed(() => props.papers ?? [])
const noun = computed(() => (papers.value.length ? 'paper' : 'book'))
const many = computed(() => books.value.length + papers.value.length > 1)

const rows = computed<Row[]>(() => [
  ...books.value.flatMap((s) => {
    const now = bookToInput(s.book)
    return (s.fields ?? []).map((field) => ({
      key: `${s.book.id}:${field}`,
      recordId: s.book.id,
      record: s.book.title,
      field,
      label: detailLabel(field),
      now: detailText(now, field, unit.value),
      nowFrom: sourceLabel(s.book.sources?.[field]),
      then: detailText(s.suggested, field, unit.value),
      thenFrom: sourceLabel(s.suggested.sources?.[field]),
    }))
  }),
  ...papers.value.flatMap((p) => {
    const now = documentToInput(p.document)
    return (p.fields ?? []).map((field) => ({
      key: `${p.document.id}:${field}`,
      recordId: p.document.id,
      record: p.document.title,
      field,
      label: paperDetailLabel(field, now.kind),
      now: paperDetailText(now, field),
      nowFrom: '',
      then: paperDetailText(p.suggested, field),
      thenFrom: 'Crossref',
    }))
  }),
])
const row = (d: unknown) => d as Row

const taken = ref(new Set<string>())
const busy = ref(false)
watch(open, (v) => {
  if (v) taken.value = new Set()
})

function take(key: string, on: boolean) {
  const next = new Set(taken.value)
  if (on) next.add(key)
  else next.delete(key)
  taken.value = next
}

function copyField<T, K extends keyof T>(to: T, from: T, k: K) {
  to[k] = from[k]
}

const saveLabel = computed(() =>
  taken.value.size ? `Make ${count(taken.value.size, 'change')}` : 'Make the changes',
)

// Each record taken from saves on its own; one that fails stops the rest, and says why.
async function save() {
  busy.value = true
  const saved: Book[] = []
  const savedPapers: Document[] = []
  let made = 0
  for (const s of books.value) {
    const fields = (s.fields ?? []).filter((f) => taken.value.has(`${s.book.id}:${f}`))
    if (!fields.length) continue
    const input = bookToInput(s.book)
    const sources = { ...(input.sources ?? {}) }
    for (const field of fields) {
      copyField<BookInput, typeof field>(input, s.suggested, field)
      // The value is the source's, so the source is named; a later lookup only fills blanks.
      const from = s.suggested.sources?.[field]
      if (from) sources[field] = from
    }
    input.sources = sources
    const { data, error } = await api.PUT('/books/{id}', {
      params: { path: { id: s.book.id } },
      body: input,
    })
    if (error) {
      notice.failed(error)
      break
    }
    saved.push(data)
    made += fields.length
  }
  for (const p of papers.value) {
    const fields = (p.fields ?? []).filter((f) => taken.value.has(`${p.document.id}:${f}`))
    if (!fields.length) continue
    const input = documentToInput(p.document)
    for (const field of fields) copyField<DocumentInput, typeof field>(input, p.suggested, field)
    const { data, error } = await api.PUT('/documents/{id}', {
      params: { path: { id: p.document.id } },
      body: input,
    })
    if (error) {
      notice.failed(error)
      break
    }
    savedPapers.push(data)
    made += fields.length
  }
  busy.value = false
  const records = saved.length + savedPapers.length
  if (!records) return
  notice.done(
    many.value
      ? `Made ${count(made, 'change')} to ${count(records, noun.value)}`
      : `Made ${count(made, 'change')}`,
  )
  open.value = false
  if (saved.length) emit('saved', saved)
  if (savedPapers.length) emit('savedPapers', savedPapers)
}
</script>

<template>
  <Dialog v-model:visible="open" modal header="What the sources say now" class="w-full max-w-3xl">
    <p class="prose-text mb-4">
      Blanks are filled. These details differ from what
      {{ many ? `the ${noun}s have` : `the ${noun} has` }}: switch on the ones the sources have
      right, and the rest stay as they are.
    </p>
    <DataTable
      :value="rows"
      data-key="key"
      :row-group-mode="many ? 'subheader' : undefined"
      group-rows-by="recordId"
      :pt="{ rowGroupHeaderCell: { colspan: 2 } }"
    >
      <!-- A record's name heads its rows across both columns: PrimeVue spans one fewer, for a
           grouping column this table does not show. -->
      <template #groupheader="{ data }">
        <span class="font-semibold">{{ row(data).record }}</span>
      </template>
      <Column header="Detail">
        <template #body="{ data }">
          <div class="font-semibold">{{ row(data).label }}</div>
          <dl class="change">
            <dt>Now</dt>
            <dd>
              <span class="change__value">{{ row(data).now }}</span>
              <span v-if="row(data).nowFrom" class="text-sm text-ink-muted">
                from {{ row(data).nowFrom }}</span
              >
            </dd>
            <dt>Suggested</dt>
            <dd>
              <span class="change__value">{{ row(data).then }}</span>
              <span v-if="row(data).thenFrom" class="text-sm text-ink-muted">
                from {{ row(data).thenFrom }}</span
              >
            </dd>
          </dl>
        </template>
      </Column>
      <Column header="Take it">
        <template #body="{ data }">
          <ToggleSwitch
            :model-value="taken.has(row(data).key)"
            :aria-label="
              many
                ? `Take the suggested ${row(data).label.toLowerCase()} of ${row(data).record}`
                : `Take the suggested ${row(data).label.toLowerCase()}`
            "
            @update:model-value="take(row(data).key, $event)"
          />
        </template>
      </Column>
    </DataTable>
    <template #footer>
      <Button
        type="button"
        severity="secondary"
        :label="many ? `Keep the ${noun}s as they are` : `Keep the ${noun} as it is`"
        @click="open = false"
      />
      <Button
        type="button"
        :label="saveLabel"
        :disabled="!taken.size"
        :loading="busy"
        @click="save"
      />
    </template>
  </Dialog>
</template>

<style scoped>
.change {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.25rem 0.75rem;
  margin-top: 0.25rem;
}

.change dt {
  color: var(--color-ink-muted);
}

/* A description is prose: kept to a readable measure. */
.change__value {
  display: block;
  max-width: 70ch;
  white-space: pre-line;
}
</style>
