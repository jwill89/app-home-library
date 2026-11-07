<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  faArrowUpRightFromSquare,
  faFileArrowDown,
} from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import { api, type Document, type OpenCopy } from '@/api/client'
import { librarySearch, librarySearchLink, workWords } from '@/app/libraryLink'
import { useNotice } from '@/app/problem'

// Where a document can be read: an open copy OpenAlex knows (an author's manuscript in a
// repository, or the publisher's open version), the owner's library, where they sign in with
// their own card, and the publisher. The app fetches only the open copy, and only when asked.
const props = defineProps<{
  doc: Pick<Document, 'id' | 'title' | 'authors' | 'doi'>
}>()
const emit = defineEmits<{ added: [doc: Document] }>()
const open = defineModel<boolean>('visible', { required: true })
const notice = useNotice()

const asking = ref(false)
const copy = ref<OpenCopy | null>(null)
const adding = ref(false)

watch(open, async (v) => {
  if (!v || !props.doc.doi) {
    return
  }
  copy.value = null
  asking.value = true
  const { data, error } = await api.GET('/documents/{id}/open-copy', {
    params: { path: { id: props.doc.id } },
  })
  asking.value = false
  if (error) {
    notice.failed(error)
    return
  }
  copy.value = data.found ? data.copy : null
})

const versions: Record<string, string> = {
  publishedVersion: 'the published version',
  acceptedVersion: 'the author’s accepted manuscript, after peer review',
  submittedVersion: 'the author’s draft as submitted, before peer review',
}

// OpenAlex writes licences as cc-by, cc-by-nc-nd, cc0, public-domain, other-oa.
function licence(l: string): string {
  if (!l) {
    return 'no licence given: free to read'
  }
  if (l === 'cc0') {
    return 'CC0'
  }
  if (l.startsWith('cc-')) {
    return `CC ${l.slice(3).toUpperCase()}`
  }
  return l.replace(/-/g, ' ')
}

const said = computed(() => {
  const c = copy.value
  if (!c) {
    return ''
  }
  const what = versions[c.version] ?? 'a copy'
  return `${what.charAt(0).toUpperCase()}${what.slice(1)}${c.host ? `, in ${c.host}` : ''}; ${licence(c.license)}.`
})

const library = computed(() =>
  librarySearchLink(librarySearch.value, workWords(props.doc.title, props.doc.authors ?? [])),
)

async function add() {
  adding.value = true
  const { data, error } = await api.POST('/documents/{id}/open-copy', {
    params: { path: { id: props.doc.id } },
  })
  adding.value = false
  if (error) {
    notice.failed(error)
    return
  }
  notice.done('Added the open copy to the files')
  open.value = false
  emit('added', data)
}
</script>

<template>
  <Dialog v-model:visible="open" modal header="Find a copy" class="w-full max-w-xl">
    <section aria-labelledby="copy-open" class="find">
      <h2 id="copy-open" class="find__title">An open copy</h2>
      <p v-if="!doc.doi" class="prose-text">
        Without a DOI, OpenAlex cannot be asked. The Edit page takes one.
      </p>
      <p v-else-if="asking" class="prose-text" role="status">Asking OpenAlex…</p>
      <template v-else-if="copy">
        <p class="prose-text">{{ said }}</p>
        <div class="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2">
          <Button v-if="copy.pdfUrl" type="button" :loading="adding" @click="add"
            ><FontAwesomeIcon :icon="faFileArrowDown" /> Add it to the files</Button
          >
          <a
            :href="copy.pdfUrl || copy.pageUrl"
            target="_blank"
            rel="noopener"
            class="link-target text-accent"
            >{{ copy.pdfUrl ? 'Open it' : 'Open its page'
            }}<span class="sr-only"> (opens in a new tab)</span>
            <FontAwesomeIcon
              :icon="faArrowUpRightFromSquare"
              class="ml-1 text-sm"
              aria-hidden="true"
          /></a>
        </div>
      </template>
      <p v-else class="prose-text">OpenAlex knows no open copy of it.</p>
    </section>

    <section aria-labelledby="copy-library" class="find">
      <h2 id="copy-library" class="find__title">In your library</h2>
      <template v-if="library">
        <p class="prose-text">
          Sign in there with your library card. Download the PDF and add it here; the record's
          Export, as RIS, fills in its details on the Edit page.
        </p>
        <a :href="library" target="_blank" rel="noopener" class="link-target mt-2 text-accent"
          >Search your library for it<span class="sr-only"> (opens in a new tab)</span>
          <FontAwesomeIcon :icon="faArrowUpRightFromSquare" class="ml-1 text-sm" aria-hidden="true"
        /></a>
      </template>
      <p v-else class="prose-text">
        Give your library's search in
        <RouterLink to="/settings" class="text-accent">Settings</RouterLink> to look for it there.
      </p>
    </section>

    <section v-if="doc.doi" aria-labelledby="copy-publisher" class="find">
      <h2 id="copy-publisher" class="find__title">From the publisher</h2>
      <a
        :href="`https://doi.org/${doc.doi}`"
        target="_blank"
        rel="noopener"
        class="link-target text-accent"
        >The publisher's page<span class="sr-only"> (opens in a new tab)</span>
        <FontAwesomeIcon :icon="faArrowUpRightFromSquare" class="ml-1 text-sm" aria-hidden="true"
      /></a>
    </section>

    <template #footer>
      <Button type="button" severity="secondary" label="Done" @click="open = false" />
    </template>
  </Dialog>
</template>

<style scoped>
.find + .find {
  margin-top: 1.5rem;
}

.find__title {
  margin-bottom: 0.5rem;
  font-size: 1rem;
}
</style>
