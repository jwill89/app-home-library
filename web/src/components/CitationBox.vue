<script setup lang="ts">
import { computed, useId } from 'vue'
import Button from 'primevue/button'
import SelectButton from 'primevue/selectbutton'
import { faCopy } from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import { cite, citeStyles, html, plainText, type CiteStyle, type CiteWork } from '@/app/cite'
import { useNotice } from '@/app/problem'
import { useRemembered } from '@/app/remember'
import FormField from './FormField.vue'
import { copyText } from '@/app/clipboard'

// The one way to cite a book or a document: the style chosen, the citation as it reads, and a
// copy that keeps its italics when pasted into a word processor.
const props = defineProps<{ work: CiteWork }>()
const notice = useNotice()
const id = useId()

// The style last chosen is remembered on this device, where storage allows.
const style = useRemembered<CiteStyle>('hl.citeStyle', ['apa', 'mla', 'chicago'], 'apa')

const pieces = computed(() => cite(props.work, style.value))

async function copy() {
  if (await copyText(plainText(pieces.value), html(pieces.value))) {
    notice.done('Citation copied')
  } else {
    notice.failed(new Error('The citation could not be copied. Select it and copy it by hand.'))
  }
}
</script>

<template>
  <div class="flex flex-col gap-3">
    <FormField label="Style" :for="`${id}-style`">
      <SelectButton
        :id="`${id}-style`"
        v-model="style"
        :aria-labelledby="`${id}-style-label`"
        :options="citeStyles"
        option-label="label"
        option-value="value"
        :allow-empty="false"
      />
    </FormField>
    <p class="prose-text citation">
      <template v-for="(p, i) in pieces" :key="i"
        ><i v-if="p.italic">{{ p.text }}</i
        ><template v-else>{{ p.text }}</template></template
      >
    </p>
    <div>
      <Button type="button" severity="secondary" @click="copy"
        ><FontAwesomeIcon :icon="faCopy" /> Copy the citation</Button
      >
    </div>
  </div>
</template>

<style scoped>
/* A citation hangs its second line, as a reference list does. */
.citation {
  padding-left: 2em;
  text-indent: -2em;
  white-space: normal;
  overflow-wrap: anywhere;
}
</style>
