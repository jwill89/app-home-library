<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import { api, type Series } from '@/api/client'
import { numberRanges } from '@/app/format'
import { useNotice } from '@/app/problem'
import LibraryTabs from '@/components/LibraryTabs.vue'
import PageHeader from '@/components/PageHeader.vue'

// Every series the library has a volume of: what is had, what is wanted, the numbers missing
// between, and the next. Each opens its own page, where its books are looked after.
const notice = useNotice()

const series = ref<Series[]>([])
const loaded = ref(false)
onMounted(async () => {
  const { data, error } = await api.GET('/series')
  if (error) return notice.failed(error)
  series.value = data.series ?? []
  loaded.value = true
})

const summary = computed(() => {
  if (!loaded.value) return ''
  const n = series.value.length
  const gaps = series.value.filter((s) => s.gaps?.length).length
  return [`${n} series`, gaps ? `${gaps} with gaps` : ''].filter(Boolean).join(' · ')
})
const row = (d: unknown) => d as Series
</script>

<template>
  <div class="mx-auto max-w-page">
    <LibraryTabs />
    <PageHeader title="Series" :subtitle="summary" />

    <DataTable
      :value="series"
      data-key="id"
      :loading="!loaded"
      :paginator="series.length > 50"
      :rows="50"
    >
      <template #empty>
        <p class="py-6 text-center text-ink-muted">No series yet.</p>
      </template>
      <Column header="Series">
        <template #body="{ data }">
          <RouterLink :to="`/series/${row(data).id}`" class="row-link">{{
            row(data).name
          }}</RouterLink>
          <div class="text-sm text-ink-muted md:hidden">
            {{ row(data).had?.length ? `Have ${numberRanges(row(data).had)}` : '' }}
          </div>
        </template>
      </Column>
      <Column header="Have" header-class="max-md:hidden" body-class="max-md:hidden">
        <template #body="{ data }">
          {{ numberRanges(row(data).had) || `${row(data).books} unnumbered` }}
        </template>
      </Column>
      <Column header="Missing">
        <template #body="{ data }">
          <span v-if="row(data).gaps?.length" class="font-semibold">{{
            numberRanges(row(data).gaps)
          }}</span>
          <span v-else class="text-ink-muted">None</span>
          <div v-if="row(data).wanted?.length" class="text-sm text-ink-muted">
            Wanted: {{ numberRanges(row(data).wanted) }}
          </div>
        </template>
      </Column>
      <Column header="Next" header-class="max-sm:hidden" body-class="max-sm:hidden">
        <template #body="{ data }">{{ row(data).next }}</template>
      </Column>
    </DataTable>
  </div>
</template>
