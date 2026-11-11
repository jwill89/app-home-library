<script setup lang="ts">
import { ref, useId } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import { messageOf } from '@/app/problem'
import { deviceName, useSession } from '@/app/session'
import FormField from '@/components/FormField.vue'
import PublicPage from '@/components/PublicPage.vue'

const session = useSession()
const router = useRouter()
const id = useId()
const code = ref('')
const label = ref(deviceName())
const busy = ref(false)
const problem = ref('')
const here = location.hostname

async function setUp() {
  problem.value = ''
  busy.value = true
  try {
    await session.redeem(code.value)
    await session.addPasskey(label.value)
    void router.replace('/')
  } catch (e) {
    problem.value =
      e instanceof Error && e.name === 'NotAllowedError'
        ? 'The passkey was not made. The code is spent; make a new one and try again.'
        : messageOf(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <PublicPage title="Set up this device">
    <p class="prose-text text-sm">
      Type a setup code to make a passkey for <span class="font-mono">{{ here }}</span>. Get one
      from Settings on a device that is signed in, or on the server with
      <span class="font-mono">homelibrary setup-code</span>.
    </p>
    <form class="flex flex-col gap-4" @submit.prevent="setUp">
      <FormField label="Setup code" :for="`${id}-code`" required>
        <InputText
          :id="`${id}-code`"
          v-model="code"
          autocomplete="one-time-code"
          autocapitalize="characters"
          class="font-mono text-xl tracking-widest"
          placeholder="ABCD-EFGH"
          required
        />
      </FormField>
      <FormField
        label="Name for this device"
        :for="`${id}-name`"
        hint="Shown in Settings, so you can tell your passkeys apart."
      >
        <InputText :id="`${id}-name`" v-model="label" maxlength="60" />
      </FormField>
      <Button
        type="submit"
        label="Make a passkey"
        :loading="busy"
        :disabled="code.trim().length < 8"
      />
    </form>
    <p v-if="problem" class="text-status-danger" role="alert">{{ problem }}</p>
    <RouterLink to="/sign-in" class="link-target self-start text-sm text-accent"
      >Back to sign in</RouterLink
    >
  </PublicPage>
</template>
