<script setup lang="ts">
import { computed, onMounted, ref, useId } from 'vue'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import { faBookArrowRight } from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import { api, type Borrower, type BorrowerInput, type Loan } from '@/api/client'
import { useConfirmAction } from '@/app/confirm'
import { useFeed } from '@/app/feed'
import { calendarDate } from '@/app/format'
import { loanLine, overdueBy, useReturnLoan } from '@/app/loans'
import { useNotice } from '@/app/problem'
import BookCover from '@/components/BookCover.vue'
import FormField from '@/components/FormField.vue'
import LateNote from '@/components/LateNote.vue'
import LoanDialog from '@/components/LoanDialog.vue'
import PageHeader from '@/components/PageHeader.vue'
import PagePanel from '@/components/PagePanel.vue'
import LoansCalendar from '@/components/LoansCalendar.vue'

const ask = useConfirmAction()
const notice = useNotice()
const id = useId()

const loans = ref<Loan[]>([])
const borrowers = ref<Borrower[]>([])
const loaded = ref(false)

async function load() {
  const [l, b] = await Promise.all([api.GET('/loans'), api.GET('/borrowers')])
  if (l.error) {
    notice.failed(l.error)
    return
  }
  if (b.error) {
    notice.failed(b.error)
    return
  }
  loans.value = l.data.loans ?? []
  borrowers.value = b.data.borrowers ?? []
  loaded.value = true
}
onMounted(load)
// A copy lent or brought back, here or on another device.
useFeed(['loan'], () => load())

// The longest overdue first, then by the day each is due, the soonest first and one with no
// due day last; then the earliest lent.
const out = computed(() =>
  loans.value
    .filter((l) => !l.returnedOn)
    .map((l) => ({ ...l, late: overdueBy(l) }))
    .sort(
      (a, b) =>
        b.late - a.late ||
        (a.dueOn || '9999').localeCompare(b.dueOn || '9999') ||
        a.lentOn.localeCompare(b.lentOn),
    ),
)
const overdue = computed(() => out.value.filter((l) => l.late > 0).length)
const summary = computed(() => {
  if (!loaded.value) {
    return ''
  }
  if (!out.value.length) {
    return 'Every copy is on the shelf'
  }
  const parts = [`${out.value.length} out`]
  if (overdue.value) {
    parts.push(`${overdue.value} overdue`)
  }
  return parts.join(' · ')
})

const ALL = 0
const who = ref(ALL)
const history = computed(() =>
  loans.value.filter((l) => l.returnedOn && (who.value === ALL || l.borrowerId === who.value)),
)
const whoOptions = computed(() => [
  { value: ALL, label: 'Everyone' },
  ...borrowers.value.map((b) => ({ value: b.id, label: b.name })),
])

// One loan dialog lends a book or changes a loan.
const loanOpen = ref(false)
const editing = ref<Loan | null>(null)
function lend() {
  editing.value = null
  loanOpen.value = true
}
function editLoan(l: Loan) {
  editing.value = l
  loanOpen.value = true
}
async function saved() {
  await load()
}

const returnLoan = useReturnLoan()
async function returned(l: Loan) {
  if (await returnLoan(l)) {
    await load()
  }
}

function removeLoan(l: Loan) {
  ask(
    {
      header: `Remove this loan from the history?`,
      message: `The record of ${l.barcode} lent to ${l.borrower} on ${calendarDate(l.lentOn)} is deleted. The copy and the borrower stay.`,
      accept: 'Remove the loan',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/loans/{id}', { params: { path: { id: l.id } } })
      if (error) {
        notice.failed(error)
        return
      }
      await load()
    },
  )
}

const borrowerOpen = ref(false)
const borrower = ref<{ id: number; input: BorrowerInput } | null>(null)
function editBorrower(b: Borrower) {
  borrower.value = { id: b.id, input: { name: b.name, contact: b.contact, notes: b.notes } }
  borrowerOpen.value = true
}
async function saveBorrower() {
  if (!borrower.value) {
    return
  }
  const { id: bid, input } = borrower.value
  const { error } = await api.PUT('/borrowers/{id}', { params: { path: { id: bid } }, body: input })
  if (error) {
    notice.failed(error)
    return
  }
  borrowerOpen.value = false
  await load()
}

function removeBorrower(b: Borrower) {
  if (b.out > 0) {
    notice.failed(
      new Error(
        `${b.name} still has ${b.out === 1 ? 'a copy' : `${b.out} copies`}. Mark it returned first.`,
      ),
    )
    return
  }
  const past = b.loans === 1 ? 'their one past loan' : `their ${b.loans} past loans`
  ask(
    {
      header: `Remove ${b.name}?`,
      message: `${b.name} and ${past} are deleted from the history. Your copies stay.`,
      accept: `Remove ${b.name}`,
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/borrowers/{id}', { params: { path: { id: b.id } } })
      if (error) {
        notice.failed(error)
        return
      }
      if (who.value === b.id) {
        who.value = ALL
      }
      await load()
    },
  )
}
</script>

<template>
  <div class="mx-auto max-w-page">
    <PageHeader title="Loans" :subtitle="summary">
      <Button type="button" @click="lend"
        ><FontAwesomeIcon :icon="faBookArrowRight" /> Lend a book</Button
      >
    </PageHeader>

    <PagePanel title="Out now">
      <DataTable :value="out" data-key="id" :loading="!loaded">
        <template #empty>
          <p class="text-ink-muted">Every copy is on the shelf.</p>
        </template>
        <Column header="Book">
          <template #body="{ data }">
            <div class="flex items-start gap-3">
              <BookCover
                :cover="(data as Loan).cover"
                :title="(data as Loan).title"
                :width="48"
                class="max-sm:hidden"
              />
              <div class="min-w-0">
                <RouterLink :to="`/books/${(data as Loan).bookId}`" class="row-link">{{
                  (data as Loan).title
                }}</RouterLink>
                <div class="text-sm text-ink-muted">{{ (data as Loan).authors }}</div>
                <div class="font-mono text-sm tabular">{{ (data as Loan).barcode }}</div>
                <!-- On a phone the loan folds under its book, as the author does in the library. -->
                <div class="text-sm md:hidden">{{ loanLine(data as Loan) }}</div>
                <LateNote
                  v-if="(data as { late: number }).late"
                  :days="(data as { late: number }).late"
                  class="md:hidden"
                />
              </div>
            </div>
          </template>
        </Column>
        <Column
          field="borrower"
          header="Lent to"
          header-class="max-md:hidden"
          body-class="max-md:hidden"
        />
        <Column header="Lent on" header-class="max-lg:hidden" body-class="max-lg:hidden">
          <template #body="{ data }">{{ calendarDate((data as Loan).lentOn) }}</template>
        </Column>
        <Column header="Due back" header-class="max-md:hidden" body-class="max-md:hidden">
          <template #body="{ data }">
            <span v-if="(data as Loan).dueOn">{{ calendarDate((data as Loan).dueOn) }}</span>
            <span v-else class="text-ink-muted">No date</span>
            <div v-if="(data as { late: number }).late">
              <LateNote :days="(data as { late: number }).late" />
            </div>
          </template>
        </Column>
        <Column header="Actions" header-class="column-unlabelled">
          <template #body="{ data }">
            <div class="flex flex-col items-end gap-1 md:flex-row md:justify-end">
              <Button
                type="button"
                severity="secondary"
                label="Returned"
                :aria-label="`${(data as Loan).barcode} is back: mark it returned today`"
                @click="returned(data as Loan)"
              />
              <Button
                type="button"
                text
                label="Edit"
                :aria-label="`Edit the loan of ${(data as Loan).barcode}`"
                @click="editLoan(data as Loan)"
              />
            </div>
          </template>
        </Column>
      </DataTable>
    </PagePanel>

    <PagePanel title="Borrowers" class="mt-6">
      <DataTable :value="borrowers" data-key="id" :loading="!loaded">
        <template #empty>
          <p class="text-ink-muted">
            No one yet. A borrower is added the first time you lend them a book.
          </p>
        </template>
        <Column header="Name">
          <template #body="{ data }">
            {{ (data as Borrower).name }}
            <div v-if="(data as Borrower).contact" class="text-sm text-ink-muted break-all">
              {{ (data as Borrower).contact }}
            </div>
          </template>
        </Column>
        <Column field="out" header="Has now" body-class="tabular" />
        <Column
          field="loans"
          header="Loans"
          header-class="max-sm:hidden"
          body-class="max-sm:hidden tabular"
        />
        <Column header="Actions" header-class="column-unlabelled">
          <template #body="{ data }">
            <div class="flex flex-col items-end gap-1 md:flex-row md:justify-end">
              <Button
                type="button"
                text
                label="Edit"
                :aria-label="`Edit ${(data as Borrower).name}`"
                @click="editBorrower(data as Borrower)"
              />
              <Button
                type="button"
                text
                severity="danger"
                label="Remove"
                :aria-label="`Remove ${(data as Borrower).name}`"
                @click="removeBorrower(data as Borrower)"
              />
            </div>
          </template>
        </Column>
      </DataTable>
    </PagePanel>

    <PagePanel title="History" class="mt-6">
      <template v-if="borrowers.length" #actions>
        <Select
          v-model="who"
          :options="whoOptions"
          option-label="label"
          option-value="value"
          aria-label="Whose loans"
        />
      </template>
      <DataTable
        :value="history"
        data-key="id"
        :loading="!loaded"
        :paginator="history.length > 25"
        :rows="25"
      >
        <template #empty>
          <p class="text-ink-muted">No loans have come back yet.</p>
        </template>
        <Column header="Book">
          <template #body="{ data }">
            <RouterLink :to="`/books/${(data as Loan).bookId}`" class="row-link">{{
              (data as Loan).title
            }}</RouterLink>
            <div class="font-mono text-sm tabular">{{ (data as Loan).barcode }}</div>
            <div class="text-sm md:hidden">{{ loanLine(data as Loan) }}</div>
            <div class="text-sm md:hidden">
              Returned {{ calendarDate((data as Loan).returnedOn) }}
            </div>
          </template>
        </Column>
        <Column
          field="borrower"
          header="Lent to"
          header-class="max-md:hidden"
          body-class="max-md:hidden"
        />
        <Column header="Lent on" header-class="max-lg:hidden" body-class="max-lg:hidden">
          <template #body="{ data }">{{ calendarDate((data as Loan).lentOn) }}</template>
        </Column>
        <Column header="Returned" header-class="max-md:hidden" body-class="max-md:hidden">
          <template #body="{ data }">{{ calendarDate((data as Loan).returnedOn) }}</template>
        </Column>
        <Column header="Actions" header-class="column-unlabelled">
          <template #body="{ data }">
            <div class="flex flex-col items-end gap-1 md:flex-row md:justify-end">
              <Button
                type="button"
                text
                label="Edit"
                :aria-label="`Edit the loan of ${(data as Loan).barcode} to ${(data as Loan).borrower}`"
                @click="editLoan(data as Loan)"
              />
              <Button
                type="button"
                text
                severity="danger"
                label="Remove"
                :aria-label="`Remove the loan of ${(data as Loan).barcode} to ${(data as Loan).borrower}`"
                @click="removeLoan(data as Loan)"
              />
            </div>
          </template>
        </Column>
      </DataTable>
    </PagePanel>

    <LoanDialog v-model:visible="loanOpen" :loan="editing" @saved="saved" />

    <Dialog
      v-model:visible="borrowerOpen"
      modal
      :header="borrower ? `Edit ${borrower.input.name || 'the borrower'}` : ''"
      class="w-full max-w-lg"
    >
      <form
        v-if="borrower"
        :id="`${id}-borrower`"
        class="flex flex-col gap-4"
        @submit.prevent="saveBorrower"
      >
        <FormField label="Name" :for="`${id}-name`" required>
          <InputText :id="`${id}-name`" v-model="borrower.input.name" maxlength="100" required />
        </FormField>
        <FormField
          label="How to reach them"
          :for="`${id}-contact`"
          hint="A phone number or an email address"
        >
          <InputText :id="`${id}-contact`" v-model="borrower.input.contact" maxlength="200" />
        </FormField>
        <FormField label="Notes" :for="`${id}-notes`">
          <Textarea
            :id="`${id}-notes`"
            v-model="borrower.input.notes"
            rows="2"
            maxlength="2000"
            auto-resize
          />
        </FormField>
      </form>
      <template #footer>
        <Button type="button" severity="secondary" label="Cancel" @click="borrowerOpen = false" />
        <Button
          type="submit"
          :form="`${id}-borrower`"
          label="Save"
          :disabled="!borrower?.input.name.trim()"
        />
      </template>
    </Dialog>
    <LoansCalendar class="mt-6" />
  </div>
</template>
