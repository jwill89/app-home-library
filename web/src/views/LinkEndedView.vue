<script setup lang="ts">
import PublicPage from '@/components/PublicPage.vue'
</script>

<template>
  <PublicPage title="This link has ended">
    <p class="prose-text">
      The library link you opened has been turned off, or the day it was good until has passed. Ask
      the person who sent it for a new one.
    </p>
    <p class="text-sm text-ink-muted">
      Your own library?
      <RouterLink to="/sign-in" class="text-accent">Sign in with a passkey</RouterLink>.
    </p>
  </PublicPage>
</template>
