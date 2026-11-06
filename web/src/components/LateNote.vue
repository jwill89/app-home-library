<script setup lang="ts">
import { faCircleExclamation } from '@fortawesome/pro-regular-svg-icons'
import { lateLabel } from '@/app/loans'

// A loan past its due date: the words say it, the color and the mark repeat it.
defineProps<{ days: number }>()
</script>

<template>
  <span class="late"><FontAwesomeIcon :icon="faCircleExclamation" /> {{ lateLabel(days) }}</span>
</template>

<!-- In the components layer, so a page's display utilities (md:hidden) win over it. -->
<style scoped>
@layer components {
  .late {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--color-status-danger);
  }
}
</style>
