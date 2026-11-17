<script setup lang="ts">
import { computed, onMounted, ref, useId, watch } from 'vue'
import { storeToRefs } from 'pinia'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import { faMagnifyingGlassArrowRight } from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import { api } from '@/api/client'
import type { Area } from '@/types'
import { stackArrangements } from '@/constants'
import { type Arrangement, arrange } from '@/app/stacks'
import { count, day } from '@/app/format'
import { messageOf } from '@/app/problem'
import { useFeed } from '@/composables/useFeed'
import { useRemembered } from '@/composables/useRemembered'
import { matching, useLibrary } from '@/stores/library'
import { useSession } from '@/stores/session'
import FormField from '@/components/FormField.vue'
import LibraryTabs from '@/components/LibraryTabs.vue'
import PageHeader from '@/components/PageHeader.vue'
import PulledBook from '@/components/PulledBook.vue'
import ShelfPreview from '@/components/ShelfPreview.vue'
import StackCase from '@/components/StackCase.vue'

// The Stacks: the library's books standing as spines in bookcases drawn on the screen, to
// browse along the shelves in the order chosen. A book pulled off its shelf shows its cover,
// what it is, and the way to its page. The owner may see their own measured bookcases too, as a
// layout fills them. The Books tab lists the same books as a table.
const session = useSession()
const library = useLibrary()
const { books, loaded, failed, offlineSince } = storeToRefs(library)
onMounted(() => {
  if (!loaded.value) {
    void library.load()
  }
})
const id = useId()
const f = (name: string) => `${id}-${name}`

const choices = computed(() => stackArrangements.filter((a) => session.isOwner || !a.owner))
const chosen = useRemembered<Arrangement>(
  'hl.stacks',
  stackArrangements.map((a) => a.value),
  'category',
)
// A choice of the owner's, kept on a device a visitor now uses, is not the visitor's.
const by = computed<Arrangement>({
  get: () => (choices.value.some((c) => c.value === chosen.value) ? chosen.value : 'category'),
  set: (v) => (chosen.value = v),
})

// The owner's areas, each with the books its rules take, for the arrangement by area.
const areas = ref<Area[] | null>(null)
const areasFailed = ref('')
async function loadAreas() {
  if (by.value !== 'area') {
    return
  }
  const { data, error } = await api.GET('/areas', { params: { query: { include: 'books' } } })
  if (error) {
    areasFailed.value = messageOf(error)
    return
  }
  areasFailed.value = ''
  areas.value = data.areas ?? []
}
watch(by, () => void loadAreas(), { immediate: true })
useFeed(['book'], loadAreas)

const cases = computed(() => arrange(books.value, by.value, areas.value ?? []))
const byId = computed(() => new Map(books.value.map((b) => [b.id, b])))
const waiting = computed(
  () => !loaded.value || (by.value === 'area' && !areas.value && !areasFailed.value),
)

// Pulling a book off its shelf opens its details; putting it back stands it there again, with
// the focus on it, where the reader was.
const pulledId = ref(0)
const showing = ref(false)
const pulled = computed(() => byId.value.get(pulledId.value) ?? null)
function pull(bookId: number) {
  pulledId.value = bookId
  showing.value = true
}
function putBack() {
  const was = pulledId.value
  pulledId.value = 0
  document.getElementById(`spine-${String(was)}`)?.focus()
}

// Finding a book on the shelves marks every one that matches, and Find next goes to each in
// turn, in the order they stand.
const query = ref('')
const found = computed(() => {
  const words = query.value.trim()
  return new Set(words ? books.value.filter(matching(words)).map((b) => b.id) : [])
})
const foundInOrder = computed(() =>
  cases.value.flatMap((c) => c.books).filter((b) => found.value.has(b.id)),
)
const foundText = computed(() => {
  if (!query.value.trim()) {
    return ''
  }
  const n = foundInOrder.value.length
  return n ? `${count(n, 'book')} found, marked on the shelves.` : 'Nothing on the shelves matches.'
})
let at = -1
watch(query, () => (at = -1))
function next() {
  const list = foundInOrder.value
  if (!list.length) {
    return
  }
  at = (at + 1) % list.length
  const spine = document.getElementById(`spine-${String(list[at]?.id)}`)
  spine?.focus({ preventScroll: true })
  const still = matchMedia('(prefers-reduced-motion: reduce)').matches
  spine?.scrollIntoView({ block: 'center', behavior: still ? 'auto' : 'smooth' })
}
</script>

<template>
  <div class="mx-auto max-w-page">
    <LibraryTabs />
    <PageHeader title="Stacks" :subtitle="loaded ? count(books.length, 'book') : ''" />

    <div class="mb-4 grid gap-3 md:grid-cols-[minmax(0,16rem)_minmax(0,1fr)] md:items-end">
      <FormField label="Arrange by" :for="f('by')">
        <Select
          v-model="by"
          :input-id="f('by')"
          :aria-labelledby="`${f('by')}-label`"
          :options="choices"
          option-label="label"
          option-value="value"
        />
      </FormField>
      <form
        v-if="by !== 'bookcases'"
        role="search"
        aria-label="On the shelves"
        class="flex items-end gap-2"
        @submit.prevent="next"
      >
        <div class="min-w-0 flex-1">
          <FormField label="Find on the shelves" :for="f('find')">
            <InputText
              :id="f('find')"
              v-model="query"
              type="search"
              class="w-full"
              placeholder="Title, author, subject, series, ISBN"
            />
          </FormField>
        </div>
        <Button type="submit" severity="secondary"
          ><FontAwesomeIcon :icon="faMagnifyingGlassArrowRight" /> Find next</Button
        >
      </form>
    </div>
    <p role="status" :class="{ 'mb-4': foundText }">{{ foundText }}</p>

    <p v-if="offlineSince" class="prose-text mb-4 font-semibold" role="status">
      No connection: these are the books as this device kept them {{ day(offlineSince) }}.
    </p>

    <div v-if="failed || areasFailed" class="mb-4 flex flex-wrap items-center gap-3" role="alert">
      <p class="prose-text font-semibold text-status-danger">
        Couldn't load the {{ failed ? 'library' : 'areas' }}: {{ failed || areasFailed }}
      </p>
      <Button
        type="button"
        severity="secondary"
        label="Try again"
        @click="failed ? library.load() : loadAreas()"
      />
    </div>
    <p v-else-if="waiting" class="text-ink-muted" role="status">
      Standing the books on the shelves…
    </p>
    <p v-else-if="!books.length" class="prose-text">
      {{
        session.isOwner ? 'No books yet. Add one, or import from Bookshelf.' : 'No books here yet.'
      }}
    </p>
    <ShelfPreview v-else-if="by === 'bookcases'" :books="byId" />
    <template v-else>
      <StackCase
        v-for="c in cases"
        :key="c.key"
        :title="c.title"
        :books="c.books"
        :by="by"
        :pulled="pulledId"
        :found="found"
        @pull="pull($event.id)"
      />
    </template>

    <PulledBook v-model:visible="showing" :book="pulled" @put-back="putBack" />
  </div>
</template>
