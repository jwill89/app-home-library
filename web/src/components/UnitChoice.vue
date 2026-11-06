<script setup lang="ts">
import { useId } from 'vue'
import { storeToRefs } from 'pinia'
import SelectButton from 'primevue/selectbutton'
import { unitOptions, useUnits } from '@/app/units'
import FormField from './FormField.vue'

// The one choice of unit, imperial or metric, offered beside wherever lengths are typed.
const { unit } = storeToRefs(useUnits())
const id = useId()
</script>

<template>
  <FormField label="Measure in" :for="`${id}-unit`">
    <SelectButton
      :id="`${id}-unit`"
      v-model="unit"
      :aria-labelledby="`${id}-unit-label`"
      :options="unitOptions"
      option-label="label"
      option-value="value"
      :allow-empty="false"
    />
  </FormField>
</template>
