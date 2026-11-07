<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import { api, type BookInput, type Choices } from '@/api/client'
import { bookToInput, bookTrail, prepare } from '@/app/book'
import { useLibrary } from '@/app/library'
import { returnTo, type Crumb } from '@/app/navigation'
import { useNotice } from '@/app/problem'
import BookForm from '@/components/BookForm.vue'
import PageHeader from '@/components/PageHeader.vue'

const props = defineProps<{ id: number }>()
const router = useRouter()
const notice = useNotice()
const library = useLibrary()

const start = ref<BookInput | null>(null)
// The pages above, as the book was saved: its series is the one it is in until it is saved.
const trail = ref<Crumb[]>()
const book = ref<BookInput | null>(null)
const choices = ref<Choices | null>(null)
const saving = ref(false)

onMounted(async () => {
  const [b, c] = await Promise.all([
    api.GET('/books/{id}', { params: { path: { id: props.id } } }),
    api.GET('/choices'),
  ])
  if (b.error) {
    notice.failed(b.error)
    return
  }
  start.value = bookToInput(b.data)
  book.value = bookToInput(b.data)
  trail.value = [...bookTrail(b.data), { label: b.data.title, to: `/books/${props.id}` }]
  choices.value = c.data ?? null
})

async function save() {
  if (!start.value || !book.value) {
    return
  }
  saving.value = true
  const { error } = await api.PUT('/books/{id}', {
    params: { path: { id: props.id } },
    body: prepare(start.value, book.value),
  })
  saving.value = false
  if (error) {
    notice.failed(error)
    return
  }
  library.loaded = false
  notice.done('Saved')
  void returnTo(router, `/books/${props.id}`)
}
</script>

<template>
  <div class="mx-auto max-w-record">
    <PageHeader title="Edit book" :subtitle="start?.title" :trail="trail" />
    <form v-if="book" class="flex flex-col gap-6" @submit.prevent="save">
      <BookForm v-model="book" :choices="choices" />
      <div class="flex gap-2">
        <Button type="submit" label="Save" :loading="saving" />
        <Button
          type="button"
          severity="secondary"
          label="Cancel"
          @click="returnTo(router, `/books/${id}`)"
        />
      </div>
    </form>
  </div>
</template>
