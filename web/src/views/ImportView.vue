<script setup lang="ts">
import { ref, useId, useTemplateRef } from 'vue'
import Button from 'primevue/button'
import Select from 'primevue/select'
import { faFileImport } from '@fortawesome/pro-regular-svg-icons'
import { api, formData, type ImportReport, type NotOwned } from '@/api/client'
import { useNotice } from '@/app/problem'
import { useQueue } from '@/app/queue'
import FormField from '@/components/FormField.vue'
import PageHeader from '@/components/PageHeader.vue'
import PagePanel from '@/components/PagePanel.vue'
import QueueProgress from '@/components/QueueProgress.vue'
import { count } from '@/app/format'

const notice = useNotice()
const { queue, total, progress } = useQueue()
const id = useId()
const input = useTemplateRef<HTMLInputElement>('input')
const busy = ref(false)
const notOwned = ref<NotOwned>('leave')
const notOwnedChoices: { value: NotOwned; label: string }[] = [
  { value: 'leave', label: 'Leave them out' },
  { value: 'library', label: 'Add them to my library' },
  { value: 'wishlist', label: 'Put them on my wishlist' },
]
const report = ref<ImportReport | null>(null)
// The choice the report answers to, for what it says of the books left out.
const reported = ref<NotOwned>('leave')

async function chosen(e: Event) {
  const el = e.target as HTMLInputElement
  const file = el.files?.[0]
  el.value = ''
  if (!file) return
  busy.value = true
  const { data, error } = await api.POST('/imports', {
    params: { query: { notOwned: notOwned.value } },
    ...formData(file),
  })
  busy.value = false
  if (error) return notice.failed(error)
  report.value = data
  reported.value = notOwned.value
}

function added(r: ImportReport): string {
  if (!r.documents) return `${r.added} of ${r.rows} books added.`
  return `${count(r.added, 'book')} and ${count(r.documents, 'paper')} added, of ${r.rows} references.`
}
</script>

<template>
  <div class="mx-auto max-w-record">
    <PageHeader
      title="Import"
      subtitle="Bring in a list of books from another app."
      :trail="[{ label: 'Add a book', to: '/add' }]"
    />

    <PagePanel title="From another app">
      <p class="prose-text mb-3 text-sm">
        Choose the app's export file. The library tells which app made it, and importing the same
        file again adds only what is new.
      </p>
      <ul class="prose-text mb-4 list-disc pl-5 text-sm">
        <li>
          <strong>Bookshelf</strong>: open the menu (☰), choose Import &amp; Export, and export
          your library as a CSV file.
        </li>
        <li>
          <strong>Goodreads</strong>: on the website, open My Books, choose Import and export under
          Tools, then Export Library. Goodreads sends the CSV file by a link when it is ready.
        </li>
        <li>
          <strong>LibraryThing</strong>: on the website, open Tools, then Export, and export your
          books as CSV or tab-delimited text.
        </li>
        <li>
          <strong>EndNote</strong>: show all your references and select them all (Edit, then Select
          All), then choose File, Export, and save as XML. EndNote exports only the references
          selected. Its books join the library; its articles, chapters, and other papers become
          papers.
        </li>
      </ul>
      <p class="prose-text mb-4 text-sm text-ink-muted">
        A book or paper already here keeps everything it has, and gains only what it lacks: its
        blank details, and reading (status, dates, rating, notes). It is linked to its page on
        Goodreads or LibraryThing. Importing the same file again fills what is blank once more. A
        book you have in another edition is yours, not wanted. One on a wish list or a to-read shelf
        goes on your wishlist. EndNote marks no book as owned, and the other apps not every one:
        choose below where those go.
      </p>
      <FormField
        label="Books not marked as owned"
        :for="`${id}-not-owned`"
        hint="The wishlist takes only a book with an ISBN, which a later import can match."
        class="mb-4 max-w-sm"
      >
        <Select
          v-model="notOwned"
          :input-id="`${id}-not-owned`"
          :aria-labelledby="`${id}-not-owned-label`"
          :options="notOwnedChoices"
          option-label="label"
          option-value="value"
        />
      </FormField>
      <Button type="button" :loading="busy" @click="input?.click()">
        <FontAwesomeIcon :icon="faFileImport" /> Choose the export file
      </Button>
      <input
        ref="input"
        type="file"
        accept=".csv,.tsv,.txt,.xml,text/csv,text/tab-separated-values,application/xml,text/xml"
        class="hidden"
        aria-label="Export file from Bookshelf, Goodreads, LibraryThing, or EndNote"
        @change="chosen"
      />

      <div v-if="report" class="mt-4" role="status">
        <p class="font-semibold">From {{ report.source }}: {{ added(report) }}</p>
        <ul class="prose-text text-sm">
          <li v-if="report.matched">
            {{ count(report.matched, 'book') }} already here, now linked, with their blanks filled
            in.
          </li>
          <li v-if="report.documentsHad">
            {{ count(report.documentsHad, 'paper') }} already here by their DOI, now linked, with
            their blanks filled in.
          </li>
          <li v-if="report.filled">
            {{ report.filled }} imported before, with blanks the file could fill.
          </li>
          <li v-if="report.already">{{ report.already }} imported before, with nothing to fill.</li>
          <li v-if="report.wanted">{{ count(report.wanted, 'book') }} put on your wishlist.</li>
          <li v-if="report.notOwned">
            {{ count(report.notOwned, 'book') }} not marked as owned{{
              reported === 'wishlist' ? ', without the ISBN the wishlist needs,' : ''
            }}
            and left out.
          </li>
          <li v-if="report.noIsbn" class="text-ink-muted">
            {{ count(report.noIsbn, 'book') }} without an ISBN; their details are as
            {{ report.source }} had them.
          </li>
        </ul>
        <ul v-if="report.problems?.length" class="mt-2 list-disc pl-5 text-sm text-status-danger">
          <li v-for="p in report.problems" :key="p">{{ p }}</li>
        </ul>
      </div>
    </PagePanel>

    <PagePanel v-if="total > 0" title="Background lookup" class="mt-6">
      <QueueProgress v-bind="queue" :progress="progress" counts>
        Each imported book is looked up in the Library of Congress, Open Library, and Google Books
        for its Dewey number, size, format, and cover. Nothing you entered is overwritten.
      </QueueProgress>
    </PagePanel>
  </div>
</template>
