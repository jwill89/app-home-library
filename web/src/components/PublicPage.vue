<script setup lang="ts">
// The one frame for the pages seen before signing in: the library's mark beside the page's
// title, then its own words. There is no bar or navigation to skip past, so no skip link.
defineProps<{ title: string }>()
</script>

<template>
  <main class="mx-auto flex min-h-dvh max-w-narrow flex-col justify-center gap-5 p-6">
    <div class="flex items-center gap-3">
      <img src="/icons/icon.svg" alt="" class="size-12" />
      <h1 class="text-2xl">{{ title }}</h1>
    </div>
    <slot />
  </main>
</template>
