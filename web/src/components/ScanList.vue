<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import type { Scan } from '@/api/client'
import { scanStatusLabel } from '@/app/format'
import BookCover from './BookCover.vue'

// The scans as they come back from the background, newest first: each with what the sources
// made of it and what can be done about it. A found book waits for the owner's Add.
defineProps<{ scans: Scan[] }>()
const emit = defineEmits<{
  add: [scan: Scan]
  want: [scan: Scan]
  remove: [scan: Scan]
  done: [scan: Scan]
  undo: [scan: Scan]
  copy: [scan: Scan]
  byHand: [scan: Scan]
}>()

const name = (s: Scan) => s.title || s.isbn13
const row = (data: unknown) => data as Scan
</script>

<template>
  <DataTable :value="scans" data-key="id">
    <Column header="Book">
      <template #body="{ data }">
        <div class="flex items-start gap-3">
          <BookCover
            :cover="row(data).cover || null"
            :url="row(data).coverUrl || undefined"
            :title="name(row(data))"
            :width="48"
          />
          <div class="min-w-0">
            <RouterLink v-if="row(data).bookId" :to="`/books/${row(data).bookId}`" class="row-link">
              {{ name(row(data)) }}
            </RouterLink>
            <span v-else-if="row(data).title" class="font-semibold">{{ row(data).title }}</span>
            <span v-else class="font-mono">{{ row(data).isbn13 }}</span>
            <div v-if="row(data).authors" class="text-sm text-ink-muted">
              {{ row(data).authors }}
            </div>
            <div class="text-sm" :class="`scans__status--${row(data).status.replace('_', '-')}`">
              {{
                row(data).wantedOn && row(data).status !== 'pending'
                  ? 'On your wishlist'
                  : scanStatusLabel(row(data).status)
              }}
            </div>
          </div>
        </div>
      </template>
    </Column>
    <Column header="Actions" header-class="column-unlabelled">
      <template #body="{ data }">
        <div class="flex flex-col items-end gap-1 md:flex-row md:justify-end">
          <Button
            v-if="row(data).status === 'found'"
            type="button"
            severity="secondary"
            label="Add"
            :aria-label="`Add ${name(row(data))}`"
            @click="emit('add', row(data))"
          />
          <Button
            v-if="row(data).status === 'found'"
            type="button"
            text
            label="Want it"
            :aria-label="`Put ${name(row(data))} on the wishlist`"
            @click="emit('want', row(data))"
          />
          <Button
            v-if="row(data).status === 'owned'"
            type="button"
            text
            :label="row(data).wantedOn ? 'Got it' : 'Add a copy'"
            :aria-label="
              row(data).wantedOn
                ? `Got ${name(row(data))}: put it in the library`
                : `Add another copy of ${name(row(data))}`
            "
            @click="emit('copy', row(data))"
          />
          <Button
            v-if="row(data).status === 'not_found' || row(data).status === 'failed'"
            type="button"
            text
            label="Add by hand"
            :aria-label="`Add ${row(data).isbn13} by hand`"
            @click="emit('byHand', row(data))"
          />
          <Button
            v-if="row(data).status === 'added'"
            type="button"
            text
            severity="danger"
            label="Undo"
            :aria-label="`Undo: remove ${name(row(data))} from the library`"
            @click="emit('undo', row(data))"
          />
          <Button
            v-if="row(data).status === 'found' || row(data).status === 'pending'"
            type="button"
            text
            label="Remove"
            :aria-label="`Take ${name(row(data))} off the list`"
            @click="emit('remove', row(data))"
          />
          <Button
            v-else-if="row(data).status !== 'adding'"
            type="button"
            text
            label="Done"
            :aria-label="`Done with ${name(row(data))}`"
            @click="emit('done', row(data))"
          />
        </div>
      </template>
    </Column>
  </DataTable>
</template>

<style scoped>
@layer components {
  /* The words say what happened; the color only repeats it. */
  .scans__status--pending,
  .scans__status--adding,
  .scans__status--owned {
    color: var(--color-ink-muted);
  }

  .scans__status--found,
  .scans__status--added {
    color: var(--color-status-success);
  }

  .scans__status--not-found,
  .scans__status--failed {
    color: var(--color-status-danger);
  }
}
</style>
