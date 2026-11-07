<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import { api, type Choices, type Course, type CourseDetail, type CourseInput } from '@/api/client'
import { levels, terms } from '@/app/format'
import { useNotice } from '@/app/problem'
import FormField from './FormField.vue'

// The one course form, for a new course and for changing one.
const props = defineProps<{ course?: Course | null }>()
const emit = defineEmits<{ saved: [course: CourseDetail] }>()
const open = defineModel<boolean>('visible', { required: true })
const notice = useNotice()
const id = useId()
const f = (name: string) => `${id}-${name}`

const blank = (): CourseInput => ({
  school: '',
  level: 'unknown',
  department: '',
  number: '',
  title: '',
  term: 'unknown',
  year: 0,
  instructor: '',
  notes: '',
})
const input = ref<CourseInput>(blank())
const choices = ref<Choices | null>(null)
const busy = ref(false)

watch(open, async (v) => {
  if (!v) {
    return
  }
  const c = props.course
  input.value = c
    ? {
        school: c.school,
        level: c.level,
        department: c.department,
        number: c.number,
        title: c.title,
        term: c.term,
        year: c.year,
        instructor: c.instructor,
        notes: c.notes,
      }
    : blank()
  const { data } = await api.GET('/choices')
  choices.value = data ?? null
})

// The year is kept as 0 when not known; the field shows nothing.
const year = computed<number | null>({
  get: () => input.value.year || null,
  set: (v) => {
    input.value.year = v ?? 0
  },
})
const ready = computed(() => input.value.title.trim() !== '' || input.value.number.trim() !== '')

async function save() {
  busy.value = true
  const { data, error } = props.course
    ? await api.PUT('/courses/{id}', {
        params: { path: { id: props.course.id } },
        body: input.value,
      })
    : await api.POST('/courses', { body: input.value })
  busy.value = false
  if (error) {
    notice.failed(error)
    return
  }
  open.value = false
  emit('saved', data)
}
</script>

<template>
  <Dialog
    v-model:visible="open"
    modal
    :header="course ? 'Edit the course' : 'Add a course'"
    class="w-full max-w-2xl"
  >
    <form :id="f('form')" class="grid gap-4 md:grid-cols-2" @submit.prevent="save">
      <div class="grid grid-cols-[1fr_1fr] gap-4">
        <FormField label="Department" :for="f('dept')" hint="MATH, ENGL">
          <InputText
            :id="f('dept')"
            v-model="input.department"
            :list="f('depts')"
            maxlength="40"
            autocapitalize="characters"
          />
          <datalist :id="f('depts')">
            <option v-for="d in choices?.departments ?? []" :key="d" :value="d" />
          </datalist>
        </FormField>
        <FormField label="Number" :for="f('number')" hint="241, 101H">
          <InputText :id="f('number')" v-model="input.number" maxlength="20" />
        </FormField>
      </div>
      <FormField
        label="Title"
        :for="f('title')"
        hint="A course needs a title or a number."
        :required="!input.number.trim()"
      >
        <InputText :id="f('title')" v-model="input.title" maxlength="300" />
      </FormField>
      <div class="grid grid-cols-[1fr_1fr] gap-4">
        <FormField label="Season" :for="f('term')">
          <Select
            v-model="input.term"
            :input-id="f('term')"
            :aria-labelledby="`${f('term')}-label`"
            :options="terms"
            option-label="label"
            option-value="value"
          />
        </FormField>
        <FormField label="Year" :for="f('year')">
          <InputNumber
            v-model="year"
            :input-id="f('year')"
            :use-grouping="false"
            :min="1900"
            :max="2100"
          />
        </FormField>
      </div>
      <FormField label="Level" :for="f('level')">
        <Select
          v-model="input.level"
          :input-id="f('level')"
          :aria-labelledby="`${f('level')}-label`"
          :options="levels"
          option-label="label"
          option-value="value"
        />
      </FormField>
      <FormField label="School" :for="f('school')">
        <InputText :id="f('school')" v-model="input.school" :list="f('schools')" maxlength="200" />
        <datalist :id="f('schools')">
          <option v-for="s in choices?.schools ?? []" :key="s" :value="s" />
        </datalist>
      </FormField>
      <FormField label="Instructor" :for="f('instructor')">
        <InputText
          :id="f('instructor')"
          v-model="input.instructor"
          :list="f('instructors')"
          maxlength="200"
        />
        <datalist :id="f('instructors')">
          <option v-for="i in choices?.instructors ?? []" :key="i" :value="i" />
        </datalist>
      </FormField>
      <FormField label="Notes" :for="f('notes')" class="md:col-span-2">
        <Textarea :id="f('notes')" v-model="input.notes" rows="2" maxlength="5000" auto-resize />
      </FormField>
    </form>
    <template #footer>
      <Button type="button" severity="secondary" label="Cancel" @click="open = false" />
      <Button
        type="submit"
        :form="f('form')"
        :label="course ? 'Save the course' : 'Add the course'"
        :loading="busy"
        :disabled="!ready"
      />
    </template>
  </Dialog>
</template>
