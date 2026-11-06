<script setup lang="ts">
import ProgressBar from 'primevue/progressbar'
import { count } from '@/app/format'

// The background lookup's progress: a line and a bar, the counts beside them when asked.
defineProps<{
  pending: number
  waiting: number
  done: number
  failed: number
  progress: number
  counts?: boolean
}>()
</script>

<template>
  <div role="status">
    <p class="prose-text mb-2 text-sm">
      <slot>
        Looking up {{ count(pending, 'book') }} in the background for Dewey numbers, sizes, and
        covers.
      </slot>
    </p>
    <ProgressBar
      :value="progress"
      :show-value="false"
      class="h-2"
      aria-label="Background lookup progress"
    />
    <p v-if="counts" class="mt-2 text-sm text-ink-muted">
      {{ done }} done, {{ pending }} to go<template v-if="waiting"
        >, {{ waiting }} to try again later</template
      ><template v-if="failed">, {{ failed }} could not be reached</template>.
    </p>
  </div>
</template>
