<script setup lang="ts">
import { nextTick, ref, useId } from 'vue'
import AutoComplete from 'primevue/autocomplete'
import type { DocumentRow } from '@/api/client'
import FormField from './FormField.vue'

// The one way to choose a paper from the library: a word of its title, an author, or where it
// appeared, typed. Each one chosen is said and the box empties for the next.
const props = defineProps<{ papers: DocumentRow[]; label: string; hint?: string }>()
const emit = defineEmits<{ chosen: [paper: DocumentRow] }>()
const id = useId()

const picked = ref<DocumentRow | string | null>(null)
const matches = ref<DocumentRow[]>([])
function search(query: string) {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  matches.value = props.papers
    .filter((p) => {
      const hay = [p.title, ...(p.authors ?? []), p.container].join(' ').toLowerCase()
      return words.every((w) => hay.includes(w))
    })
    .slice(0, 8)
}

function chose(v: DocumentRow | string | null) {
  if (!v || typeof v === 'string') return
  emit('chosen', v)
  // Emptied once the box has taken the choice: emptied at once, it never sees a change.
  void nextTick(() => (picked.value = null))
}
</script>

<template>
  <FormField
    :label="label"
    :for="`${id}-paper`"
    :hint="hint ?? 'Type a word of its title, an author, or the journal or book it is in.'"
  >
    <AutoComplete
      v-model="picked"
      :input-id="`${id}-paper`"
      :suggestions="matches"
      option-label="title"
      force-selection
      fluid
      @complete="search($event.query)"
      @update:model-value="chose"
    >
      <template #option="{ option }">
        <div class="min-w-0">
          <div>{{ (option as DocumentRow).title }}</div>
          <div class="text-sm text-ink-muted">
            {{
              [
                (option as DocumentRow).authors?.join(', '),
                (option as DocumentRow).published.slice(0, 4),
              ]
                .filter(Boolean)
                .join(' · ')
            }}
          </div>
        </div>
      </template>
    </AutoComplete>
  </FormField>
</template>
