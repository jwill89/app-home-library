<script setup lang="ts">
import { computed, onMounted, ref, useId, watch } from 'vue'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Select from 'primevue/select'
import { faArrowsRotate } from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import { api, type BookGaps, type Suggestions } from '@/api/client'
import { useConfirmAction } from '@/app/confirm'
import { useFeed } from '@/app/feed'
import { count, detailChoices, detailLabel } from '@/app/format'
import { useNotice } from '@/app/problem'
import { useQueue } from '@/app/queue'
import { maxRefresh, refreshRows } from '@/app/rows'
import BookCover from '@/components/BookCover.vue'
import FormField from '@/components/FormField.vue'
import PageHeader from '@/components/PageHeader.vue'
import PagePanel from '@/components/PagePanel.vue'
import QueueProgress from '@/components/QueueProgress.vue'
import SuggestedChanges from '@/components/SuggestedChanges.vue'

// The books that lack a detail: a cover, a Dewey number, a size, and so on. Each can be filled
// by hand on its Edit page, or looked up again, one now or all in the background; what the
// background finds otherwise waits on the page of suggested changes.
const notice = useNotice()
const id = useId()

const books = ref<BookGaps[]>([])
const loaded = ref(false)
async function load() {
  const { data, error } = await api.GET('/gaps')
  if (error) {
    notice.failed(error)
    return
  }
  books.value = data.books ?? []
  loaded.value = true
}
onMounted(load)
// A book the background lookup fills, or one changed here or on another device, is asked about
// again alone, and its row alone changes.
async function refresh(refs: ReadonlySet<number>, all: boolean) {
  if (all || refs.size > maxRefresh) {
    return load()
  }
  const { data, error } = await api.GET('/gaps', { params: { query: { ids: [...refs] } } })
  if (!error) {
    refreshRows(books.value, refs, data.books ?? [], (b) => b.bookId)
  }
}
useFeed(['book'], refresh)
const { queue, total, progress } = useQueue()

const field = ref('any')
const fields = computed(() => detailChoices(books.value.map((b) => b.missing)))
// A detail no book lacks any longer shows every book again.
watch(fields, (now) => {
  if (!now.some((o) => o.value === field.value)) {
    field.value = 'any'
  }
})
const shown = computed(() =>
  field.value === 'any'
    ? books.value
    : books.value.filter((b) => (b.missing ?? []).some((m) => m === field.value)),
)
const summary = computed(() =>
  loaded.value ? `${count(books.value.length, 'book lacks', 'books lack')} something` : '',
)
const row = (d: unknown) => d as BookGaps
const lacks = (b: BookGaps) => (b.missing ?? []).map(detailLabel).join(', ')

const busy = ref(0)
const suggestions = ref<Suggestions[]>([])
const suggesting = ref(false)
async function lookUp(b: BookGaps) {
  busy.value = b.bookId
  const { data, error } = await api.POST('/books/{id}/refresh', {
    params: { path: { id: b.bookId } },
  })
  busy.value = 0
  if (error) {
    notice.failed(error)
    return
  }
  await refresh(new Set([b.bookId]), false)
  // What a source gives otherwise is offered, to take or leave, as on the book's own page.
  if (data.fields?.length) {
    suggestions.value = [data]
    suggesting.value = true
    return
  }
  const after = books.value.find((x) => x.bookId === b.bookId)
  notice.done(
    after ? `${b.title} still lacks ${lacks(after).toLowerCase()}` : `${b.title} is whole now`,
  )
}

async function lookUpAll() {
  const ids = shown.value.filter((b) => b.isbn13).map((b) => b.bookId)
  if (!ids.length) {
    return
  }
  const { data, error } = await api.POST('/enrichment', { body: { ids } })
  if (error) {
    notice.failed(error)
    return
  }
  notice.done(
    `${count(data.queued, 'book')} queued for the background lookup. What the sources give otherwise waits in Suggested changes.`,
  )
}

// The whole library, not only the books lacking what the list counts: a source may know more
// of any book now, a place published or a subject, than it did when the book was added.
const ask = useConfirmAction()
function lookUpEvery() {
  ask(
    {
      header: 'Look up every book again?',
      message:
        'Every book with an ISBN goes back into the background lookup, and each source is asked about it again. Blank details are filled at once. Wherever a source gives a detail otherwise, the change waits in Suggested changes for you to take or leave: nothing you typed or an import brought changes until you say. It takes a second or two a book, while you carry on; a book you add meanwhile is looked up first.',
      accept: 'Look them all up',
      reject: 'Not now',
    },
    async () => {
      const { data, error } = await api.POST('/enrichment', { body: { all: true } })
      if (error) {
        notice.failed(error)
        return
      }
      notice.done(
        `${count(data.queued, 'book')} queued for the background lookup. What the sources give otherwise waits in Suggested changes.`,
      )
    },
  )
}
</script>

<template>
  <div class="mx-auto max-w-page">
    <PageHeader
      title="Missing details"
      :subtitle="summary"
      :trail="[{ label: 'Library', to: '/' }]"
    >
      <Button
        type="button"
        severity="secondary"
        :disabled="!shown.some((b) => b.isbn13)"
        @click="lookUpAll"
        ><FontAwesomeIcon :icon="faArrowsRotate" /> Look these up again</Button
      >
      <Button type="button" severity="secondary" label="Look up every book" @click="lookUpEvery" />
      <RouterLink v-slot="{ navigate }" to="/suggestions" custom>
        <Button type="button" severity="secondary" label="Suggested changes" @click="navigate" />
      </RouterLink>
    </PageHeader>

    <PagePanel v-if="total > 0 && queue.pending > 0" title="Background lookup" class="mb-4">
      <QueueProgress v-bind="queue" :progress="progress" />
    </PagePanel>

    <FormField label="Show the books lacking" :for="`${id}-field`" class="mb-4 max-w-sm">
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
        <p class="py-6 text-center text-ink-muted">Every book has every detail.</p>
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
              <div class="text-sm">Lacks {{ lacks(row(data)).toLowerCase() }}</div>
            </div>
          </div>
        </template>
      </Column>
      <Column header="Actions" header-class="column-unlabelled">
        <template #body="{ data }">
          <div class="flex flex-col items-end gap-1 md:flex-row md:justify-end">
            <RouterLink v-slot="{ navigate }" :to="`/books/${row(data).bookId}/edit`" custom>
              <Button
                type="button"
                text
                label="Edit"
                :aria-label="`Fill in ${row(data).title} by hand`"
                @click="navigate"
              />
            </RouterLink>
            <Button
              v-if="row(data).isbn13"
              type="button"
              text
              label="Look up again"
              :aria-label="`Look ${row(data).title} up again`"
              :loading="busy === row(data).bookId"
              @click="lookUp(row(data))"
            />
          </div>
        </template>
      </Column>
    </DataTable>

    <SuggestedChanges
      v-model:visible="suggesting"
      :suggestions="suggestions"
      @saved="(saved) => refresh(new Set(saved.map((b) => b.id)), false)"
    />
  </div>
</template>
