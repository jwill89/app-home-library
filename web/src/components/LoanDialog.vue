<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import { api, type Loan, type LoanInput } from '@/api/client'
import { addDays, today } from '@/app/format'
import { loanInput } from '@/app/loans'
import { useNotice } from '@/app/problem'
import BookPicker from './BookPicker.vue'
import FormField from './FormField.vue'

// The one loan form: lending a copy, from its book's page or by choosing one here, and
// changing a loan already made.
const props = defineProps<{
  /** The loan to change; none to lend. */
  loan?: Loan | null
  /** The copy to lend; none to choose one. */
  copy?: { id: number; barcode: string } | null
}>()
const emit = defineEmits<{ saved: [loan: Loan] }>()
const open = defineModel<boolean>('visible', { required: true })
const notice = useNotice()
const id = useId()
const f = (name: string) => `${id}-${name}`

const blank = (): LoanInput => ({
  borrower: '',
  lentOn: today(),
  dueOn: '',
  returnedOn: '',
  notes: '',
})
const input = ref<LoanInput>(blank())
const copyId = ref(0)
const borrowers = ref<string[]>([])
const busy = ref(false)

watch(open, async (v) => {
  if (!v) return
  input.value = props.loan ? loanInput(props.loan) : blank()
  copyId.value = props.loan?.copyId ?? props.copy?.id ?? 0
  const { data } = await api.GET('/choices')
  borrowers.value = data?.borrowers ?? []
})

const header = computed(() => {
  if (props.loan) return `The loan of ${props.loan.barcode}`
  return props.copy ? `Lend ${props.copy.barcode}` : 'Lend a book'
})
const ready = computed(
  () => copyId.value > 0 && input.value.borrower.trim() !== '' && input.value.lentOn !== '',
)
// Typing a date is slow on a phone; the usual loans are a tap.
const dueIn = [
  { days: 14, label: 'In 2 weeks' },
  { days: 28, label: 'In 4 weeks' },
]

async function save() {
  busy.value = true
  const { data, error } = props.loan
    ? await api.PUT('/loans/{id}', { params: { path: { id: props.loan.id } }, body: input.value })
    : await api.POST('/copies/{id}/loans', {
        params: { path: { id: copyId.value } },
        body: input.value,
      })
  busy.value = false
  if (error) return notice.failed(error)
  open.value = false
  emit('saved', data)
}
</script>

<template>
  <Dialog v-model:visible="open" modal :header="header" class="w-full max-w-lg">
    <form :id="f('form')" class="flex flex-col gap-4" @submit.prevent="save">
      <BookPicker v-if="!loan && !copy" v-model="copyId" on-shelf />
      <FormField
        label="Lent to"
        :for="f('borrower')"
        hint="Someone new is added to your borrowers."
        required
      >
        <InputText
          :id="f('borrower')"
          v-model="input.borrower"
          :list="f('borrowers')"
          maxlength="100"
          autocomplete="off"
          required
        />
        <datalist :id="f('borrowers')">
          <option v-for="b in borrowers" :key="b" :value="b" />
        </datalist>
      </FormField>
      <div class="grid gap-4 sm:grid-cols-2">
        <FormField label="Lent on" :for="f('lent')" required>
          <InputText :id="f('lent')" v-model="input.lentOn" type="date" required />
        </FormField>
        <FormField
          label="Due back"
          :for="f('due')"
          hint="Optional. Past this day the loan shows as overdue."
        >
          <InputText :id="f('due')" v-model="input.dueOn" type="date" :min="input.lentOn" />
        </FormField>
      </div>
      <div class="-mt-2 flex flex-wrap gap-2">
        <Button
          v-for="d in dueIn"
          :key="d.days"
          type="button"
          text
          :label="`Due ${d.label.toLowerCase()}`"
          @click="input.dueOn = addDays(input.lentOn || today(), d.days)"
        />
      </div>
      <FormField
        v-if="loan"
        label="Returned on"
        :for="f('returned')"
        hint="Empty while the copy is still out."
      >
        <InputText :id="f('returned')" v-model="input.returnedOn" type="date" :min="input.lentOn" />
      </FormField>
      <FormField label="Notes" :for="f('notes')">
        <Textarea :id="f('notes')" v-model="input.notes" rows="2" maxlength="2000" auto-resize />
      </FormField>
    </form>
    <template #footer>
      <Button type="button" severity="secondary" label="Cancel" @click="open = false" />
      <Button
        type="submit"
        :form="f('form')"
        :label="loan ? 'Save the loan' : 'Lend it'"
        :loading="busy"
        :disabled="!ready"
      />
    </template>
  </Dialog>
</template>
