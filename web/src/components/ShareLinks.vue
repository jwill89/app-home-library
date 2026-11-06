<script setup lang="ts">
import { computed, nextTick, onMounted, ref, useId, watch } from 'vue'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import SelectButton from 'primevue/selectbutton'
import ToggleSwitch from 'primevue/toggleswitch'
import { faCopy, faLink, faPen, faShareNodes } from '@fortawesome/pro-regular-svg-icons'
import {
  api,
  type BookRow,
  type Course,
  type DocumentRow,
  type ShareLink,
  type ShareScope,
} from '@/api/client'
import { useConfirmAction } from '@/app/confirm'
import {
  addDays,
  categories,
  count,
  courseLabel,
  day,
  endOf,
  lastDay,
  listed,
  today,
} from '@/app/format'
import { useLibrary } from '@/app/library'
import { useNotice } from '@/app/problem'
import { copyText } from '@/app/clipboard'
import BookPicker from './BookPicker.vue'
import FormField from './FormField.vue'
import PagePanel from './PagePanel.vue'
import PaperPicker from './PaperPicker.vue'

const ask = useConfirmAction()
const notice = useNotice()
const library = useLibrary()
const id = useId()
const shares = ref<ShareLink[]>([])

async function load() {
  const { data, error } = await api.GET('/shares')
  if (error) return notice.failed(error)
  shares.value = data.shares ?? []
}
onMounted(load)

// What a link shows, in a line for the list: its books, then its papers.
function shows(s: ShareScope): string {
  const n = (ids: number[] | null) => (ids ?? []).length
  let books = 'No books'
  if (s.books !== 'none') {
    const some = [
      ...(s.categories ?? []).map((c) => categories.find((k) => k.value === c)?.plural ?? c),
      ...(n(s.bookIds) ? [count(n(s.bookIds), 'book')] : []),
    ].map((p) => p.toLowerCase())
    books = s.books === 'all' ? 'every book' : listed(some)
    books =
      books.charAt(0).toUpperCase() + books.slice(1) + (s.wishlist ? ', with the wishlist' : '')
  }
  const papers =
    s.papers === 'none'
      ? 'no papers'
      : s.papers === 'all'
        ? 'every paper'
        : listed([
            ...(n(s.courseIds) ? [`the papers of ${count(n(s.courseIds), 'course')}`] : []),
            ...(n(s.paperIds) ? [count(n(s.paperIds), 'paper')] : []),
          ])
  return `${books}; ${papers}`
}

// The dialog makes a link, then shows it the one time it can be shown; or it changes a link,
// whose address stays as it was.
const open = ref(false)
const editing = ref<ShareLink | null>(null)
const name = ref('')
const until = ref('')
const scope = ref<ShareScope>(fresh())
const busy = ref(false)
const made = ref<{ name: string; url: string } | null>(null)
const canShare = typeof navigator.share === 'function'
const tomorrow = computed(() => addDays(today(), 1))

function fresh(): ShareScope {
  return {
    books: 'all',
    categories: [],
    bookIds: [],
    wishlist: true,
    papers: 'none',
    courseIds: [],
    paperIds: [],
  }
}

// The books, courses, and papers a link can choose among, named as the owner knows them.
const courses = ref<Course[]>([])
const papers = ref<DocumentRow[]>([])
let choicesLoaded = false
async function loadChoices() {
  if (!library.loaded) void library.load()
  if (choicesLoaded) return
  const [c, d] = await Promise.all([api.GET('/courses'), api.GET('/documents')])
  if (c.error) return notice.failed(c.error)
  if (d.error) return notice.failed(d.error)
  courses.value = c.data.courses ?? []
  papers.value = d.data.documents ?? []
  choicesLoaded = true
}

function start() {
  editing.value = null
  name.value = ''
  until.value = ''
  scope.value = fresh()
  made.value = null
  open.value = true
  void loadChoices()
}

function change(s: ShareLink) {
  editing.value = s
  name.value = s.name
  // The day the link works through, which endOf turns back into its end.
  until.value = s.expiresAt ? today(new Date(new Date(s.expiresAt).getTime() - 1)) : ''
  const sc = s.scope
  scope.value = {
    ...sc,
    categories: [...(sc.categories ?? [])],
    bookIds: [...(sc.bookIds ?? [])],
    courseIds: [...(sc.courseIds ?? [])],
    paperIds: [...(sc.paperIds ?? [])],
  }
  made.value = null
  open.value = true
  void loadChoices()
}

const bookModes = [
  { value: 'all', label: 'Every book' },
  { value: 'some', label: 'Some' },
  { value: 'none', label: 'None' },
]
const paperModes = [
  { value: 'all', label: 'Every paper' },
  { value: 'some', label: 'Some' },
  { value: 'none', label: 'None' },
]
const categoryOptions = categories.map((c) => ({ value: c.value, label: c.plural }))

const chosenBooks = computed(() =>
  (scope.value.bookIds ?? []).map(
    (b) => library.rows.find((r) => r.id === b) ?? { id: b, title: 'A book no longer here' },
  ),
)
function addBook(b: BookRow) {
  const ids = scope.value.bookIds ?? []
  if (!ids.includes(b.id)) scope.value.bookIds = [...ids, b.id]
}
function removeBook(b: number) {
  scope.value.bookIds = (scope.value.bookIds ?? []).filter((x) => x !== b)
}

// The papers a link shows, a course's whole and one by one, in one list.
interface PaperChoice {
  key: string
  kind: 'course' | 'paper'
  id: number
  label: string
}
const chosenPapers = computed<PaperChoice[]>(() => [
  ...(scope.value.courseIds ?? []).map((c) => {
    const course = courses.value.find((k) => k.id === c)
    return {
      key: `course-${c}`,
      kind: 'course' as const,
      id: c,
      label: `Every paper of ${course ? courseLabel(course) : 'a course no longer here'}`,
    }
  }),
  ...(scope.value.paperIds ?? []).map((p) => ({
    key: `paper-${p}`,
    kind: 'paper' as const,
    id: p,
    label: papers.value.find((d) => d.id === p)?.title ?? 'A paper no longer here',
  })),
])
const choice = (d: unknown) => d as PaperChoice
const courseToAdd = ref<number | null>(null)
const courseOptions = computed(() =>
  courses.value
    .filter((c) => !(scope.value.courseIds ?? []).includes(c.id))
    .map((c) => ({ value: c.id, label: courseLabel(c) })),
)
watch(courseToAdd, (c) => {
  if (c === null) return
  scope.value.courseIds = [...(scope.value.courseIds ?? []), c]
  courseToAdd.value = null
})
function addPaper(d: DocumentRow) {
  const ids = scope.value.paperIds ?? []
  if (!ids.includes(d.id)) scope.value.paperIds = [...ids, d.id]
}
function removePaper(c: PaperChoice) {
  if (c.kind === 'course')
    scope.value.courseIds = (scope.value.courseIds ?? []).filter((x) => x !== c.id)
  else scope.value.paperIds = (scope.value.paperIds ?? []).filter((x) => x !== c.id)
}

// What the link still lacks, as the server would say it: the button waits until it has it.
const lacks = computed(() => {
  const s = scope.value
  if (s.books === 'some' && !(s.categories ?? []).length && !(s.bookIds ?? []).length)
    return 'Choose a kind of book or a book to share, or share every book or none.'
  if (s.papers === 'some' && !(s.courseIds ?? []).length && !(s.paperIds ?? []).length)
    return 'Choose a course or a paper to share, or share every paper or none.'
  if (s.books === 'none' && s.papers === 'none')
    return 'A link has to share something: some books, or some papers.'
  return ''
})

async function save() {
  busy.value = true
  const body = {
    name: name.value,
    expiresAt: until.value ? endOf(until.value) : '',
    scope: scope.value,
  }
  const was = editing.value
  if (was) {
    const { error } = await api.PUT('/shares/{id}', { params: { path: { id: was.id } }, body })
    busy.value = false
    if (error) return notice.failed(error)
    notice.done(`Saved the link for ${body.name.trim()}`)
    open.value = false
    return load()
  }
  const { data, error } = await api.POST('/shares', { body })
  busy.value = false
  if (error) return notice.failed(error)
  // On the name this page is on: the link works on both, and this is the one in use.
  made.value = { name: data.share.name, url: `${location.origin}/s/${data.token}` }
  // The form that had focus is gone; the link takes it, selected, ready to copy.
  await nextTick()
  const field = document.getElementById(`${id}-url`)
  if (field instanceof HTMLInputElement) field.select()
  await load()
}

async function copy() {
  if (!made.value) return
  if (await copyText(made.value.url)) notice.done('Link copied')
  else notice.failed(new Error('The link could not be copied. Select it and copy it by hand.'))
}

async function share() {
  if (!made.value) return
  try {
    await navigator.share({ title: 'My library', url: made.value.url })
  } catch (e) {
    // Closing the share sheet is a choice, not a failure.
    if (!(e instanceof Error && e.name === 'AbortError')) notice.failed(e)
  }
}

function revoke(s: ShareLink) {
  ask(
    {
      header: `Revoke the link for “${s.name}”?`,
      message:
        'It stops working at once, for everyone it was given to, and cannot be turned back on. Make a new one to share again.',
      accept: 'Revoke the link',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/shares/{id}', { params: { path: { id: s.id } } })
      if (error) return notice.failed(error)
      await load()
    },
  )
}

const ended = (s: ShareLink) => s.expiresAt !== '' && new Date(s.expiresAt) <= new Date()
function goodUntil(s: ShareLink): string {
  if (!s.expiresAt) return 'Until revoked'
  return ended(s) ? `Ended ${lastDay(s.expiresAt)}` : `Through ${lastDay(s.expiresAt)}`
}

const header = computed(() => {
  if (made.value) return `The link for ${made.value.name}`
  return editing.value ? `Change the link for ${editing.value.name}` : 'Make a share link'
})
const link = (d: unknown) => d as ShareLink
</script>

<template>
  <PagePanel title="Share links">
    <p class="prose-text mb-4 text-sm text-ink-muted">
      A share link lets someone browse the library without a passkey: every book or only some, the
      wishlist or not, and the papers you choose. They see the covers, each book's details, and what
      you have read, and never what you paid, where or when a copy came from, its condition, or your
      notes; they cannot change anything. Anyone holding the link can use it, so give each person or
      group their own, and revoke it when it is done.
    </p>
    <DataTable :value="shares" data-key="id" class="mb-4">
      <template #empty><p class="text-ink-muted">No share links.</p></template>
      <Column header="For">
        <template #body="{ data }">
          <div class="font-semibold">{{ link(data).name }}</div>
          <div class="text-sm text-ink-muted">{{ shows(link(data).scope) }}</div>
          <!-- On a phone the name's column takes when the link works, and the room. -->
          <div class="text-sm text-ink-muted sm:hidden">{{ goodUntil(link(data)) }}</div>
        </template>
      </Column>
      <Column header="Made" header-class="max-md:hidden" body-class="max-md:hidden">
        <template #body="{ data }">{{ day(link(data).createdAt) }}</template>
      </Column>
      <Column header="Works" header-class="max-sm:hidden" body-class="max-sm:hidden">
        <template #body="{ data }">{{ goodUntil(link(data)) }}</template>
      </Column>
      <Column header="Last opened" header-class="max-lg:hidden" body-class="max-lg:hidden">
        <template #body="{ data }">{{ day(link(data).lastUsedAt) }}</template>
      </Column>
      <Column header="Actions" header-class="column-unlabelled">
        <template #body="{ data }">
          <div class="flex flex-wrap justify-end gap-1">
            <Button
              type="button"
              text
              :aria-label="`Change the link for ${link(data).name}`"
              @click="change(link(data))"
              ><FontAwesomeIcon :icon="faPen" /> Change</Button
            >
            <Button
              type="button"
              text
              severity="danger"
              label="Revoke"
              :aria-label="`Revoke the link for ${link(data).name}`"
              @click="revoke(link(data))"
            />
          </div>
        </template>
      </Column>
    </DataTable>
    <Button type="button" @click="start"
      ><FontAwesomeIcon :icon="faLink" /> Make a share link</Button
    >

    <Dialog v-model:visible="open" modal :header="header" class="w-full max-w-2xl">
      <form v-if="!made" :id="`${id}-form`" class="flex flex-col gap-4" @submit.prevent="save">
        <FormField
          label="Who it is for"
          :for="`${id}-name`"
          hint="Only you see this, in the list here: “Mom”, “Book club”."
          required
        >
          <InputText :id="`${id}-name`" v-model="name" maxlength="80" required />
        </FormField>
        <FormField
          label="Works through"
          :for="`${id}-until`"
          hint="Optional. Leave it empty for a link that works until you revoke it."
        >
          <InputText :id="`${id}-until`" v-model="until" type="date" :min="tomorrow" />
        </FormField>

        <PagePanel fieldset title="Books">
          <div class="flex flex-col gap-4">
            <FormField label="Which books" :for="`${id}-books`">
              <SelectButton
                :id="`${id}-books`"
                v-model="scope.books"
                :options="bookModes"
                option-label="label"
                option-value="value"
                :allow-empty="false"
                :aria-labelledby="`${id}-books-label`"
                class="flex-wrap"
              />
            </FormField>
            <template v-if="scope.books === 'some'">
              <FormField
                label="Every book of these kinds"
                :for="`${id}-categories`"
                hint="As the library list names each book's kind."
              >
                <SelectButton
                  :id="`${id}-categories`"
                  v-model="scope.categories"
                  :options="categoryOptions"
                  option-label="label"
                  option-value="value"
                  multiple
                  :aria-labelledby="`${id}-categories-label`"
                  class="flex-wrap"
                />
              </FormField>
              <BookPicker
                whole
                label="And these books"
                hint="Each shown whatever its kind. Type a title, an author, a series, or an ISBN."
                @chosen="addBook"
              />
              <DataTable v-if="chosenBooks.length" :value="chosenBooks" data-key="id">
                <Column field="title" header="Books chosen" />
                <Column header="Remove" header-class="column-unlabelled">
                  <template #body="{ data }">
                    <div class="flex justify-end">
                      <Button
                        type="button"
                        text
                        label="Remove"
                        :aria-label="`Remove ${(data as BookRow).title}`"
                        @click="removeBook((data as BookRow).id)"
                      />
                    </div>
                  </template>
                </Column>
              </DataTable>
            </template>
            <FormField
              v-if="scope.books !== 'none'"
              label="The wishlist"
              :for="`${id}-wishlist`"
              hint="The books you want, among those shared: what to give you."
            >
              <ToggleSwitch v-model="scope.wishlist" :input-id="`${id}-wishlist`" />
            </FormField>
          </div>
        </PagePanel>

        <PagePanel fieldset title="Papers">
          <div class="flex flex-col gap-4">
            <FormField
              label="Which papers"
              :for="`${id}-papers`"
              hint="A visitor never sees a paper's course, or your notes."
            >
              <SelectButton
                :id="`${id}-papers`"
                v-model="scope.papers"
                :options="paperModes"
                option-label="label"
                option-value="value"
                :allow-empty="false"
                :aria-labelledby="`${id}-papers-label`"
                class="flex-wrap"
              />
            </FormField>
            <template v-if="scope.papers === 'some'">
              <FormField
                v-if="courseOptions.length"
                label="Every paper of a course"
                :for="`${id}-course`"
                hint="Its papers now and those you give it later."
              >
                <Select
                  v-model="courseToAdd"
                  :input-id="`${id}-course`"
                  :aria-labelledby="`${id}-course-label`"
                  :options="courseOptions"
                  option-label="label"
                  option-value="value"
                  placeholder="Add a course"
                  filter
                />
              </FormField>
              <PaperPicker :papers="papers" label="And these papers" @chosen="addPaper" />
              <DataTable v-if="chosenPapers.length" :value="chosenPapers" data-key="key">
                <Column field="label" header="Papers chosen" />
                <Column header="Remove" header-class="column-unlabelled">
                  <template #body="{ data }">
                    <div class="flex justify-end">
                      <Button
                        type="button"
                        text
                        label="Remove"
                        :aria-label="`Remove ${choice(data).label}`"
                        @click="removePaper(choice(data))"
                      />
                    </div>
                  </template>
                </Column>
              </DataTable>
            </template>
          </div>
        </PagePanel>
        <p v-if="lacks" class="prose-text text-sm" role="status">{{ lacks }}</p>
      </form>
      <div v-else class="flex flex-col gap-3" role="status">
        <p class="prose-text text-sm">
          This is the only time the link is shown. Send it now; if it is lost, revoke it and make
          another.
        </p>
        <FormField label="Link" :for="`${id}-url`">
          <InputText
            :id="`${id}-url`"
            :model-value="made.url"
            readonly
            class="font-mono text-sm"
            @focus="($event.target as HTMLInputElement).select()"
          />
        </FormField>
        <div class="flex flex-wrap gap-2">
          <Button type="button" @click="copy"
            ><FontAwesomeIcon :icon="faCopy" /> Copy the link</Button
          >
          <Button v-if="canShare" type="button" severity="secondary" @click="share"
            ><FontAwesomeIcon :icon="faShareNodes" /> Send it…</Button
          >
        </div>
      </div>
      <template #footer>
        <template v-if="!made">
          <Button type="button" severity="secondary" label="Cancel" @click="open = false" />
          <Button
            type="submit"
            :form="`${id}-form`"
            :label="editing ? 'Save the changes' : 'Make the link'"
            :loading="busy"
            :disabled="!name.trim() || !!lacks"
          />
        </template>
        <Button v-else type="button" label="Done" @click="open = false" />
      </template>
    </Dialog>
  </PagePanel>
</template>
