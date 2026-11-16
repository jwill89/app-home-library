<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import { api } from '@/api/client'
import type { Book, BookInput, Choices } from '@/types'
import { bookToInput, bookTrail, prepare } from '@/app/book'
import { useLibrary } from '@/stores/library'
import { returnTo, type Crumb } from '@/app/navigation'
import { useNotice } from '@/composables/useNotice'
import { useStaleSave } from '@/composables/useStaleSave'
import BookForm from '@/components/BookForm.vue'
import PageHeader from '@/components/PageHeader.vue'

const props = defineProps<{ id: number }>()
const router = useRouter()
const notice = useNotice()
const library = useLibrary()
const stale = useStaleSave()

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
  take(b.data)
  trail.value = [...bookTrail(b.data), { label: b.data.title, to: `/books/${props.id}` }]
  choices.value = c.data ?? null
})

function take(b: Book) {
  start.value = bookToInput(b)
  book.value = bookToInput(b)
}

async function save() {
  if (!start.value || !book.value) {
    return
  }
  saving.value = true
  const { data, error } = await api.PUT('/books/{id}', {
    params: { path: { id: props.id } },
    body: prepare(start.value, book.value),
  })
  saving.value = false
  if (error) {
    const refused = stale(error, {
      latest: () => api.GET('/books/{id}', { params: { path: { id: props.id } } }),
      load: take,
      overwrite: (updatedAt) => {
        if (book.value) {
          book.value.updatedAt = updatedAt
          return save()
        }
      },
    })
    if (!refused) {
      notice.failed(error)
    }
    return
  }
  book.value.updatedAt = data.updatedAt
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
