<script setup lang="ts">
import { computed, onMounted, ref, useId } from 'vue'
import Button from 'primevue/button'
import ToggleSwitch from 'primevue/toggleswitch'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import SelectButton from 'primevue/selectbutton'
import { faRuler } from '@fortawesome/pro-regular-svg-icons'
import { api, apiURL, type LabelSheet, type QueuedLabel, type ShelvingRule } from '@/api/client'
import { labelKindLabel, shelvingLabel, shelvings } from '@/app/format'
import { useNotice } from '@/app/problem'
import FormField from '@/components/FormField.vue'
import LabelSheetPanel from '@/components/LabelSheetPanel.vue'
import PageHeader from '@/components/PageHeader.vue'
import PagePanel from '@/components/PagePanel.vue'

const notice = useNotice()
const id = useId()

const sheets = ref<LabelSheet[]>([])
const queue = ref<QueuedLabel[]>([])
const rules = ref<ShelvingRule[]>([])
const loaded = ref(false)

async function load() {
  const [s, l, r] = await Promise.all([
    api.GET('/label-sheets'),
    api.GET('/labels'),
    api.GET('/sections'),
  ])
  if (s.error) return notice.failed(s.error)
  if (l.error) return notice.failed(l.error)
  if (r.error) return notice.failed(r.error)
  sheets.value = s.data.sheets ?? []
  queue.value = l.data.labels ?? []
  rules.value = r.data.sections ?? []
  offsets.value = Object.fromEntries(sheets.value.map((x) => [x.kind, { x: x.xMm, y: x.yMm }]))
  loaded.value = true
}
onMounted(load)

const waiting = (kind: string) => queue.value.filter((l) => l.kind === kind)
const printed = computed(() => sheets.value.filter((s) => s.enabled))
const summary = computed(() => {
  if (!loaded.value) return ''
  const parts = printed.value
    .map((s) => ({ n: waiting(s.kind).length, kind: s.kind }))
    .filter((p) => p.n)
    .map((p) => `${p.n} ${p.kind}`)
  return parts.length ? `${parts.join(' and ')} labels waiting` : 'Every label is printed'
})

// Each kind of label is printed or not; one turned off keeps its queue, ready for later.
async function turn(sheet: LabelSheet, enabled: boolean) {
  const { error } = await api.PUT('/label-sheets/{kind}', {
    params: { path: { kind: sheet.kind } },
    body: { xMm: sheet.xMm, yMm: sheet.yMm, enabled },
  })
  if (error) return notice.failed(error)
  await load()
}

// Where this printer's labels land, per sheet, as the owner sets it.
const offsets = ref<Record<string, { x: number; y: number }>>({})
async function saveOffset(kind: string) {
  const o = offsets.value[kind]
  if (!o) return
  const { error } = await api.PUT('/label-sheets/{kind}', {
    params: { path: { kind: kind as LabelSheet['kind'] } },
    body: {
      xMm: o.x ?? 0,
      yMm: o.y ?? 0,
      enabled: sheets.value.find((s) => s.kind === kind)?.enabled ?? true,
    },
  })
  if (error) return notice.failed(error)
  notice.done(
    `${labelKindLabel(kind)} will print moved ${o.x || 0} mm across and ${o.y || 0} mm down`,
  )
  await load()
}

// A section's rule, and what its call numbers come to.
function example(r: { shelving: string; prefix: string }): string {
  if (r.shelving === 'author') return `${r.prefix} MCD v.3`
  if (r.shelving === 'series') return `${r.prefix} ONE v.12`
  return r.prefix ? `823.92 MCD, or ${r.prefix} MCD with no number` : '823.92 MCD'
}
const editing = ref<{
  rule: ShelvingRule
  shelving: ShelvingRule['shelving']
  prefix: string
} | null>(null)
const ruleOpen = ref(false)
function editRule(r: ShelvingRule) {
  editing.value = { rule: r, shelving: r.shelving, prefix: r.prefix }
  ruleOpen.value = true
}
async function saveRule() {
  if (!editing.value) return
  const { rule, shelving, prefix } = editing.value
  const { error } = await api.PUT('/sections/{id}', {
    params: { path: { id: rule.id } },
    body: { shelving, prefix },
  })
  if (error) return notice.failed(error)
  ruleOpen.value = false
  await load()
}
</script>

<template>
  <div class="mx-auto max-w-record">
    <PageHeader title="Labels" :subtitle="summary" :trail="[{ label: 'Library', to: '/' }]" />
    <p class="prose-text mb-5">
      Every copy has a barcode label, for inside the front cover, and a spine label for when you
      shelve by call number. They wait here until a sheet is full, in shelf order. Print through the
      manual feed slot, one sheet at a time, with the media type set to Label and the size set to
      actual size, not fit to page. For barcodes, Avery 6467 peels off a book clean; 5167 is the
      same sheet with a permanent adhesive.
    </p>

    <div v-if="loaded" class="flex flex-col gap-6">
      <PagePanel title="Labels to print">
        <div class="flex flex-col gap-4">
          <FormField
            v-for="s in sheets"
            :key="s.kind"
            :label="labelKindLabel(s.kind)"
            :for="`${id}-${s.kind}-on`"
            :hint="
              s.enabled
                ? `On ${s.name}, ${s.perSheet} a sheet.`
                : `Off. Each copy's label still waits here (${waiting(s.kind).length} now), ready when you turn them on.`
            "
          >
            <ToggleSwitch
              :model-value="s.enabled"
              :input-id="`${id}-${s.kind}-on`"
              @update:model-value="(v: boolean) => turn(s, v)"
            />
          </FormField>
        </div>
      </PagePanel>

      <LabelSheetPanel
        v-for="s in printed"
        :key="s.kind"
        :sheet="s"
        :labels="waiting(s.kind)"
        @changed="load"
      />

      <PagePanel title="Alignment">
        <p class="prose-text mb-4 text-sm text-ink-muted">
          Print an alignment sheet on plain paper, lay it over a sheet of labels, and hold both up
          to a window. If the outlines sit to the left of the labels, move the printing right by
          that many millimetres; if they sit high, move it down. Negative numbers go left and up.
        </p>
        <div class="grid gap-6 md:grid-cols-2">
          <form
            v-for="s in printed"
            :key="s.kind"
            class="flex flex-col gap-3"
            @submit.prevent="saveOffset(s.kind)"
          >
            <h3 class="font-semibold">{{ labelKindLabel(s.kind) }} · {{ s.name }}</h3>
            <div v-if="offsets[s.kind]" class="grid grid-cols-2 gap-3">
              <FormField label="Right, mm" :for="`${id}-${s.kind}-x`">
                <InputNumber
                  v-model="offsets[s.kind]!.x"
                  :input-id="`${id}-${s.kind}-x`"
                  :min="-10"
                  :max="10"
                  show-buttons
                  button-layout="horizontal"
                  :allow-empty="false"
                  increment-button-aria-label="One millimetre more"
                  decrement-button-aria-label="One millimetre less"
                />
              </FormField>
              <FormField label="Down, mm" :for="`${id}-${s.kind}-y`">
                <InputNumber
                  v-model="offsets[s.kind]!.y"
                  :input-id="`${id}-${s.kind}-y`"
                  :min="-10"
                  :max="10"
                  show-buttons
                  button-layout="horizontal"
                  :allow-empty="false"
                  increment-button-aria-label="One millimetre more"
                  decrement-button-aria-label="One millimetre less"
                />
              </FormField>
            </div>
            <div class="flex flex-wrap gap-2">
              <Button type="submit" severity="secondary" label="Save" />
              <Button
                as="a"
                text
                :href="apiURL(`/label-sheets/${s.kind}/alignment`)"
                target="_blank"
                rel="noopener"
                ><FontAwesomeIcon :icon="faRuler" /> Print an alignment sheet</Button
              >
            </div>
          </form>
        </div>
      </PagePanel>

      <PagePanel title="Call numbers">
        <p class="prose-text mb-4 text-sm text-ink-muted">
          A spine label carries the copy's call number, made by its section's rule: the Dewey number
          or the section's prefix, then three letters of the author's surname (of the series, when
          shelved by series), then the volume for a book in a series. A change here reaches the
          labels printed after it.
        </p>
        <DataTable :value="rules" data-key="id">
          <Column field="name" header="Section" />
          <Column header="Shelved">
            <template #body="{ data }">
              {{ shelvingLabel((data as ShelvingRule).shelving) }}
              <div class="font-mono text-sm text-ink-muted">
                {{ example(data as ShelvingRule) }}
              </div>
            </template>
          </Column>
          <Column
            field="copies"
            header="Copies"
            header-class="max-sm:hidden"
            body-class="max-sm:hidden tabular"
          />
          <Column header="Edit" header-class="column-unlabelled">
            <template #body="{ data }">
              <Button
                type="button"
                text
                label="Edit"
                :aria-label="`Edit how ${(data as ShelvingRule).name} is shelved`"
                @click="editRule(data as ShelvingRule)"
              />
            </template>
          </Column>
        </DataTable>
      </PagePanel>
    </div>

    <Dialog
      v-model:visible="ruleOpen"
      modal
      :header="editing ? `How ${editing.rule.name} is shelved` : ''"
      class="w-full max-w-lg"
    >
      <form
        v-if="editing"
        :id="`${id}-rule`"
        class="flex flex-col gap-4"
        @submit.prevent="saveRule"
      >
        <FormField label="Shelved" :for="`${id}-shelving`">
          <SelectButton
            :id="`${id}-shelving`"
            v-model="editing.shelving"
            :options="shelvings"
            option-label="label"
            option-value="value"
            :allow-empty="false"
            :aria-labelledby="`${id}-shelving-label`"
          />
        </FormField>
        <FormField
          label="Prefix"
          :for="`${id}-prefix`"
          :hint="
            editing.shelving === 'dewey'
              ? 'Optional: printed in place of a Dewey number a book does not have yet.'
              : 'Heads every call number in the section, like FIC or MANGA.'
          "
          :required="editing.shelving !== 'dewey'"
        >
          <InputText
            :id="`${id}-prefix`"
            v-model="editing.prefix"
            maxlength="10"
            autocapitalize="characters"
            class="font-mono uppercase"
          />
        </FormField>
        <p class="text-sm">
          <span class="text-ink-muted">A call number here: </span>
          <span class="font-mono">{{
            example({ ...editing, prefix: editing.prefix.toUpperCase() })
          }}</span>
        </p>
      </form>
      <template #footer>
        <Button type="button" severity="secondary" label="Cancel" @click="ruleOpen = false" />
        <Button
          type="submit"
          :form="`${id}-rule`"
          label="Save"
          :disabled="editing?.shelving !== 'dewey' && !editing?.prefix.trim()"
        />
      </template>
    </Dialog>
  </div>
</template>
