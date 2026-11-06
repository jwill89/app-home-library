<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import { faPlus } from '@fortawesome/pro-regular-svg-icons'
import { api, type Course, type CourseDetail } from '@/api/client'
import { count, courseName, levelLabel, termLabel } from '@/app/format'
import { useNotice } from '@/app/problem'
import CourseDialog from '@/components/CourseDialog.vue'
import PageHeader from '@/components/PageHeader.vue'

const router = useRouter()
const notice = useNotice()
const courses = ref<Course[]>([])
const loaded = ref(false)

onMounted(async () => {
  const { data, error } = await api.GET('/courses')
  if (error) return notice.failed(error)
  courses.value = data.courses ?? []
  loaded.value = true
})

const summary = computed(() => {
  if (!loaded.value || !courses.value.length) return ''
  const n = courses.value.length
  return count(n, 'course')
})

const adding = ref(false)
function added(c: CourseDetail) {
  void router.push(`/courses/${c.id}`)
}

function open(c: Course) {
  void router.push(`/courses/${c.id}`)
}
</script>

<template>
  <div class="mx-auto max-w-page">
    <PageHeader title="Courses" :subtitle="summary">
      <Button type="button" @click="adding = true"
        ><FontAwesomeIcon :icon="faPlus" /> Add a course</Button
      >
    </PageHeader>

    <DataTable
      :value="courses"
      data-key="id"
      :loading="!loaded"
      row-hover
      class="courses"
      @row-click="open($event.data as Course)"
    >
      <template #empty>
        <p class="py-6 text-center text-ink-muted">
          No courses yet. Add one, then put the books it used on it.
        </p>
      </template>
      <Column header="Course">
        <template #body="{ data }">
          <RouterLink :to="`/courses/${(data as Course).id}`" class="row-link" @click.stop>
            {{ courseName(data as Course) }}
          </RouterLink>
          <div class="text-sm text-ink-muted sm:hidden">{{ termLabel(data as Course) }}</div>
          <div class="text-sm text-ink-muted lg:hidden">{{ (data as Course).school }}</div>
        </template>
      </Column>
      <Column header="When" header-class="max-sm:hidden" body-class="max-sm:hidden">
        <template #body="{ data }">{{ termLabel(data as Course) }}</template>
      </Column>
      <Column
        field="school"
        header="School"
        header-class="max-lg:hidden"
        body-class="max-lg:hidden"
      />
      <Column header="Level" header-class="max-xl:hidden" body-class="max-xl:hidden">
        <template #body="{ data }">{{ levelLabel((data as Course).level) }}</template>
      </Column>
      <Column
        field="instructor"
        header="Instructor"
        header-class="max-md:hidden"
        body-class="max-md:hidden"
      />
      <Column field="bookCount" header="Books" body-class="tabular" />
    </DataTable>

    <CourseDialog v-model:visible="adding" @saved="added" />
  </div>
</template>

<style scoped>
.courses :deep(tbody tr) {
  cursor: pointer;
}
</style>
