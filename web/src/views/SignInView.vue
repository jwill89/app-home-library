<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import { faKey } from '@fortawesome/pro-regular-svg-icons'
import { messageOf } from '@/app/problem'
import { useSession } from '@/app/session'
import PublicPage from '@/components/PublicPage.vue'

const session = useSession()
const route = useRoute()
const router = useRouter()
const busy = ref(false)
const problem = ref('')

async function signIn() {
  problem.value = ''
  busy.value = true
  try {
    await session.signIn()
    const next =
      typeof route.query.next === 'string' && route.query.next.startsWith('/')
        ? route.query.next
        : '/'
    void router.replace(next)
  } catch (e) {
    // Closing the passkey prompt is a choice, not a failure.
    if (!(e instanceof Error && e.name === 'NotAllowedError')) problem.value = messageOf(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <PublicPage title="Home Library">
    <Button type="button" size="large" :loading="busy" @click="signIn"
      ><FontAwesomeIcon :icon="faKey" /> Sign in with a passkey</Button
    >
    <p v-if="problem" class="text-status-danger" role="alert">{{ problem }}</p>
    <p class="text-sm text-ink-muted">
      No passkey for this address yet?
      <RouterLink to="/setup" class="text-accent">Set it up with a code</RouterLink>.
    </p>
  </PublicPage>
</template>
