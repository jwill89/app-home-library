<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import ToggleSwitch from 'primevue/toggleswitch'
import { faPen, faPlus, faTrash } from '@fortawesome/pro-regular-svg-icons'
import { api, type CourseBook, type CourseDetail, type DocumentRow } from '@/api/client'
import { useConfirmAction } from '@/app/confirm'
import { calendarDate, courseName, documentKindLabel, levelLabel, termLabel } from '@/app/format'
import { goBack } from '@/app/navigation'
import { useNotice } from '@/app/problem'
import BookCover from '@/components/BookCover.vue'
import CourseCopyDialog from '@/components/CourseCopyDialog.vue'
import CourseDialog from '@/components/CourseDialog.vue'
import PageHeader from '@/components/PageHeader.vue'
import PagePanel from '@/components/PagePanel.vue'

const props = defineProps<{ id: number }>()
const router = useRouter()
const ask = useConfirmAction()
const notice = useNotice()
const course = ref<CourseDetail | null>(null)
const path = computed(() => ({ params: { path: { id: props.id } } }))

async function load() {
  const { data, error } = await api.GET('/courses/{id}', path.value)
  if (error) return notice.failed(error)
  shown(data)
}
function shown(c: CourseDetail) {
  course.value = c
  document.title = `${courseName(c)} · Home Library`
}
onMounted(load)
watch(() => props.id, load)

const name = computed(() => (course.value ? courseName(course.value) : ''))
const details = computed(() => {
  const c = course.value
  if (!c) return []
  return [
    { label: 'When', value: termLabel(c) },
    { label: 'School', value: c.school },
    { label: 'Level', value: levelLabel(c.level) },
    { label: 'Instructor', value: c.instructor },
  ].filter((d) => d.value)
})

const editing = ref(false)
const adding = ref(false)

function remove() {
  ask(
    {
      header: `Remove ${name.value}?`,
      message: 'The course and its list of books are deleted. The books stay in the library.',
      accept: 'Remove the course',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/courses/{id}', path.value)
      if (error) return notice.failed(error)
      notice.done('Course removed')
      await goBack(router, '/courses', `/courses/${props.id}`)
    },
  )
}

async function setRequired(b: CourseBook, required: boolean) {
  const { data, error } = await api.PUT('/courses/{id}/copies/{copyId}', {
    params: { path: { id: props.id, copyId: b.copyId } },
    body: { required },
  })
  if (error) return notice.failed(error)
  shown(data)
}

function takeOff(b: CourseBook) {
  ask(
    {
      header: `Take ${b.title} off the course?`,
      message: `${b.barcode} is no longer listed for ${name.value}. The book stays in the library.`,
      accept: 'Take it off',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/courses/{id}/copies/{copyId}', {
        params: { path: { id: props.id, copyId: b.copyId } },
      })
      if (error) return notice.failed(error)
      await load()
    },
  )
}
</script>

<template>
  <div v-if="course" class="mx-auto max-w-record">
    <PageHeader :title="name" :trail="[{ label: 'Courses', to: '/courses' }]">
      <Button type="button" severity="secondary" @click="editing = true"
        ><FontAwesomeIcon :icon="faPen" /> Edit</Button
      >
      <Button type="button" severity="danger" text @click="remove"
        ><FontAwesomeIcon :icon="faTrash" /> Remove the course</Button
      >
    </PageHeader>

    <dl v-if="details.length" class="detail-list">
      <template v-for="d in details" :key="d.label">
        <dt>{{ d.label }}</dt>
        <dd>{{ d.value }}</dd>
      </template>
    </dl>
    <p v-if="course.notes" class="prose-text mt-4">{{ course.notes }}</p>

    <PagePanel title="Books" class="mt-6">
      <template #actions>
        <Button type="button" severity="secondary" @click="adding = true"
          ><FontAwesomeIcon :icon="faPlus" /> Add a book</Button
        >
      </template>
      <DataTable :value="course.books ?? []" data-key="copyId">
        <template #empty>
          <p class="text-ink-muted">No books on this course yet.</p>
        </template>
        <Column
          header="Cover"
          header-class="column-unlabelled w-16 max-sm:hidden"
          body-class="max-sm:hidden"
        >
          <template #body="{ data }">
            <BookCover
              :cover="(data as CourseBook).cover"
              :title="(data as CourseBook).title"
              :width="48"
            />
          </template>
        </Column>
        <Column header="Book">
          <template #body="{ data }">
            <RouterLink :to="`/books/${(data as CourseBook).bookId}`" class="row-link">{{
              (data as CourseBook).title
            }}</RouterLink>
            <div class="text-sm text-ink-muted">{{ (data as CourseBook).authors }}</div>
            <div class="font-mono text-sm tabular">{{ (data as CourseBook).barcode }}</div>
          </template>
        </Column>
        <Column header="Required">
          <template #body="{ data }">
            <ToggleSwitch
              :model-value="(data as CourseBook).required"
              :aria-label="`${(data as CourseBook).title} was required for the course`"
              @update:model-value="setRequired(data as CourseBook, $event)"
            />
          </template>
        </Column>
        <Column header="Take off" header-class="column-unlabelled">
          <template #body="{ data }">
            <Button
              type="button"
              text
              severity="danger"
              class="whitespace-nowrap"
              label="Take off"
              :aria-label="`Take ${(data as CourseBook).title} off the course`"
              @click="takeOff(data as CourseBook)"
            />
          </template>
        </Column>
      </DataTable>
    </PagePanel>

    <PagePanel title="Papers" class="mt-6">
      <template #actions>
        <RouterLink v-slot="{ navigate }" :to="`/papers/new?course=${course.id}`" custom>
          <Button type="button" severity="secondary" @click="navigate"
            ><FontAwesomeIcon :icon="faPlus" /> Add a paper</Button
          >
        </RouterLink>
      </template>
      <DataTable :value="course.documents ?? []" data-key="id">
        <template #empty>
          <p class="text-ink-muted">
            No papers from this course yet: its syllabus, handouts, notes, or exams.
          </p>
        </template>
        <Column header="Paper">
          <template #body="{ data }">
            <RouterLink :to="`/papers/${(data as DocumentRow).id}`" class="row-link">{{
              (data as DocumentRow).title
            }}</RouterLink>
            <div v-if="(data as DocumentRow).authors?.length" class="text-sm text-ink-muted">
              {{ (data as DocumentRow).authors?.join(', ') }}
            </div>
          </template>
        </Column>
        <Column header="Kind">
          <template #body="{ data }">{{ documentKindLabel((data as DocumentRow).kind) }}</template>
        </Column>
        <Column header="Date" header-class="max-sm:hidden" body-class="max-sm:hidden">
          <template #body="{ data }">{{ calendarDate((data as DocumentRow).published) }}</template>
        </Column>
      </DataTable>
    </PagePanel>

    <CourseDialog v-model:visible="editing" :course="course" @saved="shown" />
    <CourseCopyDialog v-model:visible="adding" :course-id="course.id" @saved="shown" />
  </div>
</template>
