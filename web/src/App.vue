<script setup lang="ts">
import ConfirmDialog from 'primevue/confirmdialog'
import Toast from 'primevue/toast'
</script>

<template>
  <RouterView />
  <Toast position="bottom-center" />
  <Toast group="scan" position="top-center" />
  <ConfirmDialog :pt="{ root: { class: 'w-full max-w-md' } }" />
</template>
