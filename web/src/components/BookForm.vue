<script setup lang="ts">
import { computed, useId } from 'vue'
import Button from 'primevue/button'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Rating from 'primevue/rating'
import Select from 'primevue/select'
import SelectButton from 'primevue/selectbutton'
import Textarea from 'primevue/textarea'
import { faPlus, faXmark } from '@fortawesome/pro-regular-svg-icons'
import type { BookInput, Choices } from '@/api/client'
import { categories, formats, readStatuses, roles, sourceLabel } from '@/app/format'
import FormField from './FormField.vue'
import PagePanel from './PagePanel.vue'
import TagInput from './TagInput.vue'
import LengthField from './LengthField.vue'
import UnitChoice from './UnitChoice.vue'

// The one book form, for adding and for editing. It edits the model in place; the page
// decides what to send.
defineProps<{ choices?: Choices | null }>()
const book = defineModel<BookInput>({ required: true })

const id = useId()
const f = (name: string) => `${id}-${name}`

// Cleared, a category is worked out again from the details: the server keeps it empty.
const category = computed({
  get: () => book.value.category,
  set: (v: BookInput['category'] | null) => (book.value.category = v ?? ''),
})

function from(field: string): string {
  const s = book.value.sources?.[field]
  return s && s !== 'owner' ? `From ${sourceLabel(s)}` : ''
}

function addPerson() {
  book.value.contributors = [...(book.value.contributors ?? []), { name: '', role: 'author' }]
}

function removePerson(i: number) {
  book.value.contributors = (book.value.contributors ?? []).filter((_, j) => j !== i)
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <PagePanel fieldset title="The book">
      <div class="grid gap-4 md:grid-cols-2">
        <FormField
          label="Title"
          :for="f('title')"
          :hint="from('title')"
          required
          class="md:col-span-2"
        >
          <InputText :id="f('title')" v-model="book.title" required />
        </FormField>
        <FormField label="Subtitle" :for="f('subtitle')" class="md:col-span-2">
          <InputText :id="f('subtitle')" v-model="book.subtitle" />
        </FormField>
      </div>
      <fieldset class="mt-4 flex min-w-0 flex-col gap-2 border-0 p-0">
        <legend class="mb-1 text-sm font-medium">People</legend>
        <div v-for="(person, i) in book.contributors ?? []" :key="i" class="flex gap-2">
          <InputText
            v-model="person.name"
            class="min-w-0 flex-1"
            :aria-label="`Person ${i + 1}`"
            placeholder="Name"
          />
          <Select
            v-model="person.role"
            :options="roles"
            option-label="label"
            option-value="value"
            class="w-36"
            :aria-label="`Role of person ${i + 1}`"
          />
          <Button
            type="button"
            text
            severity="secondary"
            :aria-label="`Remove person ${i + 1}`"
            @click="removePerson(i)"
          >
            <FontAwesomeIcon :icon="faXmark" />
          </Button>
        </div>
        <div>
          <Button type="button" text size="small" @click="addPerson"
            ><FontAwesomeIcon :icon="faPlus" /> Add a person</Button
          >
        </div>
        <small v-if="from('contributors')" class="text-ink-muted">{{ from('contributors') }}</small>
      </fieldset>
      <div class="mt-4 grid grid-cols-[1fr_6rem] gap-4">
        <FormField label="Series" :for="f('series')" :hint="from('series')">
          <InputText :id="f('series')" v-model="book.series" :list="f('series-list')" />
          <datalist :id="f('series-list')">
            <option v-for="s in choices?.series ?? []" :key="s" :value="s" />
          </datalist>
        </FormField>
        <FormField label="Number" :for="f('number')">
          <InputText :id="f('number')" v-model="book.seriesNumber" inputmode="decimal" />
        </FormField>
      </div>
      <FormField
        label="Category"
        :for="f('category')"
        :hint="from('category') || 'Left blank, it is worked out from the details.'"
        class="mt-4"
      >
        <Select
          v-model="category"
          :input-id="f('category')"
          :aria-labelledby="`${f('category')}-label`"
          :options="[...categories]"
          option-label="label"
          option-value="value"
          placeholder="Worked out from the details"
          show-clear
        />
      </FormField>
    </PagePanel>

    <PagePanel fieldset title="This edition">
      <div class="grid gap-4 md:grid-cols-3">
        <FormField label="ISBN" :for="f('isbn')">
          <InputText :id="f('isbn')" v-model="book.isbn" inputmode="numeric" class="font-mono" />
        </FormField>
        <FormField label="Format" :for="f('format')" :hint="from('format')">
          <Select
            v-model="book.format"
            :input-id="f('format')"
            :aria-labelledby="`${f('format')}-label`"
            :options="formats"
            option-label="label"
            option-value="value"
          />
        </FormField>
        <FormField label="Pages" :for="f('pages')" :hint="from('pages')">
          <InputNumber v-model="book.pages" :input-id="f('pages')" :min="0" :use-grouping="false" />
        </FormField>
        <FormField label="Publisher" :for="f('publisher')" :hint="from('publisher')">
          <InputText :id="f('publisher')" v-model="book.publisher" />
        </FormField>
        <FormField label="Place" :for="f('place')">
          <InputText :id="f('place')" v-model="book.publishPlace" />
        </FormField>
        <FormField
          label="Published"
          :for="f('published')"
          :hint="from('published') || 'YYYY, YYYY-MM, or YYYY-MM-DD'"
        >
          <InputText :id="f('published')" v-model="book.published" inputmode="numeric" />
        </FormField>
        <FormField label="Edition" :for="f('edition')">
          <InputText
            :id="f('edition')"
            v-model="book.editionStatement"
            placeholder="1st pbk. ed."
          />
        </FormField>
        <FormField label="Language" :for="f('language')" hint="Two letters: en, ja, fr">
          <InputText :id="f('language')" v-model="book.language" maxlength="10" />
        </FormField>
      </div>
      <div class="mt-4 grid gap-4 md:grid-cols-3">
        <FormField label="Dewey" :for="f('ddc')" :hint="from('ddc')">
          <InputText :id="f('ddc')" v-model="book.ddc" inputmode="decimal" class="font-mono" />
        </FormField>
        <FormField label="Library of Congress class" :for="f('lcc')" :hint="from('lcc')">
          <InputText :id="f('lcc')" v-model="book.lcc" class="font-mono" />
        </FormField>
        <FormField label="LCCN" :for="f('lccn')">
          <InputText :id="f('lccn')" v-model="book.lccn" class="font-mono" />
        </FormField>
      </div>
      <div class="mt-4 flex flex-col gap-4">
        <UnitChoice />
        <div class="grid grid-cols-3 gap-4">
          <LengthField v-model="book.heightMm" label="Height" :hint="from('heightMm')" />
          <LengthField v-model="book.widthMm" label="Width" :hint="from('widthMm')" />
          <LengthField v-model="book.thicknessMm" label="Thickness" :hint="from('thicknessMm')" />
        </div>
      </div>
      <div class="mt-4 grid gap-4 md:grid-cols-2">
        <FormField label="Genres" :for="f('genres')" hint="Your own list. Press Enter after each.">
          <TagInput
            v-model="book.genres"
            :input-id="f('genres')"
            :suggestions="choices?.genres ?? []"
          />
        </FormField>
        <FormField label="Subjects" :for="f('subjects')" :hint="from('subjects')">
          <TagInput v-model="book.subjects" :input-id="f('subjects')" />
        </FormField>
      </div>
      <FormField
        label="Description"
        :for="f('description')"
        :hint="from('description')"
        class="mt-4"
      >
        <Textarea :id="f('description')" v-model="book.description" rows="5" auto-resize />
      </FormField>
    </PagePanel>

    <PagePanel fieldset title="Reading">
      <div class="grid gap-4 md:grid-cols-2">
        <FormField label="Status" :for="f('status')">
          <SelectButton
            :id="f('status')"
            v-model="book.readStatus"
            :aria-labelledby="`${f('status')}-label`"
            :options="readStatuses"
            option-label="label"
            option-value="value"
            :allow-empty="false"
          />
        </FormField>
        <FormField v-if="book.readStatus === 'reading'" label="Page reached" :for="f('page')">
          <InputNumber
            v-model="book.currentPage"
            :input-id="f('page')"
            :min="0"
            :use-grouping="false"
          />
        </FormField>
        <FormField label="Started" :for="f('started')">
          <InputText :id="f('started')" v-model="book.startedOn" type="date" />
        </FormField>
        <FormField label="Finished" :for="f('finished')">
          <InputText :id="f('finished')" v-model="book.finishedOn" type="date" />
        </FormField>
        <FormField label="Rating" :for="f('rating')">
          <Rating :id="f('rating')" v-model="book.rating" />
        </FormField>
      </div>
      <FormField label="Notes" :for="f('notes')" class="mt-4">
        <Textarea :id="f('notes')" v-model="book.notes" rows="3" auto-resize />
      </FormField>
    </PagePanel>
  </div>
</template>
