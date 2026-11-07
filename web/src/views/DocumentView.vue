<script setup lang="ts">
import { computed, onMounted, ref, useId, useTemplateRef, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import Textarea from 'primevue/textarea'
import {
  faArrowsRotate,
  faFileArrowUp,
  faFilePdf,
  faMagnifyingGlass,
  faPen,
  faTrash,
} from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import {
  api,
  fileURL,
  type Document,
  type DocumentFile,
  type DocumentSuggestions,
} from '@/api/client'
import { useConfirmAction } from '@/app/confirm'
import { addDocumentFile, documentCite, documentFileTypes } from '@/app/documents'
import {
  calendarDate,
  containerLabel,
  documentKindLabel,
  fileSize,
  paperDetailLabel,
  textStatusLabel,
} from '@/app/format'
import { useFeed } from '@/app/feed'
import { goBack } from '@/app/navigation'
import { useNotice } from '@/app/problem'
import { useSession } from '@/app/session'
import CitationBox from '@/components/CitationBox.vue'
import CoverPhoto from '@/components/CoverPhoto.vue'
import FindCopyDialog from '@/components/FindCopyDialog.vue'
import FormField from '@/components/FormField.vue'
import PagePanel from '@/components/PagePanel.vue'
import SuggestedChanges from '@/components/SuggestedChanges.vue'
import PageHeader from '@/components/PageHeader.vue'
import SubjectList from '@/components/SubjectList.vue'

const props = defineProps<{ id: number }>()
const router = useRouter()
const ask = useConfirmAction()
const notice = useNotice()
const uid = useId()
// A visitor reads a paper their link shows and opens its files; the rest is the owner's.
const { isOwner: owner } = useSession()

const doc = ref<Document | null>(null)
const busy = ref(false)
const path = computed(() => ({ params: { path: { id: props.id } } }))

async function load() {
  const { data, error } = await api.GET('/documents/{id}', path.value)
  if (error) {
    notice.failed(error)
    return
  }
  doc.value = data
  document.title = `${data.title} · Home Library`
}

// The text service reads a file in a process of its own; what it read, and any change made
// on another device, reaches the page on the change feed.
useFeed(['document'], (refs, all) => (all || refs.has(props.id)) && load())
onMounted(load)
watch(() => props.id, load)

const details = computed(() => {
  const d = doc.value
  if (!d) {
    return []
  }
  const where = containerLabel(d.kind)
  const numbers = [
    d.volume && `volume ${d.volume}`,
    d.issue && `issue ${d.issue}`,
    d.pages && `pages ${d.pages}`,
  ]
    .filter(Boolean)
    .join(', ')
  return [
    { label: 'Kind', value: documentKindLabel(d.kind) },
    { label: where, value: d.container },
    { label: 'Edited by', value: (d.editors ?? []).join(', ') },
    { label: 'Where in it', value: numbers },
    { label: 'Date', value: calendarDate(d.published) },
    { label: 'Publisher', value: d.publisher },
  ].filter((r) => r.value)
})

const uploads = useTemplateRef<HTMLInputElement>('uploads')

// Where else it can be read; an open copy added from there is read for its text like any file.
const finding = ref(false)
function foundCopy(d: Document) {
  doc.value = d
}

// Its DOI asked about again: the blanks are filled, and what Crossref gives otherwise offered.
const refreshing = ref(false)
const suggestions = ref<DocumentSuggestions[]>([])
const suggesting = ref(false)
async function lookUpAgain() {
  refreshing.value = true
  const { data, error } = await api.POST('/documents/{id}/refresh', path.value)
  refreshing.value = false
  if (error) {
    notice.failed(error)
    return
  }
  doc.value = data.document
  if (data.fields?.length) {
    suggestions.value = [data]
    suggesting.value = true
    return
  }
  const filled = (data.filled ?? []).map((f) =>
    f === 'doi' ? 'DOI' : paperDetailLabel(f, data.document.kind).toLowerCase(),
  )
  notice.done(
    filled.length
      ? `Filled in from the DOI: ${filled.join(', ')}.`
      : 'Looked up again. Nothing more to fill, and nothing differs.',
  )
}

async function upload(file: Blob, name: string) {
  busy.value = true
  const { data, error } = await addDocumentFile(props.id, file, name)
  busy.value = false
  if (error) {
    notice.failed(error)
    return
  }
  doc.value = data
}

async function photographed(blob: Blob) {
  const n = (doc.value?.files ?? []).filter((f) => f.contentType.startsWith('image/')).length + 1
  await upload(blob, `Page ${n}.jpg`)
  notice.done(`Page ${n} added. The text in it is read in the background.`)
}

async function chosen(e: Event) {
  const input = e.target as HTMLInputElement
  const picked = [...(input.files ?? [])]
  input.value = ''
  for (const f of picked) {
    await upload(f, f.name)
  }
  if (picked.length) {
    notice.done(picked.length === 1 ? `${picked[0]?.name} added` : `${picked.length} files added`)
  }
}

function removeFile(f: DocumentFile) {
  ask(
    {
      header: `Remove ${f.name}?`,
      message: 'The file and the text read from it are deleted. The paper stays.',
      accept: 'Remove the file',
      danger: true,
    },
    async () => {
      const { data, error } = await api.DELETE('/documents/{id}/files/{fileId}', {
        params: { path: { id: props.id, fileId: f.id } },
      })
      if (error) {
        notice.failed(error)
        return
      }
      doc.value = data
    },
  )
}

function removeDocument() {
  ask(
    {
      header: 'Remove this paper?',
      message:
        'The paper, its files, and the text read from them are deleted. This cannot be undone.',
      accept: 'Remove the paper',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/documents/{id}', path.value)
      if (error) {
        notice.failed(error)
        return
      }
      notice.done('Paper removed')
      await goBack(router, '/papers', `/papers/${props.id}`)
    },
  )
}

// One dialog shows a file's text, and lets the owner type or correct it.
const texting = ref<{ file: DocumentFile; text: string } | null>(null)
const textOpen = ref(false)
function showText(f: DocumentFile) {
  texting.value = { file: f, text: f.text }
  textOpen.value = true
}
async function saveText(text: string) {
  if (!texting.value) {
    return
  }
  const { data, error } = await api.PUT('/documents/{id}/files/{fileId}/text', {
    params: { path: { id: props.id, fileId: texting.value.file.id } },
    body: { text },
  })
  if (error) {
    notice.failed(error)
    return
  }
  doc.value = data
  textOpen.value = false
}

const isImage = (f: DocumentFile) => f.contentType.startsWith('image/')
</script>

<template>
  <div v-if="doc" class="mx-auto max-w-record">
    <PageHeader :title="doc.title" :trail="[{ label: 'Papers', to: '/papers' }]">
      <template #details>
        <p v-if="doc.authors?.length" class="mt-2">{{ doc.authors.join(', ') }}</p>
        <p v-if="owner && doc.course" class="mt-1 text-ink-muted">
          From
          <RouterLink :to="`/courses/${doc.courseId}`" class="text-accent">{{
            doc.course
          }}</RouterLink>
        </p>
      </template>
      <template v-if="owner">
        <RouterLink v-slot="{ navigate }" :to="`/papers/${doc.id}/edit`" custom>
          <Button type="button" severity="secondary" @click="navigate"
            ><FontAwesomeIcon :icon="faPen" /> Edit</Button
          >
        </RouterLink>
        <Button type="button" severity="secondary" @click="finding = true"
          ><FontAwesomeIcon :icon="faMagnifyingGlass" /> Find a copy</Button
        >
        <Button
          v-if="doc.doi"
          type="button"
          severity="secondary"
          :loading="refreshing"
          @click="lookUpAgain"
          ><FontAwesomeIcon :icon="faArrowsRotate" /> Look up again</Button
        >
        <Button type="button" severity="danger" text @click="removeDocument"
          ><FontAwesomeIcon :icon="faTrash" /> Remove the paper</Button
        >
      </template>
    </PageHeader>

    <dl v-if="details.length" class="detail-list">
      <template v-for="d in details" :key="d.label">
        <dt>{{ d.label }}</dt>
        <dd>{{ d.value }}</dd>
      </template>
      <template v-if="doc.doi">
        <dt>DOI</dt>
        <dd>
          <a
            :href="`https://doi.org/${doc.doi}`"
            class="font-mono text-accent"
            rel="noopener"
            target="_blank"
            >{{ doc.doi }}</a
          >
        </dd>
      </template>
      <template v-else-if="doc.url">
        <dt>Link</dt>
        <dd>
          <a :href="doc.url" class="text-accent" rel="noopener" target="_blank">{{ doc.url }}</a>
        </dd>
      </template>
    </dl>
    <p v-if="owner && doc.notes" class="prose-text mt-4">{{ doc.notes }}</p>
    <section v-if="doc.abstract" class="mt-6">
      <h2 class="mb-2 text-lg">Abstract</h2>
      <p class="prose-text">{{ doc.abstract }}</p>
    </section>
    <SubjectList :subjects="doc.subjects ?? []" class="mt-6" />

    <PagePanel title="Files" class="mt-8">
      <template v-if="owner" #actions>
        <CoverPhoto subject="page" :size="2400" @picked="photographed" />
        <Button type="button" severity="secondary" :loading="busy" @click="uploads?.click()"
          ><FontAwesomeIcon :icon="faFileArrowUp" /> Add a PDF or image</Button
        >
        <input
          ref="uploads"
          type="file"
          :accept="documentFileTypes"
          multiple
          class="hidden"
          aria-label="Choose a PDF or an image to add"
          @change="chosen"
        />
      </template>
      <p v-if="owner && !doc.files?.length" class="prose-text text-ink-muted">
        No files yet. Photograph each page, or add the PDF. The words in them are read in the
        background, so a search finds them.
      </p>
      <DataTable v-if="doc.files?.length" :value="doc.files" data-key="id">
        <Column
          header="Preview"
          header-class="column-unlabelled w-24 max-sm:hidden"
          body-class="max-sm:hidden"
        >
          <template #body="{ data }">
            <a
              :href="fileURL(doc.id, (data as DocumentFile).id)"
              target="_blank"
              rel="noopener"
              class="block"
            >
              <img
                v-if="isImage(data as DocumentFile)"
                :src="fileURL(doc.id, (data as DocumentFile).id)"
                :alt="`${(data as DocumentFile).name}, opens in a new tab`"
                loading="lazy"
                class="thumb"
              />
              <span v-else class="thumb"
                ><FontAwesomeIcon :icon="faFilePdf" class="text-2xl" /><span class="sr-only"
                  >{{ (data as DocumentFile).name }}, opens in a new tab</span
                ></span
              >
            </a>
          </template>
        </Column>
        <Column header="File">
          <template #body="{ data }">
            <a
              :href="fileURL(doc.id, (data as DocumentFile).id)"
              target="_blank"
              rel="noopener"
              class="row-link break-all"
              >{{ (data as DocumentFile).name }}</a
            >
            <div class="text-sm text-ink-muted">{{ fileSize((data as DocumentFile).size) }}</div>
            <div v-if="owner" class="text-sm">
              {{ textStatusLabel((data as DocumentFile).textStatus) }}
            </div>
          </template>
        </Column>
        <Column v-if="owner" header="Actions" header-class="column-unlabelled">
          <template #body="{ data }">
            <div class="flex flex-col items-end gap-1 sm:flex-row sm:justify-end">
              <Button
                type="button"
                text
                class="whitespace-nowrap"
                :label="(data as DocumentFile).text ? 'The text' : 'Type the text'"
                :aria-label="`The text of ${(data as DocumentFile).name}`"
                @click="showText(data as DocumentFile)"
              />
              <Button
                type="button"
                text
                severity="danger"
                label="Remove"
                :aria-label="`Remove ${(data as DocumentFile).name}`"
                @click="removeFile(data as DocumentFile)"
              />
            </div>
          </template>
        </Column>
      </DataTable>
    </PagePanel>

    <PagePanel title="Cite it" class="mt-6">
      <CitationBox :work="documentCite(doc)" />
    </PagePanel>

    <Dialog
      v-if="owner"
      v-model:visible="textOpen"
      modal
      :header="texting ? `The text of ${texting.file.name}` : ''"
      class="w-full max-w-2xl"
    >
      <template v-if="texting">
        <p
          v-if="texting.file.textStatus === 'failed'"
          class="prose-text mb-3 text-sm"
          role="status"
        >
          The text service could not read this file ({{ texting.file.textError }}). Type its text
          here, or clear it to have the service try again.
        </p>
        <p
          v-else-if="texting.file.textStatus === 'pending'"
          class="prose-text mb-3 text-sm"
          role="status"
        >
          The text service has not read this file yet. You can type its text yourself.
        </p>
        <FormField
          label="Text"
          :for="`${uid}-text`"
          hint="What you type here is searched, and the text service leaves it as you wrote it."
        >
          <Textarea
            :id="`${uid}-text`"
            v-model="texting.text"
            rows="12"
            class="font-mono text-sm"
          />
        </FormField>
      </template>
      <template #footer>
        <Button
          v-if="texting?.file.textStatus === 'owner'"
          type="button"
          text
          label="Read it again"
          @click="saveText('')"
        />
        <Button type="button" severity="secondary" label="Cancel" @click="textOpen = false" />
        <Button type="button" label="Save the text" @click="saveText(texting?.text ?? '')" />
      </template>
    </Dialog>
    <FindCopyDialog v-if="owner" v-model:visible="finding" :doc="doc" @added="foundCopy" />
    <SuggestedChanges
      v-if="owner"
      v-model:visible="suggesting"
      :papers="suggestions"
      @saved-papers="([d]) => d && (doc = d)"
    />
  </div>
</template>

<style scoped>
.thumb {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 4.5rem;
  height: 6rem;
  object-fit: cover;
  border: 1px solid var(--color-hairline);
  border-radius: var(--radius-control);
  background: var(--color-surface-page);
  color: var(--color-ink-muted);
}
</style>
