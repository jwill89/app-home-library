<script setup lang="ts">
import { ref, useId, watch } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import { api, type GoneInput } from '@/api/client'
import { goneKindLabel, goneKinds, today } from '@/app/format'
import { useNotice } from '@/app/problem'
import FormField from './FormField.vue'

// The one way a copy leaves the library: how it went, the day, and a note. Its record stays,
// and it can come back.
const props = defineProps<{
  copy: { id: number; barcode: string } | null
  /** How it went, to start with: a shelf check's missing copies are lost. */
  as?: GoneInput['as']
}>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('visible', { required: true })
const notice = useNotice()
const id = useId()
const f = (name: string) => `${id}-${name}`

const input = ref<GoneInput>({ as: 'lost', on: today(), note: '' })
const busy = ref(false)
watch(open, (v) => {
  if (v) input.value = { as: props.as ?? 'lost', on: today(), note: '' }
})

async function save() {
  if (!props.copy) return
  busy.value = true
  const { error } = await api.PUT('/copies/{id}/gone', {
    params: { path: { id: props.copy.id } },
    body: input.value,
  })
  busy.value = false
  if (error) return notice.failed(error)
  notice.done(`${props.copy.barcode}: ${goneKindLabel(input.value.as).toLowerCase()}`)
  open.value = false
  emit('saved')
}
</script>

<template>
  <Dialog
    v-model:visible="open"
    modal
    :header="`${copy?.barcode ?? 'The copy'} has left the library`"
    class="w-full max-w-md"
  >
    <form :id="f('form')" class="flex flex-col gap-4" @submit.prevent="save">
      <p class="prose-text text-sm text-ink-muted">
        Its record stays, with the day and your note, and it leaves the shelves, the labels, and the
        counts. You can bring it back.
      </p>
      <FormField label="How it went" :for="f('as')">
        <Select
          v-model="input.as"
          :input-id="f('as')"
          :aria-labelledby="`${f('as')}-label`"
          :options="goneKinds"
          option-label="label"
          option-value="value"
        />
      </FormField>
      <FormField label="When" :for="f('on')" required>
        <InputText :id="f('on')" v-model="input.on" type="date" required />
      </FormField>
      <FormField label="Note" :for="f('note')" hint="To whom, for how much, or where">
        <InputText :id="f('note')" v-model="input.note" maxlength="500" />
      </FormField>
    </form>
    <template #footer>
      <Button type="button" severity="secondary" label="Cancel" @click="open = false" />
      <Button type="submit" :form="f('form')" label="Mark it gone" :loading="busy" />
    </template>
  </Dialog>
</template>
