<script setup lang="ts">
import { useId } from 'vue'
import { storeToRefs } from 'pinia'
import InputNumber from 'primevue/inputnumber'
import { useLength, useUnits } from '@/app/units'
import FormField from './FormField.vue'

// The one field for a length: typed in inches or centimetres, as the owner measures, and kept
// in whole millimetres, as most sources give lengths. Empty is 0, not known.
defineProps<{ label: string; hint?: string; required?: boolean }>()
const mm = defineModel<number>({ required: true })
const { unit } = storeToRefs(useUnits())
const id = useId()
const value = useLength(
  () => mm.value,
  (v) => {
    mm.value = v
  },
)
</script>

<template>
  <FormField :label="`${label}, ${unit}`" :for="id" :hint="hint" :required="required">
    <InputNumber
      v-model="value"
      :input-id="id"
      :min="0"
      :min-fraction-digits="0"
      :max-fraction-digits="2"
      :use-grouping="false"
    />
  </FormField>
</template>
