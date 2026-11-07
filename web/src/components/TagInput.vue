<script setup lang="ts">
import { ref, useId } from 'vue'
import InputText from 'primevue/inputtext'
import { faXmark } from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'

// A list of names typed one at a time (Enter or a comma adds one), with the names already
// in use offered as the owner types.
const props = defineProps<{ suggestions?: string[]; placeholder?: string; inputId?: string }>()
const model = defineModel<string[] | null>({ required: true })

const draft = ref('')
const listId = useId()

function add() {
  const v = draft.value.trim().replace(/,$/, '').trim()
  draft.value = ''
  if (!v) {
    return
  }
  const list = model.value ?? []
  if (!list.some((x) => x.toLowerCase() === v.toLowerCase())) {
    model.value = [...list, v]
  }
}

function remove(i: number) {
  model.value = (model.value ?? []).filter((_, j) => j !== i)
}

function key(e: KeyboardEvent) {
  if (e.key === 'Enter' || e.key === ',') {
    e.preventDefault()
    add()
  } else if (e.key === 'Backspace' && draft.value === '' && (model.value ?? []).length) {
    remove((model.value ?? []).length - 1)
  }
}
</script>

<template>
  <div class="tags">
    <ul v-if="model?.length" class="tags__list">
      <li v-for="(tag, i) in model" :key="tag" class="tags__tag">
        {{ tag }}
        <button type="button" class="tags__remove" :aria-label="`Remove ${tag}`" @click="remove(i)">
          <FontAwesomeIcon :icon="faXmark" />
        </button>
      </li>
    </ul>
    <InputText
      :id="props.inputId"
      v-model="draft"
      :list="listId"
      :placeholder="placeholder"
      class="w-full"
      @keydown="key"
      @blur="add"
    />
    <datalist :id="listId">
      <option v-for="s in suggestions ?? []" :key="s" :value="s" />
    </datalist>
  </div>
</template>

<style scoped>
.tags__list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  margin: 0 0 0.4rem;
  padding: 0;
  list-style: none;
}

.tags__tag {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.15rem 0.2rem 0.15rem 0.6rem;
  border-radius: 999px;
  background: var(--color-highlight);
  font-size: 0.875rem;

  /* A long subject ("Human evolution -- relation to ethics -- 19th century") wraps inside its
     tag on a phone, rather than squeezing the button beside it. */
  max-width: 100%;
  overflow-wrap: anywhere;
}

.tags__remove {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--spacing-target);
  height: var(--spacing-target);
  border: none;
  border-radius: 999px;
  background: transparent;
  color: inherit;
  cursor: pointer;
}
</style>
