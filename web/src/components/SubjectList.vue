<script setup lang="ts">
// The one way a record's subjects are shown, a book's or a document's: as tags, under their
// heading, and nothing at all when there are none.
defineProps<{ subjects: string[] }>()
</script>

<template>
  <section v-if="subjects.length">
    <h2 class="mb-2 text-lg">Subjects</h2>
    <ul class="flex flex-wrap gap-2">
      <li
        v-for="s in subjects"
        :key="s"
        class="rounded-control bg-highlight px-2 py-1 text-sm text-highlight-ink"
      >
        {{ s }}
      </li>
    </ul>
  </section>
</template>
