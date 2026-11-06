<script setup lang="ts">
import { computed, useId } from 'vue'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import SelectButton from 'primevue/selectbutton'
import Textarea from 'primevue/textarea'
import ToggleSwitch from 'primevue/toggleswitch'
import type { CopyInput } from '@/api/client'
import { acquiredAs, conditions, dustJackets } from '@/app/format'
import FormField from './FormField.vue'

// The one copy form: where this physical book came from and what shape it is in. Brief, it is
// what a batch of copies shares: their section and whether they were bought new or used.
defineProps<{ sections?: string[] | null; brief?: boolean }>()
const copy = defineModel<CopyInput>({ required: true })

const id = useId()
const f = (name: string) => `${id}-${name}`

// The price is kept in cents, with -1 for not known; the field shows dollars or nothing.
const dollars = computed<number | null>({
  get: () => (copy.value.priceCents < 0 ? null : copy.value.priceCents / 100),
  set: (v) => {
    copy.value.priceCents = v === null ? -1 : Math.round(v * 100)
  },
})
</script>

<template>
  <div class="grid gap-4 md:grid-cols-2">
    <FormField
      label="Section"
      :for="f('section')"
      :hint="
        brief
          ? 'Optional. The books added now go in this section.'
          : 'Where it is shelved: Fiction, Nonfiction, …'
      "
    >
      <InputText :id="f('section')" v-model="copy.section" :list="f('sections')" />
      <datalist :id="f('sections')">
        <option v-for="s in sections ?? []" :key="s" :value="s" />
      </datalist>
    </FormField>
    <FormField label="Bought" :for="f('as')">
      <SelectButton
        :id="f('as')"
        v-model="copy.acquiredAs"
        :aria-labelledby="`${f('as')}-label`"
        :options="acquiredAs"
        option-label="label"
        option-value="value"
        :allow-empty="false"
      />
    </FormField>
    <template v-if="!brief">
      <FormField label="Acquired on" :for="f('on')">
        <InputText :id="f('on')" v-model="copy.acquiredOn" type="date" />
      </FormField>
      <FormField label="From" :for="f('from')" hint="A shop, a person, a course bookstore">
        <InputText :id="f('from')" v-model="copy.acquiredFrom" />
      </FormField>
      <FormField label="Price paid" :for="f('price')">
        <InputNumber
          v-model="dollars"
          :input-id="f('price')"
          mode="currency"
          currency="USD"
          :min="0"
        />
      </FormField>
      <FormField label="Condition" :for="f('condition')">
        <Select
          v-model="copy.condition"
          :input-id="f('condition')"
          :aria-labelledby="`${f('condition')}-label`"
          :options="conditions"
          option-label="label"
          option-value="value"
        />
      </FormField>
      <FormField label="Dust jacket" :for="f('jacket')">
        <Select
          v-model="copy.dustJacket"
          :input-id="f('jacket')"
          :aria-labelledby="`${f('jacket')}-label`"
          :options="dustJackets"
          option-label="label"
          option-value="value"
        />
      </FormField>
      <FormField label="Signed" :for="f('signed')">
        <ToggleSwitch v-model="copy.signed" :input-id="f('signed')" />
      </FormField>
      <FormField label="Location" :for="f('location')" hint="Room, bookcase, or shelf">
        <InputText :id="f('location')" v-model="copy.location" />
      </FormField>
      <FormField label="Notes" :for="f('notes')" class="md:col-span-2">
        <Textarea :id="f('notes')" v-model="copy.notes" rows="2" auto-resize />
      </FormField>
    </template>
  </div>
</template>
