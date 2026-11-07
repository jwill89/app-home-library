<script setup lang="ts">
import { ref, useTemplateRef } from 'vue'
import Button from 'primevue/button'
import { faCamera, faImage } from '@awesome.me/kit-4f2bd9b614/icons/classic/regular'
import CropDialog from './CropDialog.vue'

// The one way a photo comes from the owner, of a cover or of a document's page: taken now
// (the phone's own camera) or an image file, straightened and cropped, then handed to the
// page as a JPEG.
const props = withDefaults(defineProps<{ subject?: 'cover' | 'page'; size?: number }>(), {
  subject: 'cover',
  size: 1600,
})
const emit = defineEmits<{ picked: [blob: Blob] }>()

const camera = useTemplateRef<HTMLInputElement>('camera')
const chooser = useTemplateRef<HTMLInputElement>('chooser')
const file = ref<File | null>(null)
const cropping = ref(false)

function chosen(e: Event) {
  const input = e.target as HTMLInputElement
  const f = input.files?.[0]
  input.value = ''
  if (!f) {
    return
  }
  file.value = f
  cropping.value = true
}
</script>

<template>
  <div class="flex flex-wrap gap-2">
    <Button type="button" severity="secondary" @click="camera?.click()">
      <FontAwesomeIcon :icon="faCamera" /> Take a photo
    </Button>
    <Button type="button" severity="secondary" @click="chooser?.click()">
      <FontAwesomeIcon :icon="faImage" /> Choose an image
    </Button>
    <input
      ref="camera"
      type="file"
      accept="image/*"
      capture="environment"
      class="hidden"
      :aria-label="`Take a photo of the ${props.subject}`"
      @change="chosen"
    />
    <input
      ref="chooser"
      type="file"
      accept="image/jpeg,image/png,image/webp"
      class="hidden"
      :aria-label="`Choose an image of the ${props.subject}`"
      @change="chosen"
    />
    <CropDialog
      v-model:visible="cropping"
      :file="file"
      :subject="subject"
      :size="size"
      @done="emit('picked', $event)"
    />
  </div>
</template>
