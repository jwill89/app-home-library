<script setup lang="ts">
import { computed, onMounted, ref, useId } from 'vue'
import { storeToRefs } from 'pinia'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Select from 'primevue/select'
import { api, type BookRow, type FinishedRead, type ReadingStats } from '@/api/client'
import { calendarDate, count } from '@/app/format'
import { useLibrary } from '@/app/library'
import { useNotice } from '@/app/problem'
import BookCover from '@/components/BookCover.vue'
import FormField from '@/components/FormField.vue'
import LibraryTabs from '@/components/LibraryTabs.vue'
import PageHeader from '@/components/PageHeader.vue'
import PagePanel from '@/components/PagePanel.vue'

// What is being read, and what has been: the books and pages finished each year, a book read
// twice counted twice.
const library = useLibrary()
const { books, loaded: listLoaded } = storeToRefs(library)
const notice = useNotice()
const id = useId()

const stats = ref<ReadingStats | null>(null)
onMounted(async () => {
  if (!listLoaded.value) {
    void library.load()
  }
  const { data, error } = await api.GET('/reading')
  if (error) {
    notice.failed(error)
    return
  }
  stats.value = data
  year.value = data.years?.[0]?.year ?? ''
})

const now = computed(() => books.value.filter((r) => r.readStatus === 'reading'))
const year = ref('')
const years = computed(() =>
  (stats.value?.years ?? []).map((y) => ({ value: y.year, label: y.year })),
)
const inYear = computed(() =>
  (stats.value?.finished ?? []).filter((f) => f.finishedOn.startsWith(year.value)),
)
const summary = computed(() => {
  const y = stats.value?.years?.[0]
  return y ? `${count(y.books, 'book')} finished in ${y.year}` : ''
})
const book = (d: unknown) => d as BookRow
const read = (d: unknown) => d as FinishedRead
</script>

<template>
  <div class="mx-auto max-w-page">
    <LibraryTabs />
    <PageHeader title="Reading" :subtitle="summary" />

    <div class="flex flex-col gap-6">
      <PagePanel title="Reading now">
        <DataTable :value="now" data-key="id" :loading="!listLoaded">
          <template #empty><p class="text-ink-muted">Nothing is being read.</p></template>
          <Column header="Book">
            <template #body="{ data }">
              <div class="flex items-start gap-3">
                <BookCover :cover="book(data).cover" :title="book(data).title" :width="48" />
                <div class="min-w-0">
                  <RouterLink :to="`/books/${book(data).id}`" class="row-link">{{
                    book(data).title
                  }}</RouterLink>
                  <div class="text-sm text-ink-muted">{{ book(data).authors }}</div>
                </div>
              </div>
            </template>
          </Column>
        </DataTable>
      </PagePanel>

      <PagePanel title="Each year">
        <DataTable :value="stats?.years ?? []" data-key="year" :loading="!stats">
          <template #empty><p class="text-ink-muted">No book has been finished yet.</p></template>
          <Column field="year" header="Year" />
          <Column field="books" header="Books" />
          <Column header="Pages">
            <template #body="{ data }">{{
              (data as { pages: number }).pages.toLocaleString()
            }}</template>
          </Column>
        </DataTable>
      </PagePanel>

      <PagePanel v-if="years.length" title="Finished">
        <FormField label="Year" :for="`${id}-year`" class="mb-4 max-w-xs">
          <Select
            v-model="year"
            :input-id="`${id}-year`"
            :aria-labelledby="`${id}-year-label`"
            :options="years"
            option-label="label"
            option-value="value"
          />
        </FormField>
        <DataTable :value="inYear" :data-key="(r: FinishedRead) => `${r.bookId}-${r.finishedOn}`">
          <Column header="Book">
            <template #body="{ data }">
              <RouterLink :to="`/books/${read(data).bookId}`" class="row-link">{{
                read(data).title
              }}</RouterLink>
            </template>
          </Column>
          <Column header="Finished">
            <template #body="{ data }">{{ calendarDate(read(data).finishedOn) }}</template>
          </Column>
          <Column header="Pages" header-class="max-sm:hidden" body-class="max-sm:hidden">
            <template #body="{ data }">{{ read(data).pages || '' }}</template>
          </Column>
        </DataTable>
      </PagePanel>
    </div>
  </div>
</template>
