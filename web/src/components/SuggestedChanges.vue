<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import { storeToRefs } from 'pinia'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import SelectButton from 'primevue/selectbutton'
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
import FormField from '@/components/FormField.vue'

// What a lookup the owner asked for would change, for one book or paper or many: each detail
// the sources now give otherwise, beside what it has, taken or left one by one. Nothing
// changes until the owner says. Books come as `suggestions`, papers as `papers`. A book's detail
// the owner keeps is declined, and the server does not offer that value again; one left
// undecided waits on the page of suggested changes.
const props = defineProps<{ suggestions?: Suggestions[]; papers?: DocumentSuggestions[] }>()
const emit = defineEmits<{ saved: [books: Book[]]; savedPapers: [papers: Document[]] }>()
const open = defineModel<boolean>('visible', { required: true })
const notice = useNotice()
const id = useId()
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
// Only a book's declines are kept; a paper looked up again offers the same again.
const remembers = computed(() => !papers.value.length)
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
      nowFrom: sourceLabel(s.book.sources[field]),
      then: detailText(s.suggested, field, unit.value),
      thenFrom: sourceLabel(s.suggested.sources[field]),
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

// Every detail is the owner's to decide: taken, kept, or left for later. Nothing is chosen for
// them, and only what they chose is saved.
type Choice = 'take' | 'keep'
const choices: { label: string; value: Choice }[] = [
  { label: 'Take it', value: 'take' },
  { label: 'Keep', value: 'keep' },
]
const decided = ref(new Map<string, Choice>())
const busy = ref(false)
watch(open, (v) => {
  if (v) {
    decided.value = new Map()
  }
})

function decide(key: string, choice: Choice | null) {
  const next = new Map(decided.value)
  if (choice) {
    next.set(key, choice)
  } else {
    next.delete(key)
  }
  decided.value = next
}

// Many to go through, the owner may decide every one alike, then change those they would not.
const everyOne = computed<Choice | null>({
  get: () => {
    const all = new Set(rows.value.map((r) => decided.value.get(r.key)))
    const [only] = [...all]
    return all.size === 1 && only ? only : null
  },
  set: (choice) => (decided.value = new Map(choice ? rows.value.map((r) => [r.key, choice]) : [])),
})

function copyField<T>(to: T, from: T, k: keyof T) {
  to[k] = from[k]
}

const saveLabel = computed(() =>
  decided.value.size ? `Save ${count(decided.value.size, 'decision')}` : 'Save the decisions',
)

// The details of a book kept as they are: what the sources give for them is not offered again.
async function decline(
  s: Suggestions,
  fields: NonNullable<Suggestions['fields']>,
): Promise<boolean> {
  if (!fields.length) {
    return true
  }
  const { error } = await api.POST('/books/{id}/declined-suggestions', {
    params: { path: { id: s.book.id } },
    body: { fields },
  })
  if (error) {
    notice.failed(error)
  }
  return !error
}

const as = (n: number) => (n === 1 ? 'it was' : 'they were')

// Each record decided saves on its own; one that fails stops the rest, and says why.
async function save() {
  busy.value = true
  const chose = (id: number, field: string) => decided.value.get(`${id}:${field}`)
  const saved: Book[] = []
  const savedPapers: Document[] = []
  let made = 0
  let kept = 0
  for (const s of books.value) {
    const fields = (s.fields ?? []).filter((f) => chose(s.book.id, f) === 'take')
    const keep = (s.fields ?? []).filter((f) => chose(s.book.id, f) === 'keep')
    if (!(await decline(s, keep))) {
      break
    }
    kept += keep.length
    if (!fields.length) {
      continue
    }
    const input = bookToInput(s.book)
    const sources = { ...input.sources }
    for (const field of fields) {
      copyField<BookInput>(input, s.suggested, field)
      // The value is the source's, so the source is named; a later lookup only fills blanks.
      const from = s.suggested.sources[field]
      if (from) {
        sources[field] = from
      }
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
    const fields = (p.fields ?? []).filter((f) => chose(p.document.id, f) === 'take')
    // A paper's detail kept is simply left: nothing is sent for it.
    kept += (p.fields ?? []).filter((f) => chose(p.document.id, f) === 'keep').length
    if (!fields.length) {
      continue
    }
    const input = documentToInput(p.document)
    for (const field of fields) {
      copyField<DocumentInput>(input, p.suggested, field)
    }
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
  if (!records && !kept) {
    return
  }
  const left = rows.value.length - decided.value.size
  notice.done(
    [
      records && many.value
        ? `Made ${count(made, 'change')} to ${count(records, noun.value)}.`
        : '',
      records && !many.value ? `Made ${count(made, 'change')}.` : '',
      kept ? `Kept ${count(kept, 'detail')} as ${as(kept)}.` : '',
      left && remembers.value
        ? `${count(left, 'detail')} left to decide in Suggested changes.`
        : '',
    ]
      .filter(Boolean)
      .join(' '),
  )
  open.value = false
  if (saved.length) {
    emit('saved', saved)
  }
  if (savedPapers.length) {
    emit('savedPapers', savedPapers)
  }
}
</script>

<template>
  <Dialog v-model:visible="open" modal header="What the sources say now" class="w-full max-w-3xl">
    <p class="prose-text mb-4">
      Blanks are filled. These details differ from what
      {{ many ? `the ${noun}s have` : `the ${noun} has` }}. For each one, take what the sources
      give, or keep the {{ noun }}'s own<template v-if="remembers"
        >: a detail you keep is not offered again unless a source gives something else. What you
        leave undecided waits in Suggested changes.</template
      ><template v-else>. What you leave stays as it is.</template>
    </p>
    <FormField
      v-if="rows.length > 1"
      label="Every one"
      :for="`${id}-every`"
      hint="Then change the ones you decide otherwise."
      class="mb-4"
    >
      <SelectButton
        :id="`${id}-every`"
        v-model="everyOne"
        :options="choices"
        option-label="label"
        option-value="value"
        :aria-labelledby="`${id}-every-label`"
      />
    </FormField>
    <DataTable
      :value="rows"
      data-key="key"
      :row-group-mode="many ? 'subheader' : undefined"
      group-rows-by="recordId"
      :pt="{ rowGroupHeaderCell: { colspan: 1 } }"
    >
      <!-- A record's name heads its rows: PrimeVue spans one column fewer than there are, for a
           grouping column this table does not show. -->
      <template #groupheader="{ data }">
        <span class="font-semibold">{{ row(data).record }}</span>
      </template>
      <Column header="Detail">
        <template #body="{ data }">
          <div :id="`${id}-${row(data).key}`" class="font-semibold">
            {{ row(data).label }}<span v-if="many" class="sr-only"> of {{ row(data).record }}</span>
          </div>
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
          <SelectButton
            :model-value="decided.get(row(data).key) ?? null"
            :options="choices"
            option-label="label"
            option-value="value"
            :aria-labelledby="`${id}-${row(data).key}`"
            class="mt-2"
            @update:model-value="decide(row(data).key, $event)"
          />
        </template>
      </Column>
    </DataTable>
    <template #footer>
      <Button
        type="button"
        severity="secondary"
        :label="remembers ? 'Decide later' : 'Close'"
        :disabled="busy"
        @click="open = false"
      />
      <Button
        type="button"
        :label="saveLabel"
        :disabled="!decided.size"
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
