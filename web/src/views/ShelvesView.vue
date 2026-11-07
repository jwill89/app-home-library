<script setup lang="ts">
import { computed, onMounted, ref, useId, watch } from 'vue'
import { storeToRefs } from 'pinia'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import { faPlus, faTags } from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import {
  api,
  apiURL,
  type Bookcase,
  type Layout,
  type ShelfBook,
  type ShelfInput,
  type ShelfPlace,
  type ShelfPlan,
} from '@/api/client'
import { useConfirmAction } from '@/app/confirm'
import { callNumberText, count, day, lengthIn, placeLine, shelfSize } from '@/app/format'
import { useNotice } from '@/app/problem'
import { useUnits } from '@/app/units'
import BookPicker from '@/components/BookPicker.vue'
import FormField from '@/components/FormField.vue'
import PageHeader from '@/components/PageHeader.vue'
import PagePanel from '@/components/PagePanel.vue'
import ShelfFields from '@/components/ShelfFields.vue'
import UnitChoice from '@/components/UnitChoice.vue'

const ask = useConfirmAction()
const notice = useNotice()
const id = useId()

const bookcases = ref<Bookcase[]>([])
const layout = ref<Layout | null>(null)
const loaded = ref(false)
// While a layout is in use, a fresh proposal can be looked at before it replaces it.
const proposal = ref(false)

async function load() {
  const [b, l] = await Promise.all([
    api.GET('/bookcases'),
    api.GET('/layout', { params: { query: { proposal: proposal.value } } }),
  ])
  if (b.error) {
    notice.failed(b.error)
    return
  }
  if (l.error) {
    notice.failed(l.error)
    return
  }
  bookcases.value = b.data.bookcases ?? []
  layout.value = l.data
  fill.value = l.data.fillPercent
  loaded.value = true
}
onMounted(load)
watch(proposal, load)

// Lengths show in the unit the owner measures in, the one choice every form shares.
const { unit } = storeToRefs(useUnits())

const shelfCount = computed(() => bookcases.value.reduce((n, b) => n + (b.shelves?.length ?? 0), 0))
const summary = computed(() => {
  if (!loaded.value || !bookcases.value.length) {
    return ''
  }
  const n = bookcases.value.length
  return `${count(n, 'bookcase')}, ${count(shelfCount.value, 'shelf', 'shelves')}`
})
const plansOf = (b: Bookcase) => (layout.value?.shelves ?? []).filter((p) => p.bookcaseId === b.id)
const percent = (p: ShelfPlan) => Math.round((p.usedMm / p.widthMm) * 100)
// A shelf's range, by call number; a book without one (no Dewey number, no letters in its
// title) is named by its title instead.
const range = (p: ShelfPlan) => {
  const books = p.books ?? []
  if (!books.length) {
    return ''
  }
  const first = callNumberText(p.first) || (books[0]?.title ?? '')
  const last = callNumberText(p.last) || (books.at(-1)?.title ?? '')
  return first === last ? first : `${first} to ${last}`
}

// The layout: how full, put to use, let go.
const fill = ref(80)
async function saveFill() {
  const { error } = await api.PUT('/layout', { body: { fillPercent: fill.value } })
  if (error) {
    notice.failed(error)
    return
  }
  notice.done(`Shelves are planned ${fill.value}% full`)
  await load()
}

function useLayout() {
  const go = async () => {
    const { error } = await api.PUT('/layout/in-use')
    if (error) {
      notice.failed(error)
      return
    }
    proposal.value = false
    notice.done('Layout in use. New books will go to the shelf they belong on.')
    await load()
  }
  if (!layout.value?.inUse && !proposal.value) {
    return void go()
  }
  ask(
    {
      header: 'Use the new layout?',
      message:
        'Every shelf takes its range from this proposal instead. Books on the shelves now may need to move to match it.',
      accept: 'Use the new layout',
    },
    go,
  )
}

function stopLayout() {
  ask(
    {
      header: 'Stop using the layout?',
      message:
        'The shelves no longer keep their ranges, and the planner proposes from scratch, so new books push the rest along.',
      accept: 'Stop using it',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/layout/in-use')
      if (error) {
        notice.failed(error)
        return
      }
      await load()
    },
  )
}

// A list of books from the layout, shown in a dialog: a shelf's, or those without a place.
const listing = ref<{ title: string; books: ShelfBook[] } | null>(null)
const listOpen = ref(false)
function list(title: string, books: ShelfBook[] | null | undefined) {
  listing.value = { title, books: books ?? [] }
  listOpen.value = true
}

// Bookcases and shelves.
const blankShelf = (): ShelfInput => ({
  widthMm: 0,
  heightMm: 0,
  depthMm: 0,
  oversize: false,
  braceHeightMm: 0,
  braceWidthMm: 0,
})
const newCase = ref({ name: '', shelves: 5, shelf: blankShelf() })
const newCaseOpen = ref(false)
function openNewCase() {
  const last = bookcases.value.at(-1)?.shelves?.[0]
  newCase.value = {
    name: '',
    shelves: 5,
    shelf: last ? { ...last, oversize: false } : blankShelf(),
  }
  newCaseOpen.value = true
}
const measured = (s: ShelfInput) => s.widthMm >= 100 && s.heightMm >= 50 && s.depthMm >= 50
async function createCase() {
  const { name, shelves, shelf } = newCase.value
  const { error } = await api.POST('/bookcases', { body: { name, shelves, ...shelf } })
  if (error) {
    notice.failed(error)
    return
  }
  newCaseOpen.value = false
  await load()
}

const editingCase = ref<{ id: number; name: string; position: number } | null>(null)
const caseOpen = ref(false)
function editCase(b: Bookcase) {
  editingCase.value = { id: b.id, name: b.name, position: b.position }
  caseOpen.value = true
}
async function saveCase() {
  if (!editingCase.value) {
    return
  }
  const { id: bid, name, position } = editingCase.value
  const { error } = await api.PUT('/bookcases/{id}', {
    params: { path: { id: bid } },
    body: { name, position },
  })
  if (error) {
    notice.failed(error)
    return
  }
  caseOpen.value = false
  await load()
}

function removeCase(b: Bookcase) {
  const n = b.shelves?.length ?? 0
  ask(
    {
      header: `Remove ${b.name}?`,
      message: `${b.name} and its ${n === 1 ? 'shelf' : `${n} shelves`} are removed. The books on them are planned onto the other shelves.`,
      accept: 'Remove the bookcase',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/bookcases/{id}', { params: { path: { id: b.id } } })
      if (error) {
        notice.failed(error)
        return
      }
      await load()
    },
  )
}

// One dialog adds a shelf to a bookcase or changes one.
const shelfEdit = ref<{
  shelfId: number
  bookcaseId: number
  title: string
  shelf: ShelfInput
} | null>(null)
const shelfOpen = ref(false)
function addShelf(b: Bookcase) {
  const last = b.shelves?.at(-1)
  shelfEdit.value = {
    shelfId: 0,
    bookcaseId: b.id,
    title: `Add a shelf to ${b.name}`,
    shelf: last ? { ...last, oversize: false } : blankShelf(),
  }
  shelfOpen.value = true
}
function editShelf(p: ShelfPlan) {
  shelfEdit.value = {
    shelfId: p.id,
    bookcaseId: p.bookcaseId,
    title: `${p.bookcase}, shelf ${p.position}`,
    shelf: {
      widthMm: p.widthMm,
      heightMm: p.heightMm,
      depthMm: p.depthMm,
      oversize: p.oversize,
      braceHeightMm: p.braceHeightMm,
      braceWidthMm: p.braceWidthMm,
    },
  }
  shelfOpen.value = true
}
async function saveShelf() {
  if (!shelfEdit.value) {
    return
  }
  const { shelfId, bookcaseId, shelf } = shelfEdit.value
  const { error } = shelfId
    ? await api.PUT('/shelves/{id}', { params: { path: { id: shelfId } }, body: shelf })
    : await api.POST('/bookcases/{id}/shelves', {
        params: { path: { id: bookcaseId } },
        body: shelf,
      })
  if (error) {
    notice.failed(error)
    return
  }
  shelfOpen.value = false
  await load()
}

function removeShelf(p: ShelfPlan) {
  ask(
    {
      header: `Remove shelf ${p.position} of ${p.bookcase}?`,
      message:
        'The shelf is removed and the ones below it move up a number. Its books are planned onto the other shelves.',
      accept: 'Remove the shelf',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/shelves/{id}', { params: { path: { id: p.id } } })
      if (error) {
        notice.failed(error)
        return
      }
      await load()
    },
  )
}

// Where does this go: a copy chosen, or scanned by its label, and its place.
const findCopy = ref(0)
const place = ref<ShelfPlace | null>(null)
watch(findCopy, async (copyId) => {
  place.value = null
  if (!copyId) {
    return
  }
  const { data, error } = await api.GET('/copies/{id}/place', { params: { path: { id: copyId } } })
  if (error) {
    notice.failed(error)
    return
  }
  place.value = data
})
const whereItGoes = computed(() => (place.value ? placeLine(place.value, true) : ''))
</script>

<template>
  <div class="mx-auto max-w-page">
    <PageHeader title="Shelves" :subtitle="summary" :trail="[{ label: 'Library', to: '/' }]">
      <Button type="button" @click="openNewCase"
        ><FontAwesomeIcon :icon="faPlus" /> Add a bookcase</Button
      >
    </PageHeader>

    <p v-if="loaded && !bookcases.length" class="prose-text mb-6">
      Measure each bookcase's shelves (the width inside the sides, the clear height above the shelf,
      and the depth) and add them here. The planner then lays the library out in call number order,
      leaving room on every shelf for books to come, and says where any book belongs.
    </p>

    <PagePanel v-if="layout && bookcases.length" title="The layout">
      <p class="prose-text mb-4">
        <template v-if="layout.inUse">
          In use since {{ day(layout.appliedAt) }}. Each shelf keeps the range it was given, so a
          new book goes to the shelf its call number belongs on, into the room left there.
        </template>
        <template v-else-if="proposal">
          A fresh proposal, not yet in use: the shelves still keep the ranges of the layout in use.
        </template>
        <template v-else>
          A proposal: every book in call number order, each shelf filled to
          {{ layout.fillPercent }}%. Put the books on the shelves this way, then use the layout so
          new books find their place without moving the rest.
        </template>
      </p>
      <div class="mb-4 flex flex-wrap gap-2">
        <Button
          v-if="!layout.inUse"
          type="button"
          :label="proposal ? 'Use the new layout' : 'Use this layout'"
          @click="useLayout"
        />
        <Button
          v-if="layout.inUse || proposal"
          type="button"
          severity="secondary"
          :label="proposal ? 'Back to the layout in use' : 'Plan afresh'"
          @click="proposal = !proposal"
        />
        <Button
          as="a"
          severity="secondary"
          :href="apiURL('/layout/labels')"
          target="_blank"
          rel="noopener"
          ><FontAwesomeIcon :icon="faTags" /> Print shelf labels</Button
        >
        <Button
          v-if="layout.inUse"
          type="button"
          text
          severity="danger"
          label="Stop using it"
          @click="stopLayout"
        />
      </div>
      <div class="mb-4 flex flex-wrap items-end gap-3">
        <form class="flex flex-wrap items-end gap-3" @submit.prevent="saveFill">
          <FormField
            label="Fill each shelf to, %"
            :for="`${id}-fill`"
            hint="The rest is room for books to come."
          >
            <InputNumber
              v-model="fill"
              :input-id="`${id}-fill`"
              :min="50"
              :max="100"
              show-buttons
              button-layout="horizontal"
              :step="5"
              :allow-empty="false"
              increment-button-aria-label="Five percent fuller"
              decrement-button-aria-label="Five percent emptier"
            />
          </FormField>
          <Button type="submit" severity="secondary" label="Save" />
        </form>
        <!-- Its own row on a phone, so the fill's Save never reads as saving the unit. -->
        <UnitChoice class="w-full sm:ml-auto sm:w-auto" />
      </div>
      <ul class="flex flex-col gap-2">
        <li v-if="layout.unplaced?.length" class="flex flex-wrap items-center gap-x-3">
          <span class="font-semibold"
            >No shelf has room for {{ count(layout.unplaced.length, 'book') }}.</span
          >
          <span class="text-sm text-ink-muted">Add shelves, or plan them fuller.</span>
          <Button
            type="button"
            text
            label="Show them"
            @click="list('Books with no room', layout.unplaced)"
          />
        </li>
        <li v-if="layout.tooTall?.length" class="flex flex-wrap items-center gap-x-3">
          <span class="font-semibold"
            >{{ count(layout.tooTall.length, 'book is', 'books are') }} taller than every shelf that
            could take them.</span
          >
          <span class="text-sm text-ink-muted">Mark a tall shelf for oversize books.</span>
          <Button
            type="button"
            text
            label="Show them"
            @click="list('Books too tall', layout.tooTall)"
          />
        </li>
        <li v-if="layout.estimated" class="prose-text text-sm text-ink-muted">
          {{ layout.estimated }} books' sizes are worked out from their format and pages. Their
          height and thickness on a book's Edit page make the plan truer.
        </li>
      </ul>
    </PagePanel>

    <PagePanel v-if="layout?.shelves?.length" title="Where does this go?" class="mt-6">
      <BookPicker v-model="findCopy" />
      <p v-if="whereItGoes" class="mt-4 text-lg font-semibold" role="status">{{ whereItGoes }}</p>
    </PagePanel>

    <PagePanel v-for="b in bookcases" :key="b.id" :title="b.name" class="mt-6">
      <template #actions>
        <Button type="button" severity="secondary" @click="addShelf(b)"
          ><FontAwesomeIcon :icon="faPlus" /> Add a shelf</Button
        >
        <Button
          type="button"
          text
          label="Edit"
          :aria-label="`Rename or move ${b.name}`"
          @click="editCase(b)"
        />
        <Button
          type="button"
          text
          severity="danger"
          label="Remove"
          :aria-label="`Remove ${b.name}`"
          @click="removeCase(b)"
        />
      </template>
      <DataTable :value="plansOf(b)" data-key="id">
        <template #empty><p class="text-ink-muted">No shelves yet.</p></template>
        <Column header="Shelf">
          <template #body="{ data }">
            <span class="font-semibold">{{ (data as ShelfPlan).position }}</span>
            <div v-if="(data as ShelfPlan).oversize" class="text-sm">Oversize</div>
          </template>
        </Column>
        <Column header="Holds">
          <template #body="{ data }">
            <span v-if="(data as ShelfPlan).books?.length" class="font-mono">{{
              range(data as ShelfPlan)
            }}</span>
            <span v-else class="text-ink-muted">Empty</span>
            <div class="fill mt-2" aria-hidden="true">
              <div
                class="fill__bar"
                :class="{ 'fill__bar--over': (data as ShelfPlan).overMm > 0 }"
                :style="{ width: `${Math.min(percent(data as ShelfPlan), 100)}%` }"
              />
            </div>
            <div class="mt-1 text-sm">
              {{ count((data as ShelfPlan).books?.length ?? 0, 'book') }},
              {{ percent(data as ShelfPlan) }}% full
              <span v-if="(data as ShelfPlan).overMm > 0" class="font-semibold">
                · over by {{ lengthIn((data as ShelfPlan).overMm, unit) }}
              </span>
              <span v-if="layout?.inUse && !(data as ShelfPlan).inUse" class="text-ink-muted">
                · not in the layout in use</span
              >
            </div>
            <div v-if="(data as ShelfPlan).gapMm > 0" class="text-sm">
              Leave {{ lengthIn((data as ShelfPlan).gapMm, unit) }} empty at the left: the first
              book is too tall for the brace.
            </div>
            <div class="text-sm text-ink-muted md:hidden">
              {{ shelfSize(data as ShelfPlan, unit) }}
            </div>
          </template>
        </Column>
        <Column
          header="Size"
          header-class="max-md:hidden"
          body-class="max-md:hidden whitespace-nowrap"
        >
          <template #body="{ data }">{{ shelfSize(data as ShelfPlan, unit) }}</template>
        </Column>
        <Column header="Actions" header-class="column-unlabelled">
          <template #body="{ data }">
            <div class="flex flex-col items-end gap-1 md:flex-row md:justify-end">
              <Button
                type="button"
                text
                label="Books"
                :aria-label="`The books on shelf ${(data as ShelfPlan).position}`"
                :disabled="!(data as ShelfPlan).books?.length"
                @click="
                  list(
                    `${b.name}, shelf ${(data as ShelfPlan).position}`,
                    (data as ShelfPlan).books,
                  )
                "
              />
              <RouterLink
                v-if="layout?.inUse && (data as ShelfPlan).books?.length"
                v-slot="{ navigate }"
                :to="`/shelves/${(data as ShelfPlan).id}/check`"
                custom
              >
                <Button
                  type="button"
                  text
                  label="Check"
                  :aria-label="`Check shelf ${(data as ShelfPlan).position}: scan its books`"
                  @click="navigate"
                />
              </RouterLink>
              <Button
                type="button"
                text
                label="Edit"
                :aria-label="`Edit shelf ${(data as ShelfPlan).position}`"
                @click="editShelf(data as ShelfPlan)"
              />
              <Button
                type="button"
                text
                severity="danger"
                label="Remove"
                :aria-label="`Remove shelf ${(data as ShelfPlan).position}`"
                @click="removeShelf(data as ShelfPlan)"
              />
            </div>
          </template>
        </Column>
      </DataTable>
    </PagePanel>

    <Dialog v-model:visible="newCaseOpen" modal header="Add a bookcase" class="w-full max-w-2xl">
      <form :id="`${id}-case`" class="flex flex-col gap-4" @submit.prevent="createCase">
        <div class="grid gap-4 sm:grid-cols-[2fr_1fr]">
          <FormField
            label="Name"
            :for="`${id}-name`"
            hint="Where it is: Living room, left"
            required
          >
            <InputText :id="`${id}-name`" v-model="newCase.name" maxlength="100" required />
          </FormField>
          <FormField label="Shelves" :for="`${id}-count`" hint="Each can be changed after">
            <InputNumber
              v-model="newCase.shelves"
              :input-id="`${id}-count`"
              :min="1"
              :max="20"
              show-buttons
              button-layout="horizontal"
              :allow-empty="false"
              increment-button-aria-label="One shelf more"
              decrement-button-aria-label="One shelf fewer"
            />
          </FormField>
        </div>
        <ShelfFields v-model="newCase.shelf" />
      </form>
      <template #footer>
        <Button type="button" severity="secondary" label="Cancel" @click="newCaseOpen = false" />
        <Button
          type="submit"
          :form="`${id}-case`"
          label="Add the bookcase"
          :disabled="!newCase.name.trim() || !measured(newCase.shelf)"
        />
      </template>
    </Dialog>

    <Dialog v-model:visible="caseOpen" modal header="Edit the bookcase" class="w-full max-w-lg">
      <form
        v-if="editingCase"
        :id="`${id}-edit-case`"
        class="flex flex-col gap-4"
        @submit.prevent="saveCase"
      >
        <FormField label="Name" :for="`${id}-case-name`" required>
          <InputText :id="`${id}-case-name`" v-model="editingCase.name" maxlength="100" required />
        </FormField>
        <FormField
          label="Place in the walk"
          :for="`${id}-case-position`"
          :hint="`1 is the first bookcase the shelves run along, ${bookcases.length} the last.`"
        >
          <InputNumber
            v-model="editingCase.position"
            :input-id="`${id}-case-position`"
            :min="1"
            :max="bookcases.length"
            show-buttons
            button-layout="horizontal"
            :allow-empty="false"
            increment-button-aria-label="Later in the walk"
            decrement-button-aria-label="Earlier in the walk"
          />
        </FormField>
      </form>
      <template #footer>
        <Button type="button" severity="secondary" label="Cancel" @click="caseOpen = false" />
        <Button
          type="submit"
          :form="`${id}-edit-case`"
          label="Save"
          :disabled="!editingCase?.name.trim()"
        />
      </template>
    </Dialog>

    <Dialog
      v-model:visible="shelfOpen"
      modal
      :header="shelfEdit?.title ?? ''"
      class="w-full max-w-2xl"
    >
      <form v-if="shelfEdit" :id="`${id}-shelf`" @submit.prevent="saveShelf">
        <ShelfFields v-model="shelfEdit.shelf" oversize-choice />
      </form>
      <template #footer>
        <Button type="button" severity="secondary" label="Cancel" @click="shelfOpen = false" />
        <Button
          type="submit"
          :form="`${id}-shelf`"
          :label="shelfEdit?.shelfId ? 'Save the shelf' : 'Add the shelf'"
          :disabled="!shelfEdit || !measured(shelfEdit.shelf)"
        />
      </template>
    </Dialog>

    <Dialog
      v-model:visible="listOpen"
      modal
      :header="listing?.title ?? ''"
      class="w-full max-w-2xl"
    >
      <DataTable
        v-if="listing"
        :value="listing.books"
        data-key="copyId"
        :paginator="listing.books.length > 20"
        :rows="20"
      >
        <Column header="Book">
          <template #body="{ data }">
            <RouterLink :to="`/books/${(data as ShelfBook).bookId}`" class="row-link">{{
              (data as ShelfBook).title
            }}</RouterLink>
            <div class="font-mono text-sm">
              {{ callNumberText((data as ShelfBook).callNumber) }}
            </div>
            <div v-if="(data as ShelfBook).onLoan" class="text-sm text-ink-muted">Out on loan</div>
          </template>
        </Column>
        <Column header="Size" body-class="whitespace-nowrap text-sm">
          <template #body="{ data }">
            {{ lengthIn((data as ShelfBook).thicknessMm, unit) }} thick,
            {{ lengthIn((data as ShelfBook).heightMm, unit) }} tall
            <div v-if="(data as ShelfBook).estimated" class="text-ink-muted">estimated</div>
          </template>
        </Column>
      </DataTable>
    </Dialog>
  </div>
</template>

<style scoped>
/* How full a shelf is, which the words beside it also say. */
.fill {
  height: 0.5rem;
  max-width: 20rem;
  border-radius: 999px;
  background: var(--color-surface-page);
  box-shadow: inset 0 0 0 1px var(--color-hairline);
}

.fill__bar {
  height: 100%;
  border-radius: inherit;
  background: var(--color-accent);
}

.fill__bar--over {
  background: var(--color-status-danger);
}
</style>
