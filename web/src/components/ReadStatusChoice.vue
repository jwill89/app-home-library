<script setup lang="ts">
import Select from 'primevue/select'
import type { BookRow, ReadStatus } from '@/api/client'
import { readStatuses, readStatusLabel } from '@/app/format'
import { useLibrary } from '@/app/library'
import { useNotice } from '@/app/problem'
import { useSession } from '@/app/session'

// A book's reading status in a row of a table: a choice for the owner, kept as soon as it is
// made, and the word for a visitor. A click on it is not a click on the row.
const props = defineProps<{ book: Pick<BookRow, 'id' | 'title' | 'readStatus'> }>()
const session = useSession()
const library = useLibrary()
const notice = useNotice()

async function choose(v: ReadStatus) {
  const { error } = await library.setStatus(props.book.id, v)
  if (error) {
    notice.failed(error)
  }
}
</script>

<template>
  <Select
    v-if="session.isOwner"
    :model-value="book.readStatus"
    :options="readStatuses"
    option-label="label"
    option-value="value"
    :aria-label="`Reading status of ${book.title}`"
    class="status-choice"
    @click.stop
    @update:model-value="choose"
  />
  <span v-else class="status" :class="`status--${book.readStatus}`">{{
    readStatusLabel(book.readStatus)
  }}</span>
</template>

<style scoped>
/* Wide enough for the longest choice, so the column does not shift as one changes. */
.status-choice {
  min-width: 7.75rem;
}

/* The word says the status; the color only repeats it. */
.status {
  font-size: 0.875rem;
  white-space: nowrap;
}

.status--unread {
  color: var(--color-ink-muted);
}

.status--reading {
  color: var(--color-accent);
  font-weight: 600;
}

.status--read {
  color: var(--color-status-success);
}
</style>
