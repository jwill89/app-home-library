<script setup lang="ts">
// A label, its control in the slot, and a line under it: where the value came from, or help.
// The label's id is the control's with "-label": a Select names itself by aria-labelledby,
// since a label's `for` only reaches form elements and a Select's box is not one.
defineProps<{ label: string; for: string; hint?: string; required?: boolean }>()
</script>

<template>
  <div class="flex min-w-0 flex-col gap-1">
    <label :id="`${$props.for}-label`" :for="$props.for" class="text-sm font-medium">
      {{ label }}<span v-if="required" class="text-status-danger" aria-hidden="true"> *</span>
      <span v-if="required" class="sr-only"> (required)</span>
    </label>
    <slot />
    <small v-if="hint" class="text-ink-muted">{{ hint }}</small>
  </div>
</template>
