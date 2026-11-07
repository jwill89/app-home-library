<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import { faPlus, faTrash } from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import { api, type BookRow } from '@/api/client'
import { emptyCopy } from '@/app/book'
import { useConfirmAction } from '@/app/confirm'
import { calendarDate, count } from '@/app/format'
import { useLibrary } from '@/app/library'
import { useNotice } from '@/app/problem'
import { useSession } from '@/app/session'
import BookCover from '@/components/BookCover.vue'
import LibraryTabs from '@/components/LibraryTabs.vue'
import PageHeader from '@/components/PageHeader.vue'

// The books wanted and not yet had. A visitor sees the list too: it is what to give.
const library = useLibrary()
const { wishlist, loaded } = storeToRefs(library)
const session = useSession()
const notice = useNotice()
const router = useRouter()
onMounted(() => {
  if (!loaded.value) {
    void library.load()
  }
})

const summary = computed(() => {
  const n = wishlist.value.length
  return loaded.value ? `${count(n, 'book')} wanted` : ''
})
const row = (data: unknown) => data as BookRow

// Wanted books the house has in another edition (an import's hardcover of the owner's
// paperback): each says so, and they come off the list together, after asking.
const hadAlready = computed(() => wishlist.value.filter((r) => r.hadAs))
const ask = useConfirmAction()
function removeHad() {
  const had = hadAlready.value
  const n = had.length
  ask(
    {
      header: `Remove the ${n === 1 ? 'book' : `${n} books`} you have?`,
      message: `${n === 1 ? 'It is another edition' : 'Each is another edition'} of a book in your library. ${n === 1 ? 'It comes' : 'They come'} off the wishlist and ${n === 1 ? 'is' : 'are'} deleted; the books you have stay as they are.`,
      accept: n === 1 ? 'Remove it' : `Remove ${n} books`,
      reject: n === 1 ? 'Keep it' : 'Keep them',
      danger: true,
    },
    async () => {
      let removed = 0
      for (const r of had) {
        const { error } = await api.DELETE('/books/{id}', { params: { path: { id: r.id } } })
        if (error) {
          notice.failed(error)
          break
        }
        removed++
      }
      if (removed) {
        notice.done(`Removed ${count(removed, 'book')} you have`)
      }
      await library.load()
    },
  )
}

// Bought or given: the book gets its first copy, which takes it off the wishlist.
async function gotIt(r: BookRow) {
  const { error } = await api.POST('/books/{id}/copies', {
    params: { path: { id: r.id } },
    body: emptyCopy(''),
  })
  if (error) {
    notice.failed(error)
    return
  }
  notice.done(`${r.title} is in your library now`)
  await library.load()
}
</script>

<template>
  <div class="mx-auto max-w-page">
    <LibraryTabs />
    <PageHeader title="Wishlist" :subtitle="summary">
      <template v-if="session.isOwner">
        <RouterLink v-slot="{ navigate }" to="/add?wishlist=1" custom>
          <Button type="button" @click="navigate"
            ><FontAwesomeIcon :icon="faPlus" /> Add a book you want</Button
          >
        </RouterLink>
        <Button v-if="hadAlready.length" type="button" severity="danger" text @click="removeHad"
          ><FontAwesomeIcon :icon="faTrash" />
          {{
            hadAlready.length === 1
              ? 'Remove the one you have'
              : `Remove the ${hadAlready.length} you have`
          }}</Button
        >
      </template>
    </PageHeader>

    <DataTable
      :value="wishlist"
      data-key="id"
      :loading="!loaded"
      row-hover
      @row-click="router.push(`/books/${row($event.data).id}`)"
    >
      <template #empty>
        <p class="py-6 text-center text-ink-muted">
          Nothing wanted yet. Add a book you want, or put one on the list from a scan.
        </p>
      </template>
      <Column header="Book">
        <template #body="{ data }">
          <div class="flex items-start gap-3">
            <BookCover :cover="row(data).cover" :title="row(data).title" :width="48" />
            <div class="min-w-0">
              <RouterLink :to="`/books/${row(data).id}`" class="row-link">{{
                row(data).title
              }}</RouterLink>
              <div class="text-sm text-ink-muted">{{ row(data).authors }}</div>
              <p v-if="row(data).hadAs" class="text-sm">
                Had already, in
                <RouterLink :to="`/books/${row(data).hadAs}`" class="text-accent" @click.stop
                  >another edition</RouterLink
                >.
              </p>
            </div>
          </div>
        </template>
      </Column>
      <Column header="Wanted since" header-class="max-md:hidden" body-class="max-md:hidden">
        <template #body="{ data }">{{ calendarDate(row(data).wantedOn) }}</template>
      </Column>
      <Column v-if="session.isOwner" header="Actions" header-class="column-unlabelled">
        <template #body="{ data }">
          <div class="flex justify-end">
            <Button
              type="button"
              severity="secondary"
              label="Got it"
              :aria-label="`Got ${row(data).title}: put it in the library`"
              @click.stop="gotIt(row(data))"
            />
          </div>
        </template>
      </Column>
    </DataTable>
  </div>
</template>
