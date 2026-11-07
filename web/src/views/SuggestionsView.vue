<script setup lang="ts">
import { computed, onMounted, ref, useId, watch } from 'vue'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Select from 'primevue/select'
import { api, type SuggestedBook, type Suggestions } from '@/api/client'
import { useFeed } from '@/app/feed'
import { count, detailChoices, detailLabel, listed } from '@/app/format'
import { useNotice } from '@/app/problem'
import { useQueue } from '@/app/queue'
import { maxRefresh, refreshRows } from '@/app/rows'
import BookCover from '@/components/BookCover.vue'
import FormField from '@/components/FormField.vue'
import PageHeader from '@/components/PageHeader.vue'
import PagePanel from '@/components/PagePanel.vue'
import QueueProgress from '@/components/QueueProgress.vue'
import SuggestedChanges from '@/components/SuggestedChanges.vue'

// Every book whose lookup, asked for by the owner, found details the sources give otherwise:
// each waits here until it is taken or kept as it is, a book or many at a time.
const notice = useNotice()
const id = useId()

const books = ref<SuggestedBook[]>([])
const loaded = ref(false)
async function load() {
  const { data, error } = await api.GET('/suggestions')
  if (error) {
    notice.failed(error)
    return
  }
  books.value = data.books ?? []
  loaded.value = true
}
onMounted(load)
// A book the background lookup reaches, and a change taken or kept here or on another device,
// is asked about again alone, and its row alone changes: nothing is worked out for the rest.
async function refresh(refs: ReadonlySet<number>, all: boolean) {
  if (all || refs.size > maxRefresh) {
    return load()
  }
  const { data, error } = await api.GET('/suggestions', { params: { query: { ids: [...refs] } } })
  if (!error) {
    refreshRows(books.value, refs, data.books ?? [], (b) => b.bookId)
  }
}
useFeed(['book'], refresh)
const { queue, total, progress } = useQueue()

type Field = NonNullable<SuggestedBook['fields']>[number]
const field = ref<Field | 'any'>('any')
const fields = computed(() => detailChoices(books.value.map((b) => b.fields)))
// A detail no book differs in any longer shows every book again.
watch(fields, (now) => {
  if (!now.some((o) => o.value === field.value)) {
    field.value = 'any'
  }
})
const shown = computed(() =>
  field.value === 'any'
    ? books.value
    : books.value.filter((b) => (b.fields ?? []).includes(field.value as Field)),
)
const summary = computed(() =>
  loaded.value
    ? `${count(books.value.length, 'book has', 'books have')} details the sources give otherwise`
    : '',
)
const row = (d: unknown) => d as SuggestedBook
const differs = (b: SuggestedBook) =>
  listed((b.fields ?? []).map((f) => detailLabel(f).toLowerCase()))

// A few books at a time, so one sitting is not the whole library: as each is decided it leaves
// the list, and the next come up.
const batch = 25
const reviewLabel = computed(() =>
  shown.value.length > batch ? `Review the first ${batch}` : 'Review them',
)
const suggestions = ref<Suggestions[]>([])
const suggesting = ref(false)
const opening = ref(0)
async function review(list: SuggestedBook[]) {
  opening.value = list.length === 1 ? (list[0]?.bookId ?? -1) : -1
  const answers = await Promise.all(
    list.map((b) => api.GET('/books/{id}/suggestions', { params: { path: { id: b.bookId } } })),
  )
  opening.value = 0
  const failed = answers.find((a) => a.error)?.error
  if (failed) {
    notice.failed(failed)
    return
  }
  // Shown by one detail, only that detail is offered: the book's others wait their turn.
  const only = field.value
  suggestions.value = answers
    .flatMap((a) => a.data ?? [])
    .map((s) => (only === 'any' ? s : { ...s, fields: (s.fields ?? []).filter((f) => f === only) }))
    .filter((s) => s.fields?.length)
  if (!suggestions.value.length) {
    await refresh(new Set(list.map((b) => b.bookId)), false)
    notice.done('Nothing is left to decide there: the books have changed since.')
    return
  }
  suggesting.value = true
}
// The books just decided are asked about again at once, without waiting for the feed.
watch(suggesting, (open) => {
  if (!open) {
    void refresh(new Set(suggestions.value.map((s) => s.book.id)), false)
  }
})
</script>

<template>
  <div class="mx-auto max-w-page">
    <PageHeader
      title="Suggested changes"
      :subtitle="summary"
      :trail="[
        { label: 'Library', to: '/' },
        { label: 'Missing details', to: '/gaps' },
      ]"
    >
      <Button
        v-if="shown.length"
        type="button"
        :label="reviewLabel"
        :loading="opening === -1"
        @click="review(shown.slice(0, batch))"
      />
    </PageHeader>

    <p class="prose-text mb-4">
      A lookup you ask for fills a book's blanks at once. Where the sources give a detail otherwise
      than the book has, the change waits here: take it, or keep the book as it is, and that value
      is not offered again unless a source gives something else.
    </p>

    <PagePanel v-if="total > 0 && queue.pending > 0" title="Background lookup" class="mb-4">
      <QueueProgress v-bind="queue" :progress="progress" />
    </PagePanel>

    <FormField
      v-if="books.length"
      label="Show the books that differ in"
      :for="`${id}-field`"
      class="mb-4 max-w-sm"
    >
      <Select
        v-model="field"
        :input-id="`${id}-field`"
        :aria-labelledby="`${id}-field-label`"
        :options="fields"
        option-label="label"
        option-value="value"
      />
    </FormField>

    <DataTable
      :value="shown"
      data-key="bookId"
      sort-field="title"
      :sort-order="1"
      :loading="!loaded"
      :paginator="shown.length > 50"
      :rows="50"
    >
      <template #empty>
        <p v-if="loaded" class="prose-text mx-auto py-6 text-center text-ink-muted">
          Nothing to decide. To ask the sources about every book again, choose Look up every book on
          <RouterLink to="/gaps" class="underline">Missing details</RouterLink>.
        </p>
      </template>
      <Column header="Book">
        <template #body="{ data }">
          <div class="flex items-start gap-3">
            <BookCover :cover="row(data).cover" :title="row(data).title" :width="48" />
            <div class="min-w-0">
              <RouterLink :to="`/books/${row(data).bookId}`" class="row-link">{{
                row(data).title
              }}</RouterLink>
              <div class="text-sm text-ink-muted">{{ row(data).authors }}</div>
              <div class="text-sm">Differs in {{ differs(row(data)) }}</div>
            </div>
          </div>
        </template>
      </Column>
      <Column header="Actions" header-class="column-unlabelled">
        <template #body="{ data }">
          <div class="flex justify-end">
            <Button
              type="button"
              text
              label="Review"
              :aria-label="`Review what the sources give for ${row(data).title}`"
              :loading="opening === row(data).bookId"
              @click="review([row(data)])"
            />
          </div>
        </template>
      </Column>
    </DataTable>

    <SuggestedChanges v-model:visible="suggesting" :suggestions="suggestions" />
  </div>
</template>
