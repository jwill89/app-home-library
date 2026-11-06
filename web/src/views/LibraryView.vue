<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import IconField from 'primevue/iconfield'
import InputIcon from 'primevue/inputicon'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import {
  faListCheck,
  faShelves,
  faMagnifyingGlass,
  faPlus,
  faTags,
} from '@fortawesome/pro-regular-svg-icons'
import { api, type BookRow } from '@/api/client'
import {
  categories,
  count,
  day,
  formatLabel,
  formats,
  readStatuses,
  seriesLine,
} from '@/app/format'
import { useFeed } from '@/app/feed'
import { ALL, useLibrary } from '@/app/library'
import { useQueue } from '@/app/queue'
import { useSession } from '@/app/session'
import BookCover from '@/components/BookCover.vue'
import LibraryTabs from '@/components/LibraryTabs.vue'
import PageHeader from '@/components/PageHeader.vue'
import PagePanel from '@/components/PagePanel.vue'
import QueueProgress from '@/components/QueueProgress.vue'
import ReadStatusChoice from '@/components/ReadStatusChoice.vue'

const library = useLibrary()
const {
  filtered,
  books,
  search,
  section,
  status,
  format,
  category,
  first,
  pageRows,
  sortField,
  sortOrder,
  sections,
  loaded,
  offlineSince,
  failed,
} = storeToRefs(library)
const router = useRouter()
const session = useSession()
// While the background lookup works through an import, the list follows it.
// The list keeps up with the lookup's books by itself, from the change feed.
const { queue, progress } = useQueue(undefined, session.isOwner)

// Full sheets of labels waiting, which the Labels button says, so the owner knows to print.
const readySheets = ref(0)
async function loadSheets() {
  const { data } = await api.GET('/label-sheets')
  readySheets.value = (data?.sheets ?? []).reduce(
    (n, s) => n + (s.enabled ? Math.floor(s.waiting / s.perSheet) : 0),
    0,
  )
}
useFeed(['label'], loadSheets)
onMounted(() => {
  if (!loaded.value) void library.load()
  if (session.isOwner) void loadSheets()
})

const summary = computed(() => {
  const n = books.value.length
  const reading = books.value.filter((r) => r.readStatus === 'reading').length
  const parts = [count(n, 'book')]
  if (reading) parts.push(`${reading} being read`)
  return parts.join(' · ')
})

const sectionOptions = computed(() => [
  { value: ALL, label: 'Every section' },
  ...sections.value.map((s) => ({ value: s, label: s })),
])
const statusOptions = [{ value: ALL, label: 'Any status' }, ...readStatuses]
const formatOptions = [{ value: ALL, label: 'Any format' }, ...formats.filter((f) => f.value)]

// Every category a book here is in, with how many, and the one chosen though it has none now.
const categoryTabs = computed(() => {
  const counts = new Map<string, number>()
  for (const r of books.value) counts.set(r.category, (counts.get(r.category) ?? 0) + 1)
  return [
    { value: ALL, label: 'All', count: books.value.length },
    ...categories
      .filter((c) => counts.has(c.value) || category.value === c.value)
      .map((c) => ({ value: c.value, label: c.plural, count: counts.get(c.value) ?? 0 })),
  ]
})

function open(row: BookRow) {
  void router.push(`/books/${row.id}`)
}
</script>

<template>
  <div class="mx-auto max-w-page">
    <LibraryTabs />
    <PageHeader title="Library" :subtitle="loaded ? summary : ''">
      <RouterLink v-if="session.isOwner" v-slot="{ navigate }" to="/shelves" custom>
        <Button type="button" severity="secondary" @click="navigate"
          ><FontAwesomeIcon :icon="faShelves" /> Shelves</Button
        >
      </RouterLink>
      <RouterLink v-if="session.isOwner" v-slot="{ navigate }" to="/gaps" custom>
        <Button type="button" severity="secondary" @click="navigate"
          ><FontAwesomeIcon :icon="faListCheck" /> Missing details</Button
        >
      </RouterLink>
      <RouterLink v-if="session.isOwner" v-slot="{ navigate }" to="/labels" custom>
        <Button type="button" severity="secondary" @click="navigate"
          ><FontAwesomeIcon :icon="faTags" /> Labels<template v-if="readySheets">
            ({{ count(readySheets, 'sheet') }} ready)</template
          ></Button
        >
      </RouterLink>
      <RouterLink v-if="session.isOwner" v-slot="{ navigate }" to="/add" custom>
        <Button type="button" @click="navigate"
          ><FontAwesomeIcon :icon="faPlus" /> Add a book</Button
        >
      </RouterLink>
    </PageHeader>

    <div v-if="failed" class="mb-4 flex flex-wrap items-center gap-3" role="alert">
      <p class="prose-text font-semibold text-status-danger">
        The library could not be read: {{ failed }}
      </p>
      <Button type="button" severity="secondary" label="Try again" @click="library.load()" />
    </div>

    <p v-if="offlineSince" class="prose-text mb-4 font-semibold" role="status">
      No connection: this is the list as this device kept it {{ day(offlineSince) }}.
    </p>

    <PagePanel v-if="queue.pending > 0" class="mb-4">
      <QueueProgress v-bind="queue" :progress="progress" />
    </PagePanel>

    <!-- One list of books, by kind: each category the library has, with how many. -->
    <div role="group" aria-label="Category" class="mb-4 flex flex-wrap gap-2">
      <Button
        v-for="c in categoryTabs"
        :key="c.value"
        type="button"
        :severity="category === c.value ? undefined : 'secondary'"
        :text="category !== c.value"
        :aria-pressed="category === c.value"
        @click="category = c.value"
        >{{ c.label }} <span class="tabular">{{ c.count }}</span></Button
      >
    </div>

    <div class="mb-4 grid gap-2 md:grid-cols-[1fr_auto_auto_auto]">
      <IconField>
        <InputIcon><FontAwesomeIcon :icon="faMagnifyingGlass" /></InputIcon>
        <InputText
          v-model="search"
          type="search"
          placeholder="Title, author, subject, series, ISBN, Dewey, barcode"
          class="w-full"
          aria-label="Search the library"
        />
      </IconField>
      <!-- Sections are the owner's to make; with none, there is nothing to filter by. -->
      <Select
        v-if="sections.length"
        v-model="section"
        :options="sectionOptions"
        option-label="label"
        option-value="value"
        aria-label="Section"
      />
      <Select
        v-model="status"
        :options="statusOptions"
        option-label="label"
        option-value="value"
        aria-label="Reading status"
      />
      <Select
        v-model="format"
        :options="formatOptions"
        option-label="label"
        option-value="value"
        aria-label="Format"
      />
    </div>

    <DataTable
      v-model:first="first"
      v-model:rows="pageRows"
      v-model:sort-field="sortField"
      v-model:sort-order="sortOrder"
      :value="filtered"
      data-key="id"
      paginator
      :rows-per-page-options="[25, 50, 100, 250]"
      removable-sort
      row-hover
      class="library"
      :loading="!loaded"
      @row-click="open($event.data as BookRow)"
    >
      <template #empty>
        <p class="py-6 text-center text-ink-muted">
          {{
            books.length
              ? 'Nothing matches. Clear the search or a filter.'
              : session.isOwner
                ? 'No books yet. Add one, or import from Bookshelf.'
                : 'No books here yet.'
          }}
        </p>
      </template>
      <Column header="Cover" header-class="column-unlabelled w-16">
        <template #body="{ data }">
          <BookCover
            :cover="(data as BookRow).cover"
            :title="(data as BookRow).title"
            :width="48"
          />
        </template>
      </Column>
      <Column field="title" header="Title" sortable>
        <template #body="{ data }">
          <RouterLink :to="`/books/${(data as BookRow).id}`" class="row-link" @click.stop>
            {{ (data as BookRow).title }}
          </RouterLink>
          <div v-if="(data as BookRow).subtitle" class="text-sm text-ink-muted">
            {{ (data as BookRow).subtitle }}
          </div>
          <div class="text-sm text-ink-muted md:hidden">{{ (data as BookRow).authors }}</div>
          <div v-if="(data as BookRow).series" class="text-sm text-ink-muted">
            {{ seriesLine((data as BookRow).series, (data as BookRow).seriesNumber) }}
          </div>
        </template>
      </Column>
      <Column
        field="authors"
        header="Author"
        sortable
        header-class="max-md:hidden"
        body-class="max-md:hidden"
      />
      <Column
        field="format"
        header="Format"
        sortable
        header-class="max-lg:hidden"
        body-class="max-lg:hidden"
      >
        <template #body="{ data }">{{
          (data as BookRow).format ? formatLabel((data as BookRow).format) : ''
        }}</template>
      </Column>
      <Column
        field="ddc"
        header="Dewey"
        sortable
        header-class="max-md:hidden"
        body-class="max-md:hidden font-mono tabular"
      />
      <Column
        field="readStatus"
        header="Status"
        sortable
        :header-class="session.isOwner ? '' : 'max-sm:hidden'"
        :body-class="session.isOwner ? '' : 'max-sm:hidden'"
      >
        <template #body="{ data }">
          <ReadStatusChoice :book="data as BookRow" />
        </template>
      </Column>
      <Column
        v-if="sections.length"
        field="sections"
        header="Section"
        sortable
        header-class="max-xl:hidden"
        body-class="max-xl:hidden"
      />
    </DataTable>
  </div>
</template>

<style scoped>
.library :deep(tbody tr) {
  cursor: pointer;
}

/* On a phone a row holds a cover, the title, and the status: closer cells fit all three. */
@media (width < 40rem) {
  .library :deep(th),
  .library :deep(td) {
    padding-inline: 0.5rem;
  }
}
</style>
