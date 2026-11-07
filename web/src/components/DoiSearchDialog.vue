<script setup lang="ts">
import { ref, watch } from 'vue'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import { api, type DocProposal } from '@/api/client'
import { calendarDate, listed } from '@/app/format'
import { messageOf } from '@/app/problem'

// The search for a paper's DOI: the works Crossref matches to its title and first author, for
// the owner to choose the one it is. The form takes the DOI chosen and fills its blanks from it.
const props = defineProps<{ title: string; author: string }>()
const emit = defineEmits<{ chosen: [doi: string] }>()
const open = defineModel<boolean>('visible', { required: true })

const found = ref<DocProposal[]>([])
const searching = ref(false)
const problem = ref('')
watch(open, async (now) => {
  if (!now) {
    return
  }
  found.value = []
  problem.value = ''
  searching.value = true
  const { data, error } = await api.GET('/lookup/dois', {
    params: { query: { title: props.title, author: props.author } },
  })
  searching.value = false
  if (error) {
    problem.value = messageOf(error)
    return
  }
  found.value = data.candidates ?? []
})

const paper = (d: unknown) => d as DocProposal
// A long list of authors is cut short: the first three tell one paper from another.
const people = (names: string[] | null) =>
  (names ?? []).length > 3
    ? `${(names ?? []).slice(0, 3).join(', ')}, and others`
    : listed(names ?? [])

function choose(p: DocProposal) {
  open.value = false
  emit('chosen', p.doi)
}
</script>

<template>
  <Dialog v-model:visible="open" modal header="Find the DOI" class="w-full max-w-2xl">
    <p class="prose-text mb-4">
      The papers Crossref has closest to “{{ title }}”<template v-if="author">
        by {{ author }}</template
      >. Choose the one it is: its DOI fills in what the form still lacks.
    </p>
    <p v-if="searching" class="prose-text" role="status">Asking Crossref…</p>
    <p v-else-if="problem" class="prose-text font-semibold text-status-danger" role="alert">
      {{ problem }}
    </p>
    <DataTable v-else :value="found" data-key="doi">
      <template #empty>
        <p class="prose-text text-ink-muted">
          Crossref has nothing close to that title. Check it, or fill the details in yourself.
        </p>
      </template>
      <Column header="Paper">
        <template #body="{ data }">
          <div class="font-semibold">{{ paper(data).title }}</div>
          <div class="text-sm">{{ people(paper(data).authors) }}</div>
          <div class="text-sm text-ink-muted">
            {{
              [paper(data).container, calendarDate(paper(data).published)]
                .filter(Boolean)
                .join(' · ')
            }}
          </div>
          <!-- A DOI is one long word: it breaks anywhere, so the choice beside it stays in view. -->
          <div class="font-mono text-sm break-all">{{ paper(data).doi }}</div>
        </template>
      </Column>
      <Column header="Actions" header-class="column-unlabelled">
        <template #body="{ data }">
          <div class="flex justify-end">
            <Button
              type="button"
              label="This one"
              :aria-label="`Use ${paper(data).title}, ${paper(data).doi}`"
              @click="choose(paper(data))"
            />
          </div>
        </template>
      </Column>
    </DataTable>
    <template #footer>
      <Button type="button" severity="secondary" label="None of these" @click="open = false" />
    </template>
  </Dialog>
</template>
