<script setup lang="ts">
import { useId } from 'vue'
import ToggleSwitch from 'primevue/toggleswitch'
import type { ShelfInput } from '@/api/client'
import FormField from './FormField.vue'
import LengthField from './LengthField.vue'
import PagePanel from './PagePanel.vue'
import UnitChoice from './UnitChoice.vue'

// The one shelf form: its inside measurements and its corner braces, typed in inches or
// centimetres and kept in millimetres.
defineProps<{ oversizeChoice?: boolean }>()
const shelf = defineModel<ShelfInput>({ required: true })
const id = useId()
</script>

<template>
  <div class="flex flex-col gap-4">
    <UnitChoice />
    <div class="grid gap-4 sm:grid-cols-3">
      <LengthField v-model="shelf.widthMm" label="Width" hint="Inside the sides" required />
      <LengthField
        v-model="shelf.heightMm"
        label="Height"
        hint="Clear space above the shelf"
        required
      />
      <LengthField v-model="shelf.depthMm" label="Depth" hint="Front to back" required />
    </div>
    <PagePanel fieldset title="Corner braces">
      <p class="prose-text mb-3 text-sm text-ink-muted">
        Blocks under the shelf above, at both ends, that only shorter books fit beneath. Leave them
        empty for a shelf without.
      </p>
      <div class="grid gap-4 sm:grid-cols-2">
        <LengthField
          v-model="shelf.braceHeightMm"
          label="Brace height"
          hint="How far it hangs down"
        />
        <LengthField
          v-model="shelf.braceWidthMm"
          label="Brace width"
          hint="How far in from the side"
        />
      </div>
    </PagePanel>
    <FormField
      v-if="oversizeChoice"
      label="For oversize books"
      :for="`${id}-oversize`"
      hint="Takes the books too tall for the shelf their call number would put them on."
    >
      <ToggleSwitch v-model="shelf.oversize" :input-id="`${id}-oversize`" />
    </FormField>
  </div>
</template>
