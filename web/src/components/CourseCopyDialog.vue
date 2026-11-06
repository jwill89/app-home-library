<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import Select from 'primevue/select'
import ToggleSwitch from 'primevue/toggleswitch'
import { faPlus } from '@fortawesome/pro-regular-svg-icons'
import { api, type Book, type Course, type CourseDetail } from '@/api/client'
import { copyLabel, courseLabel } from '@/app/format'
import { useNotice } from '@/app/problem'
import BookPicker from './BookPicker.vue'
import CourseDialog from './CourseDialog.vue'
import FormField from './FormField.vue'

// The one way to put a copy on a course's list: from the course, choosing the book, or from
// the book, choosing the course (or making it).
const props = defineProps<{ courseId?: number; book?: Book | null }>()
const emit = defineEmits<{ saved: [course: CourseDetail] }>()
const open = defineModel<boolean>('visible', { required: true })
const notice = useNotice()
const id = useId()
const f = (name: string) => `${id}-${name}`

const course = ref(0)
const copyId = ref(0)
const required = ref(true)
const courses = ref<Course[]>([])
const busy = ref(false)
const making = ref(false)

watch(open, async (v) => {
  if (!v) return
  course.value = props.courseId ?? 0
  copyId.value = props.book?.copies?.[0]?.id ?? 0
  required.value = true
  if (props.courseId) return
  const { data, error } = await api.GET('/courses')
  if (error) return notice.failed(error)
  courses.value = data.courses ?? []
  course.value = courses.value[0]?.id ?? 0
})

const courseOptions = computed(() =>
  courses.value.map((c) => ({
    value: c.id,
    label: courseLabel(c),
  })),
)
// A copy that has gone cannot be put on a course.
const copyOptions = computed(() =>
  (props.book?.copies ?? [])
    .filter((c) => !c.goneAs)
    .map((c) => ({ value: c.id, label: copyLabel(c) })),
)

function made(c: CourseDetail) {
  courses.value = [c, ...courses.value]
  course.value = c.id
}

async function save() {
  busy.value = true
  const { data, error } = await api.PUT('/courses/{id}/copies/{copyId}', {
    params: { path: { id: course.value, copyId: copyId.value } },
    body: { required: required.value },
  })
  busy.value = false
  if (error) return notice.failed(error)
  open.value = false
  emit('saved', data)
}
</script>

<template>
  <!-- The course form takes this one's place while it is open, rather than stacking on it. -->
  <Dialog
    :visible="open && !making"
    modal
    :header="courseId ? 'Add a book to the course' : 'Add to a course'"
    class="w-full max-w-lg"
    @update:visible="open = $event"
  >
    <form :id="f('form')" class="flex flex-col gap-4" @submit.prevent="save">
      <BookPicker v-if="courseId" v-model="copyId" />
      <template v-else>
        <FormField v-if="courseOptions.length" label="Course" :for="f('course')" required>
          <Select
            v-model="course"
            :input-id="f('course')"
            :aria-labelledby="`${f('course')}-label`"
            :options="courseOptions"
            option-label="label"
            option-value="value"
            filter
          />
        </FormField>
        <p v-else class="text-sm text-ink-muted">No courses yet. Make the first one.</p>
        <div>
          <Button type="button" severity="secondary" @click="making = true"
            ><FontAwesomeIcon :icon="faPlus" /> A new course</Button
          >
        </div>
        <FormField v-if="copyOptions.length > 1" label="Copy" :for="f('copy')" required>
          <Select
            v-model="copyId"
            :input-id="f('copy')"
            :aria-labelledby="`${f('copy')}-label`"
            :options="copyOptions"
            option-label="label"
            option-value="value"
          />
        </FormField>
      </template>
      <FormField
        label="Required for the course"
        :for="f('required')"
        hint="Off for a book that was recommended or optional."
      >
        <ToggleSwitch v-model="required" :input-id="f('required')" />
      </FormField>
    </form>
    <template #footer>
      <Button type="button" severity="secondary" label="Cancel" @click="open = false" />
      <Button
        type="submit"
        :form="f('form')"
        label="Add it"
        :loading="busy"
        :disabled="!course || !copyId"
      />
    </template>
  </Dialog>
  <CourseDialog v-model:visible="making" @saved="made" />
</template>
