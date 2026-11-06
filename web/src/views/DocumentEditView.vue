<script setup lang="ts">
import { onMounted, ref, useTemplateRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import { faFileArrowUp } from '@fortawesome/pro-regular-svg-icons'
import { api, type Course, type DocumentInput } from '@/api/client'
import { addDocumentFile, documentFileTypes, documentToInput, emptyDocument } from '@/app/documents'
import { fileSize } from '@/app/format'
import { goBack, returnTo } from '@/app/navigation'
import { messageOf, useNotice } from '@/app/problem'
import DocumentForm from '@/components/DocumentForm.vue'
import PageHeader from '@/components/PageHeader.vue'
import PagePanel from '@/components/PagePanel.vue'

// A new document, with the files the owner has to hand, or the details of one already made,
// whose files are added on its own page.
const props = defineProps<{ id?: number }>()
const route = useRoute()
const router = useRouter()
const notice = useNotice()

const doc = ref<DocumentInput>(emptyDocument(Number(route.query.course) || 0))
const courses = ref<Course[]>([])
const loaded = ref(!props.id)
// The document's title as saved, for the trail back to it while the form changes it.
const savedTitle = ref('')
const busy = ref(false)

// The files picked for a new document, sent once it is made.
const files = ref<File[]>([])
const picker = useTemplateRef<HTMLInputElement>('picker')
function picked(e: Event) {
  const input = e.target as HTMLInputElement
  files.value = [...files.value, ...(input.files ?? [])]
  input.value = ''
}
function leaveOut(f: File) {
  files.value = files.value.filter((x) => x !== f)
}

onMounted(async () => {
  const c = await api.GET('/courses')
  courses.value = c.data?.courses ?? []
  if (!props.id) return
  const { data, error } = await api.GET('/documents/{id}', { params: { path: { id: props.id } } })
  if (error) return notice.failed(error)
  doc.value = documentToInput(data)
  savedTitle.value = data.title
  loaded.value = true
})

async function save() {
  busy.value = true
  const { data, error } = props.id
    ? await api.PUT('/documents/{id}', { params: { path: { id: props.id } }, body: doc.value })
    : await api.POST('/documents', { body: doc.value })
  if (error) {
    busy.value = false
    return notice.failed(error)
  }
  if (props.id) {
    busy.value = false
    notice.done('Paper saved')
    return returnTo(router, `/papers/${data.id}`)
  }
  // The document is made; a file that will not go stays out, and says why.
  let added = 0
  for (const f of files.value) {
    const sent = await addDocumentFile(data.id, f, f.name)
    if (sent.error) notice.failed(new Error(`${f.name} was not added: ${messageOf(sent.error)}`))
    else added++
  }
  busy.value = false
  notice.done(
    added === 0
      ? 'Paper added. Now add its PDF or photos of its pages.'
      : added === 1
        ? 'Paper added with its file. The words in it are read in the background.'
        : `Paper added with ${added} files. The words in them are read in the background.`,
  )
  // The new document takes the form's place: Back goes where the owner was before adding it.
  await router.replace(`/papers/${data.id}`)
}

function cancel() {
  if (props.id) return returnTo(router, `/papers/${props.id}`)
  return goBack(router, '/papers')
}
</script>

<template>
  <div class="mx-auto max-w-record">
    <PageHeader
      :title="id ? 'Edit the paper' : 'Add a paper'"
      :trail="[
        { label: 'Papers', to: '/papers' },
        ...(id ? [{ label: savedTitle, to: `/papers/${id}` }] : []),
      ]"
    />
    <form v-if="loaded" class="flex flex-col gap-6" @submit.prevent="save">
      <DocumentForm v-model="doc" :courses="courses" />
      <PagePanel v-if="!id" title="Files">
        <template #actions>
          <Button type="button" severity="secondary" @click="picker?.click()"
            ><FontAwesomeIcon :icon="faFileArrowUp" /> Choose a PDF or image</Button
          >
          <input
            ref="picker"
            type="file"
            :accept="documentFileTypes"
            multiple
            class="hidden"
            aria-label="Choose a PDF or an image to add with the paper"
            @change="picked"
          />
        </template>
        <ul v-if="files.length" class="flex flex-col gap-1">
          <li
            v-for="f in files"
            :key="`${f.name}-${f.size}-${f.lastModified}`"
            class="flex items-center gap-3"
          >
            <span class="min-w-0 flex-1 break-words">{{ f.name }}</span>
            <span class="text-sm text-ink-muted tabular">{{ fileSize(f.size) }}</span>
            <Button
              type="button"
              text
              label="Leave out"
              :aria-label="`Leave out ${f.name}`"
              @click="leaveOut(f)"
            />
          </li>
        </ul>
        <p v-else class="prose-text text-ink-muted">
          The PDF, if you have it, or images a scanner made: they go in with the paper, and their
          words are read in the background. Photos of pages are taken on its own page.
        </p>
      </PagePanel>
      <div class="flex flex-wrap gap-2">
        <Button
          type="submit"
          :label="id ? 'Save the paper' : 'Add the paper'"
          :loading="busy"
          :disabled="!doc.title.trim()"
        />
        <Button type="button" severity="secondary" label="Cancel" @click="cancel" />
      </div>
    </form>
  </div>
</template>
