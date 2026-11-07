<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import IconField from 'primevue/iconfield'
import InputIcon from 'primevue/inputicon'
import InputText from 'primevue/inputtext'
import {
  faArrowsRotate,
  faMagnifyingGlass,
  faPlus,
} from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import { api, type DocumentRow, type DocumentSuggestions } from '@/api/client'
import { useFeed } from '@/app/feed'
import { count, documentKindLabel } from '@/app/format'
import { useNotice } from '@/app/problem'
import { useSession } from '@/app/session'
import LibraryTabs from '@/components/LibraryTabs.vue'
import PageHeader from '@/components/PageHeader.vue'
import SuggestedChanges from '@/components/SuggestedChanges.vue'

const router = useRouter()
const notice = useNotice()
const session = useSession()

const rows = ref<DocumentRow[]>([])
const loaded = ref(false)
const search = ref('')
const searched = ref('')

// The words inside the files live on the server, so a search asks it, a moment after typing
// stops.
let timer: ReturnType<typeof setTimeout> | undefined
async function load() {
  const q = search.value.trim()
  const { data, error } = await api.GET('/documents', { params: { query: q ? { q } : {} } })
  if (error) {
    notice.failed(error)
    return
  }
  rows.value = data.documents ?? []
  searched.value = q
  loaded.value = true
}
onMounted(load)
// A paper added, changed, or read by the text service shows in the list and its search.
useFeed(['document'], () => load())
watch(search, () => {
  clearTimeout(timer)
  timer = setTimeout(() => void load(), 300)
})

// The list sorts by any column; by title until the owner chooses another. A search's answers
// come best match first, until a column is chosen.
const sortField = ref<string | undefined>('title')
const sortOrder = ref<1 | -1 | 0 | undefined>(1)
watch(searched, (now, before) => {
  if (now && !before) {
    sortField.value = undefined
  } else if (!now && sortField.value === undefined) {
    ;[sortField.value, sortOrder.value] = ['title', 1]
  }
})
// The words the Kind and Year columns sort by, beside each row.
const shown = computed(() =>
  rows.value.map((r) => ({
    ...r,
    kindLabel: documentKindLabel(r.kind),
    year: r.published.slice(0, 4),
  })),
)
type Row = (typeof shown.value)[number]
const row = (d: unknown) => d as Row

// Every paper with a DOI asked about again, one after another: blanks filled, and what
// Crossref gives otherwise offered together at the end.
const looking = ref(false)
const stopping = ref(false)
const progress = ref('')
const suggestions = ref<DocumentSuggestions[]>([])
const suggesting = ref(false)
const lookable = computed(() => rows.value.filter((r) => r.doi))
async function lookUpAll() {
  const list = lookable.value
  looking.value = true
  stopping.value = false
  const found: DocumentSuggestions[] = []
  const failed: string[] = []
  let asked = 0
  for (const d of list) {
    // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition -- the Stop button sets it during the await below
    if (stopping.value) {
      break
    }
    progress.value = `Looking up ${asked + 1} of ${list.length}: ${d.title}`
    const { data, error } = await api.POST('/documents/{id}/refresh', {
      params: { path: { id: d.id } },
    })
    asked++
    if (error) {
      failed.push(d.title)
    } else if (data.fields?.length) {
      found.push(data)
    }
  }
  looking.value = false
  const said = [
    `Looked up ${count(asked, 'paper')}${asked < list.length ? ` of ${list.length}` : ''}.`,
    found.length
      ? `${count(found.length, 'has', 'have')} details Crossref gives otherwise.`
      : 'Blanks are filled where Crossref knew more; nothing else differs.',
    failed.length ? `Crossref did not answer for ${failed.join(', ')}: try again later.` : '',
  ]
  progress.value = said.filter(Boolean).join(' ')
  if (found.length) {
    suggestions.value = found
    suggesting.value = true
  }
}

const summary = computed(() => {
  if (!loaded.value) {
    return ''
  }
  const n = rows.value.length
  if (searched.value) {
    return `${count(n, 'paper has', 'papers have')} “${searched.value}”`
  }
  return count(n, 'paper')
})

/** A snippet's text, with the matches the server marked between \x02 and \x03. */
function marked(snippet: string): { text: string; match: boolean }[] {
  return snippet
    .split(/(\u0002[^\u0003]*\u0003)/)
    .filter(Boolean)
    .map((part) =>
      part.startsWith('\u0002')
        ? { text: part.slice(1, -1), match: true }
        : { text: part, match: false },
    )
}

const byline = (r: DocumentRow) =>
  [(r.authors ?? []).join(', '), r.container, r.published.slice(0, 4)].filter(Boolean).join(' · ')

function open(r: DocumentRow) {
  void router.push(`/papers/${r.id}`)
}
</script>

<template>
  <div class="mx-auto max-w-page">
    <LibraryTabs />
    <PageHeader title="Papers" :subtitle="summary">
      <template v-if="session.isOwner">
        <RouterLink v-slot="{ navigate }" to="/papers/new" custom>
          <Button type="button" @click="navigate"
            ><FontAwesomeIcon :icon="faPlus" /> Add a paper</Button
          >
        </RouterLink>
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
      </template>
    </PageHeader>

    <p v-if="progress" role="status" class="prose-text mb-4">{{ progress }}</p>

    <IconField class="mb-4">
      <InputIcon><FontAwesomeIcon :icon="faMagnifyingGlass" /></InputIcon>
      <InputText
        v-model="search"
        type="search"
        :placeholder="
          session.isOwner
            ? 'Title, author, journal, notes, or words in the files'
            : 'Title, author, journal, subject, or abstract'
        "
        class="w-full"
        aria-label="Search the papers"
      />
    </IconField>

    <DataTable
      v-model:sort-field="sortField"
      v-model:sort-order="sortOrder"
      :value="shown"
      data-key="id"
      :loading="!loaded"
      :paginator="shown.length > 50"
      :rows="50"
      removable-sort
      row-hover
      class="documents"
      @row-click="open($event.data as DocumentRow)"
    >
      <template #empty>
        <p class="py-6 text-center text-ink-muted">
          {{
            searched
              ? 'Nothing has those words. Try fewer, or other ones.'
              : session.isOwner
                ? 'No papers yet. Add an article, a handout, or a syllabus, with a PDF or photos of its pages.'
                : 'No papers are shared here.'
          }}
        </p>
      </template>
      <Column header="Paper" field="title" sortable>
        <template #body="{ data }">
          <RouterLink :to="`/papers/${(data as DocumentRow).id}`" class="row-link" @click.stop>
            {{ (data as DocumentRow).title }}
          </RouterLink>
          <div v-if="byline(data as DocumentRow)" class="text-sm text-ink-muted">
            {{ byline(data as DocumentRow) }}
          </div>
          <div class="text-sm text-ink-muted md:hidden">
            {{ documentKindLabel((data as DocumentRow).kind) }}
          </div>
          <p v-if="(data as DocumentRow).snippet" class="snippet mt-1 text-sm">
            <template v-for="(part, i) in marked((data as DocumentRow).snippet)" :key="i"
              ><mark v-if="part.match">{{ part.text }}</mark
              ><template v-else>{{ part.text }}</template></template
            >
          </p>
        </template>
      </Column>
      <Column
        header="Kind"
        field="kindLabel"
        sortable
        header-class="max-md:hidden"
        body-class="max-md:hidden"
      >
        <template #body="{ data }">{{ row(data).kindLabel }}</template>
      </Column>
      <Column
        header="Year"
        field="published"
        sortable
        header-class="max-md:hidden"
        body-class="max-md:hidden tabular"
      >
        <template #body="{ data }">{{ row(data).year }}</template>
      </Column>
      <Column
        v-if="session.isOwner"
        field="course"
        header="Course"
        sortable
        header-class="max-lg:hidden"
        body-class="max-lg:hidden"
      />
      <Column
        header="Files"
        field="files"
        sortable
        header-class="max-sm:hidden"
        body-class="max-sm:hidden tabular"
      >
        <template #body="{ data }">
          {{ (data as DocumentRow).files }}
          <div v-if="(data as DocumentRow).reading" class="text-sm text-ink-muted">
            {{ (data as DocumentRow).reading }} being read
          </div>
        </template>
      </Column>
    </DataTable>

    <SuggestedChanges
      v-if="session.isOwner"
      v-model:visible="suggesting"
      :papers="suggestions"
      @saved-papers="load()"
    />
  </div>
</template>

<style scoped>
.documents :deep(tbody tr) {
  cursor: pointer;
}

.snippet {
  max-width: 70ch;
  line-height: 1.5;
}

/* A match is marked by weight and an underline, in the panel's own colors. */
.snippet mark {
  background: none;
  color: inherit;
  font-weight: 700;
  text-decoration: underline;
  text-underline-offset: 0.15em;
}
</style>
