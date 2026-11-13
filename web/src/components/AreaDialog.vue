<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import SelectButton from 'primevue/selectbutton'
import { api, type Area, type AreaInput, type Bookcase } from '@/api/client'
import { areaKeyText, areaKeys, categories, shelfNumbers } from '@/app/format'
import { useNotice } from '@/app/problem'
import FormField from './FormField.vue'
import LengthField from './LengthField.vue'
import PagePanel from './PagePanel.vue'
import TagInput from './TagInput.vue'

// The one dialog for an area: its name, the rules for which books it takes, the order they
// stand in, and its shelves. Every rule is optional; an area with none takes whatever no area
// before it does.
const props = defineProps<{ area: Area | null; areas: Area[]; bookcases: Bookcase[] }>()
const emit = defineEmits<{ saved: [areas: Area[]] }>()
const open = defineModel<boolean>('visible', { required: true })
const notice = useNotice()
const id = useId()

type Key = NonNullable<AreaInput['order']>[number]
const blank = (): AreaInput => ({
  name: '',
  position: 0,
  categories: [],
  deweyFrom: '',
  deweyTo: '',
  subject: '',
  authors: [],
  series: [],
  minHeightMm: 0,
  maxHeightMm: 0,
  order: ['subject', 'author', 'series', 'title'],
  shelfIds: [],
})
const form = ref<AreaInput>(blank())
const authors = ref<string[]>([])
const seriesNames = ref<string[]>([])
watch(open, async (now) => {
  if (!now) {
    return
  }
  const a = props.area
  form.value = a
    ? {
        name: a.name,
        position: 0,
        categories: [...(a.categories ?? [])],
        deweyFrom: a.deweyFrom,
        deweyTo: a.deweyTo,
        subject: a.subject,
        authors: [...(a.authors ?? [])],
        series: [...(a.series ?? [])],
        minHeightMm: a.minHeightMm,
        maxHeightMm: a.maxHeightMm,
        order: [...(a.order ?? [])],
        shelfIds: [...(a.shelfIds ?? [])],
      }
    : blank()
  const { data } = await api.GET('/choices')
  authors.value = data?.authors ?? []
  seriesNames.value = data?.series ?? []
})

const categoryOptions = categories.map((c) => ({ value: c.value, label: c.plural }))

// The order: a key a row, each used once.
const order = computed(() => form.value.order ?? [])
const keyLabel = (k: Key) => areaKeyText(k, false)
const unused = (k: Key) => areaKeys.filter((o) => o.value === k || !order.value.includes(o.value))
function setKey(i: number, k: Key) {
  const next = [...order.value]
  next[i] = k
  form.value.order = next
}
function addKey() {
  const k = areaKeys.find((o) => !order.value.includes(o.value))
  if (k) {
    form.value.order = [...order.value, k.value]
  }
}
function moveKey(i: number, by: -1 | 1) {
  const next = [...order.value]
  const [k] = next.splice(i, 1)
  if (k) {
    next.splice(i + by, 0, k)
  }
  form.value.order = next
}
function removeKey(i: number) {
  form.value.order = order.value.filter((_, j) => j !== i)
}

// The shelves, chosen bookcase by bookcase: a shelf in another area moves to this one.
const chosen = computed(() => new Set(form.value.shelfIds ?? []))
const shelvesIn = (b: Bookcase) =>
  (b.shelves ?? []).map((s) => ({ value: s.id, label: String(s.position) }))
const chosenIn = (b: Bookcase) =>
  (b.shelves ?? []).filter((s) => chosen.value.has(s.id)).map((s) => s.id)
function choose(b: Bookcase, ids: number[] | null) {
  const here = new Set((b.shelves ?? []).map((s) => s.id))
  form.value.shelfIds = [...(form.value.shelfIds ?? []).filter((s) => !here.has(s)), ...(ids ?? [])]
}
// The shelves of a bookcase in other areas, area by area: shelves 1 to 4 in Fiction.
function elsewhere(b: Bookcase): string {
  const byArea = new Map<string, number[]>()
  for (const s of b.shelves ?? []) {
    if (s.areaId && s.areaId !== props.area?.id) {
      byArea.set(s.area, [...(byArea.get(s.area) ?? []), s.position])
    }
  }
  const parts = [...byArea].map(([area, at]) => `${shelfNumbers(at)} in ${area}`)
  return parts.length ? `Already: ${parts.join('; ')}.` : ''
}

const busy = ref(false)
async function save() {
  busy.value = true
  const a = props.area
  const { data, error } = a
    ? await api.PUT('/areas/{id}', { params: { path: { id: a.id } }, body: form.value })
    : await api.POST('/areas', { body: form.value })
  busy.value = false
  if (error) {
    notice.failed(error)
    return
  }
  open.value = false
  emit('saved', data.areas ?? [])
}
</script>

<template>
  <Dialog
    v-model:visible="open"
    modal
    :header="area ? `Edit ${area.name}` : 'Add an area'"
    class="w-full max-w-2xl"
  >
    <form :id="`${id}-area`" class="flex flex-col gap-4" @submit.prevent="save">
      <FormField label="Name" :for="`${id}-name`" hint="Nonfiction, Manga, Tall books" required>
        <InputText :id="`${id}-name`" v-model="form.name" maxlength="100" required />
      </FormField>

      <PagePanel fieldset title="Which books it takes">
        <p class="prose-text mb-3 text-sm text-ink-muted">
          A book goes to the first area that takes it. Leave a rule empty to take every book; an
          area with no rules takes whatever no area before it does.
        </p>
        <div class="flex flex-col gap-4">
          <FormField label="Categories" :for="`${id}-categories`">
            <SelectButton
              :id="`${id}-categories`"
              v-model="form.categories"
              :options="categoryOptions"
              option-label="label"
              option-value="value"
              multiple
              :aria-labelledby="`${id}-categories-label`"
              class="flex-wrap"
            />
          </FormField>
          <div class="grid gap-4 sm:grid-cols-2">
            <FormField
              label="Dewey from"
              :for="`${id}-from`"
              hint="Like 500. A book without one is estimated from its LC class."
            >
              <InputText :id="`${id}-from`" v-model="form.deweyFrom" inputmode="decimal" />
            </FormField>
            <FormField label="Dewey to" :for="`${id}-to`" hint="Like 599: takes 599.9 too.">
              <InputText :id="`${id}-to`" v-model="form.deweyTo" inputmode="decimal" />
            </FormField>
          </div>
          <FormField
            label="Subject heading has"
            :for="`${id}-subject`"
            hint="Words in one of a book's subject headings, like World War"
          >
            <InputText :id="`${id}-subject`" v-model="form.subject" maxlength="100" />
          </FormField>
          <FormField label="By these authors" :for="`${id}-authors`">
            <TagInput
              v-model="form.authors"
              :input-id="`${id}-authors`"
              :suggestions="authors"
              placeholder="Type a name, then Enter"
            />
          </FormField>
          <FormField label="In these series" :for="`${id}-series`">
            <TagInput
              v-model="form.series"
              :input-id="`${id}-series`"
              :suggestions="seriesNames"
              placeholder="Type a series, then Enter"
            />
          </FormField>
          <div class="grid gap-4 sm:grid-cols-2">
            <LengthField v-model="form.minHeightMm" label="At least this tall" />
            <LengthField v-model="form.maxHeightMm" label="No taller than" />
          </div>
        </div>
      </PagePanel>

      <PagePanel fieldset title="Their order on the shelves">
        <ol class="flex flex-col gap-3">
          <li v-for="(k, i) in order" :key="k" class="flex flex-wrap items-end gap-x-2">
            <FormField
              :label="i ? 'Then by' : 'Order by'"
              :for="`${id}-key-${i}`"
              class="w-full sm:w-auto sm:grow"
            >
              <Select
                :model-value="k"
                :input-id="`${id}-key-${i}`"
                :aria-labelledby="`${id}-key-${i}-label`"
                :options="unused(k)"
                option-label="label"
                option-value="value"
                @update:model-value="setKey(i, $event)"
              />
            </FormField>
            <Button
              type="button"
              text
              label="Up"
              :aria-label="`Move ${keyLabel(k)} up`"
              :disabled="i === 0"
              @click="moveKey(i, -1)"
            />
            <Button
              type="button"
              text
              label="Down"
              :aria-label="`Move ${keyLabel(k)} down`"
              :disabled="i === order.length - 1"
              @click="moveKey(i, 1)"
            />
            <Button
              type="button"
              text
              severity="danger"
              label="Remove"
              :aria-label="`Order by ${keyLabel(k)} no longer`"
              :disabled="order.length === 1"
              @click="removeKey(i)"
            />
          </li>
        </ol>
        <Button
          type="button"
          severity="secondary"
          label="Then by another"
          class="mt-3"
          :disabled="order.length >= areaKeys.length"
          @click="addKey"
        />
      </PagePanel>

      <PagePanel fieldset title="Its shelves">
        <p v-if="!bookcases.length" class="prose-text text-sm text-ink-muted">
          Add a bookcase first; its shelves can then be given to the area.
        </p>
        <div class="flex flex-col gap-4">
          <FormField
            v-for="b in bookcases"
            :key="b.id"
            :label="b.name"
            :for="`${id}-case-${b.id}`"
            :hint="elsewhere(b) || 'Shelves numbered from the top.'"
          >
            <SelectButton
              :id="`${id}-case-${b.id}`"
              :model-value="chosenIn(b)"
              :options="shelvesIn(b)"
              option-label="label"
              option-value="value"
              multiple
              :aria-labelledby="`${id}-case-${b.id}-label`"
              class="flex-wrap"
              @update:model-value="choose(b, $event)"
            />
          </FormField>
        </div>
      </PagePanel>
    </form>
    <template #footer>
      <Button type="button" severity="secondary" label="Cancel" @click="open = false" />
      <Button
        type="submit"
        :form="`${id}-area`"
        :label="area ? 'Save the area' : 'Add the area'"
        :loading="busy"
        :disabled="!form.name.trim()"
      />
    </template>
  </Dialog>
</template>
