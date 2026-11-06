<script setup lang="ts">
import { computed, ref, useId, useTemplateRef } from 'vue'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import { faFileImport, faMagnifyingGlass } from '@fortawesome/pro-regular-svg-icons'
import { api, type Course, type DocumentFill, type DocumentInput } from '@/api/client'
import { containerLabel, courseLabel, documentKinds, paperDetailLabel } from '@/app/format'
import { messageOf } from '@/app/problem'
import FormField from './FormField.vue'
import PagePanel from './PagePanel.vue'
import DoiSearchDialog from './DoiSearchDialog.vue'
import TagInput from './TagInput.vue'

// The one document form. A DOI fills in what Crossref knows, and only what is still blank,
// and adds the subjects OpenAlex gives it; so does a citation exported from a library's
// database, as RIS, with its subject terms. Either brings an abstract where it has one.
// Nothing the owner typed is replaced.
const props = defineProps<{ courses?: Course[] | null }>()
const doc = defineModel<DocumentInput>({ required: true })
const id = useId()
const f = (name: string) => `${id}-${name}`

const hints: Partial<Record<DocumentInput['kind'], string>> = {
  article: 'Or the magazine or site it appeared in',
  chapter: 'The book the chapter is in',
  review: 'The journal the review appeared in',
}
const container = computed(() => ({
  label: containerLabel(doc.value.kind),
  hint: hints[doc.value.kind] ?? 'A journal, a book, or a site, if it has one',
}))
const courseOptions = computed(() => [
  { value: 0, label: 'No course' },
  ...(props.courses ?? []).map((c) => ({
    value: c.id,
    label: courseLabel(c),
  })),
])

const looking = ref(false)
const found = ref('')
const problem = ref('')
async function fillFromDOI() {
  problem.value = ''
  found.value = ''
  looking.value = true
  const { data, error } = await api.POST('/lookup/doi', {
    body: { doi: doc.value.doi, document: doc.value },
  })
  looking.value = false
  if (error) {
    problem.value = messageOf(error)
    return
  }
  took(data, 'the DOI')
}

// A paper without its DOI: Crossref is searched by its title and first author, and the one
// chosen fills the form as a DOI typed does.
const searchingDOI = ref(false)
async function chosenDOI(doi: string) {
  doc.value.doi = doi
  await fillFromDOI()
}

// A record the owner exported from the library's database, signed in there with their card.
const exports = useTemplateRef<HTMLInputElement>('exports')
const reading = ref(false)
async function fillFromExport(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  problem.value = ''
  found.value = ''
  reading.value = true
  const { data, error } = await api.POST('/lookup/citation', {
    body: { ris: await file.text(), document: doc.value },
  })
  reading.value = false
  if (error) {
    problem.value = messageOf(error)
    return
  }
  took(data, 'the export')
}

// The server fills the blanks, as it does for an import, and says which: here they are named,
// the container for what it is now that the kind is known. That is read from the paper sent
// back, since the model takes it only once the page around the form has drawn it.
function took(fill: DocumentFill, from: string) {
  doc.value = fill.document
  const filled = (fill.filled ?? []).map((f) => {
    const label = paperDetailLabel(f, fill.document.kind)
    return f === 'doi' ? label : label.toLowerCase()
  })
  found.value = filled.length
    ? `Filled in from ${from}: ${filled.join(', ')}.`
    : `Everything ${from} gives is filled in already.`
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <PagePanel fieldset title="What it is">
      <div class="grid gap-4 md:grid-cols-2">
        <FormField label="Kind" :for="f('kind')">
          <Select
            v-model="doc.kind"
            :input-id="f('kind')"
            :aria-labelledby="`${f('kind')}-label`"
            :options="documentKinds"
            option-label="label"
            option-value="value"
          />
        </FormField>
        <div class="flex min-w-0 flex-col items-start gap-1">
          <FormField
            label="DOI"
            :for="f('doi')"
            hint="For an article or a chapter: 10.1038/… or its doi.org link"
            class="self-stretch"
          >
            <div class="flex gap-2">
              <InputText :id="f('doi')" v-model="doc.doi" class="min-w-0 flex-1" maxlength="200" />
              <Button
                type="button"
                severity="secondary"
                :loading="looking"
                :disabled="doc.doi.trim().length < 5"
                aria-label="Fill in from the DOI"
                @click="fillFromDOI"
                ><FontAwesomeIcon :icon="faMagnifyingGlass" /> Fill in</Button
              >
            </div>
          </FormField>
          <Button
            v-if="!doc.doi.trim()"
            type="button"
            severity="secondary"
            text
            @click="searchingDOI = true"
            ><FontAwesomeIcon :icon="faMagnifyingGlass" /> Search for its DOI</Button
          >
        </div>
      </div>
      <div class="mt-3">
        <Button type="button" severity="secondary" text :loading="reading" @click="exports?.click()"
          ><FontAwesomeIcon :icon="faFileImport" /> Fill in from a library export</Button
        >
        <input
          ref="exports"
          type="file"
          accept=".ris,application/x-research-info-systems,text/plain"
          class="hidden"
          aria-label="Choose a citation exported as RIS"
          @change="fillFromExport"
        />
        <p class="prose-text text-sm text-ink-muted">
          In your library's database, Export the record as RIS, then choose that file.
        </p>
      </div>
      <p v-if="found" class="mt-3 text-sm" role="status">{{ found }}</p>
      <p v-if="problem" class="mt-3 text-sm text-status-danger" role="alert">{{ problem }}</p>
      <div class="mt-4 grid gap-4">
        <FormField label="Title" :for="f('title')" required>
          <InputText :id="f('title')" v-model="doc.title" maxlength="500" required />
        </FormField>
        <FormField
          label="Authors"
          :for="f('authors')"
          hint="One at a time, as first name then last; Enter after each."
        >
          <TagInput v-model="doc.authors" :input-id="f('authors')" />
        </FormField>
        <!-- A chapter's book has editors, whom its citation names. -->
        <FormField
          v-if="doc.kind === 'chapter' || doc.editors?.length"
          label="The book's editors"
          :for="f('editors')"
          hint="One at a time, as first name then last; Enter after each."
        >
          <TagInput v-model="doc.editors" :input-id="f('editors')" />
        </FormField>
        <FormField
          label="Subjects"
          :for="f('subjects')"
          hint="What it is about. A DOI brings its keywords; add your own, Enter after each."
          class="md:col-span-2"
        >
          <TagInput v-model="doc.subjects" :input-id="f('subjects')" />
        </FormField>
        <FormField
          label="Abstract"
          :for="f('abstract')"
          hint="What it argues, as it gives it. A DOI or a library export fills it in."
        >
          <Textarea
            :id="f('abstract')"
            v-model="doc.abstract"
            rows="4"
            maxlength="20000"
            auto-resize
          />
        </FormField>
      </div>
    </PagePanel>

    <PagePanel fieldset title="Where it appeared">
      <div class="grid gap-4 md:grid-cols-2">
        <FormField :label="container.label" :for="f('container')" :hint="container.hint">
          <InputText :id="f('container')" v-model="doc.container" maxlength="300" />
        </FormField>
        <FormField label="Date" :for="f('published')" hint="YYYY, YYYY-MM, or YYYY-MM-DD">
          <InputText
            :id="f('published')"
            v-model="doc.published"
            maxlength="10"
            inputmode="numeric"
          />
        </FormField>
        <div class="grid grid-cols-3 gap-4">
          <FormField label="Volume" :for="f('volume')">
            <InputText :id="f('volume')" v-model="doc.volume" maxlength="20" />
          </FormField>
          <FormField label="Issue" :for="f('issue')">
            <InputText :id="f('issue')" v-model="doc.issue" maxlength="20" />
          </FormField>
          <FormField label="Pages" :for="f('pages')">
            <InputText :id="f('pages')" v-model="doc.pages" maxlength="30" />
          </FormField>
        </div>
        <FormField label="Publisher" :for="f('publisher')">
          <InputText :id="f('publisher')" v-model="doc.publisher" maxlength="300" />
        </FormField>
        <FormField label="Link" :for="f('url')" class="md:col-span-2">
          <InputText :id="f('url')" v-model="doc.url" type="url" maxlength="2000" />
        </FormField>
      </div>
    </PagePanel>

    <PagePanel fieldset title="Yours">
      <div class="grid gap-4 md:grid-cols-2">
        <FormField label="Course" :for="f('course')" hint="The course it came from, if any">
          <Select
            v-model="doc.courseId"
            :input-id="f('course')"
            :aria-labelledby="`${f('course')}-label`"
            :options="courseOptions"
            option-label="label"
            option-value="value"
            filter
          />
        </FormField>
        <FormField label="Notes" :for="f('notes')" class="md:col-span-2">
          <Textarea :id="f('notes')" v-model="doc.notes" rows="3" maxlength="20000" auto-resize />
        </FormField>
      </div>
    </PagePanel>
    <DoiSearchDialog
      v-model:visible="searchingDOI"
      :title="doc.title"
      :author="doc.authors?.[0] ?? ''"
      @chosen="chosenDOI"
    />
  </div>
</template>
