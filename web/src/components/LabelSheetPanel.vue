<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import { faPrint } from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import { api, apiURL, type LabelSheet, type QueuedLabel } from '@/api/client'
import { callNumberText, count, labelKindLabel } from '@/app/format'
import { useNotice } from '@/app/problem'
import PagePanel from './PagePanel.vue'

// One kind of label sheet: what waits for it, printing it a sheet at a time, and taking what
// printed well off the list. Labels wait for a full sheet, since a sheet half used should not
// go through a laser printer again.
const props = defineProps<{ sheet: LabelSheet; labels: QueuedLabel[] }>()
const emit = defineEmits<{ changed: [] }>()
const notice = useNotice()

const title = computed(() => labelKindLabel(props.sheet.kind))
const per = computed(() => props.sheet.perSheet)
const full = computed(() => Math.floor(props.labels.length / per.value))
const rest = computed(() => props.labels.length % per.value)
const status = computed(() => {
  const n = props.labels.length
  if (!n) {
    return 'Nothing is waiting.'
  }
  if (!full.value) {
    return `${n} waiting, ${per.value - n} short of a full sheet.`
  }
  const more = rest.value ? `, and ${rest.value} toward the next` : ''
  return `${n} waiting: ${count(full.value, 'full sheet')}${more}.`
})
const fullCopies = computed(() =>
  props.labels.slice(0, full.value * per.value).map((l) => l.copyId),
)
const allCopies = computed(() => props.labels.map((l) => l.copyId))

// A spine label shows the call number, so what a call number lacks is worth filling first.
const isSpine = computed(() => props.sheet.kind === 'spine')
const noDewey = computed(() => props.labels.filter((l) => l.missing?.includes('dewey')).length)
const noVolume = computed(() => props.labels.filter((l) => l.missing?.includes('volume')).length)
const onlyMissing = ref(false)
const shown = computed(() =>
  onlyMissing.value ? props.labels.filter((l) => l.missing?.length) : props.labels,
)
const gaps = computed(() => {
  const parts: string[] = []
  if (noDewey.value) {
    parts.push(
      `${noDewey.value} ${noDewey.value === 1 ? 'has' : 'have'} no Dewey number, so the label would carry only the author's letters`,
    )
  }
  if (noVolume.value) {
    parts.push(
      `${noVolume.value} ${noVolume.value === 1 ? 'is' : 'are'} in a series with no volume number`,
    )
  }
  return parts.join(', and ')
})
const missingLabel = (m: string) => (m === 'dewey' ? 'No Dewey number' : 'No volume number')

// The labels sent to the printer, until the owner says they came out well.
const printed = ref<number[]>([])

async function takeOff(copies: number[]) {
  const { error } = await api.DELETE('/labels/{kind}', {
    params: { path: { kind: props.sheet.kind }, query: { copies } },
  })
  if (error) {
    notice.failed(error)
    return
  }
  emit('changed')
}

async function done() {
  const n = printed.value.length
  await takeOff(printed.value)
  printed.value = []
  notice.done(`${count(n, 'label')} taken off the list`)
}
</script>

<template>
  <PagePanel :title="title">
    <p class="prose-text mb-3 text-sm text-ink-muted">
      {{ sheet.name }}, {{ sheet.perSheet }} a sheet ({{ sheet.across }} across,
      {{ sheet.down }} down).
    </p>
    <p class="mb-3 font-semibold" role="status">{{ status }}</p>
    <div class="mb-4 flex flex-wrap gap-2">
      <Button
        v-if="full"
        as="a"
        :href="apiURL(`/labels/${sheet.kind}/sheets`, fullCopies)"
        target="_blank"
        rel="noopener"
        @click="printed = fullCopies"
        ><FontAwesomeIcon :icon="faPrint" /> Print {{ count(full, 'full sheet') }}</Button
      >
      <Button
        v-if="rest"
        as="a"
        severity="secondary"
        :href="apiURL(`/labels/${sheet.kind}/sheets`, allCopies)"
        target="_blank"
        rel="noopener"
        @click="printed = allCopies"
        ><FontAwesomeIcon :icon="faPrint" />
        {{ full ? `Print all ${labels.length}` : `Print these ${labels.length} now` }}</Button
      >
    </div>
    <p v-if="rest && labels.length" class="prose-text -mt-2 mb-4 text-sm text-ink-muted">
      Printing before a sheet is full leaves the rest of it unused: a sheet with labels peeled off
      should not go through the printer again.
    </p>

    <div v-if="printed.length" class="printed mb-4" role="status">
      <p class="prose-text text-sm">
        The {{ printed.length }} labels opened in a new tab. Print them from there, then take them
        off the list. If they came out wrong, keep them, adjust the alignment below, and print
        again.
      </p>
      <div class="mt-3 flex flex-wrap gap-2">
        <Button type="button" label="They printed well: take them off" @click="done" />
        <Button type="button" severity="secondary" label="Keep them" @click="printed = []" />
      </div>
    </div>

    <div v-if="isSpine && gaps" class="mb-4">
      <p class="prose-text text-sm">
        Of these, {{ gaps }}. Fill them in on each book's Edit page before printing, or the labels
        print without them.
      </p>
      <Button
        type="button"
        text
        :aria-pressed="onlyMissing"
        :label="onlyMissing ? 'Show every label' : 'Show only these'"
        @click="onlyMissing = !onlyMissing"
      />
    </div>

    <DataTable
      v-if="labels.length"
      :value="shown"
      data-key="copyId"
      :paginator="shown.length > 10"
      :rows="10"
    >
      <Column header="Label">
        <template #body="{ data }">
          <span class="font-mono tabular">{{ (data as QueuedLabel).barcode }}</span>
        </template>
      </Column>
      <Column header="Book">
        <template #body="{ data }">
          <RouterLink :to="`/books/${(data as QueuedLabel).bookId}`" class="row-link">{{
            (data as QueuedLabel).title
          }}</RouterLink>
          <div class="font-mono text-sm sm:hidden">
            {{ callNumberText((data as QueuedLabel).callNumber) }}
          </div>
          <template v-if="isSpine">
            <div
              v-for="m in (data as QueuedLabel).missing ?? []"
              :key="m"
              class="text-sm sm:hidden"
            >
              {{ missingLabel(m) }}
            </div>
          </template>
        </template>
      </Column>
      <Column header="Call number" header-class="max-sm:hidden" body-class="max-sm:hidden">
        <template #body="{ data }">
          <span class="font-mono">{{ callNumberText((data as QueuedLabel).callNumber) }}</span>
          <template v-if="isSpine">
            <div v-for="m in (data as QueuedLabel).missing ?? []" :key="m" class="text-sm">
              {{ missingLabel(m) }}
            </div>
          </template>
        </template>
      </Column>
      <Column header="Take off" header-class="column-unlabelled">
        <template #body="{ data }">
          <Button
            type="button"
            text
            class="whitespace-nowrap"
            label="Skip"
            :aria-label="`Take the ${sheet.kind} label for ${(data as QueuedLabel).barcode} off the list without printing it`"
            @click="takeOff([(data as QueuedLabel).copyId])"
          />
        </template>
      </Column>
    </DataTable>
  </PagePanel>
</template>

<style scoped>
.printed {
  padding: 0.75rem 1rem;
  border: 1px solid var(--color-hairline);
  border-radius: var(--radius-control);
  background: var(--color-surface-page);
}
</style>
