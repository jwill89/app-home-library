<script setup lang="ts">
import { computed, onMounted, ref, useId } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import SelectButton from 'primevue/selectbutton'
import {
  faFileCode,
  faFileCsv,
  faKey,
  faRightFromBracket,
} from '@fortawesome/pro-regular-svg-icons'
import { api, apiURL, type Passkey, type SetupCode } from '@/api/client'
import { useConfirmAction } from '@/app/confirm'
import { day } from '@/app/format'
import { librarySearch, librarySearchLink } from '@/app/libraryLink'
import { useNotice } from '@/app/problem'
import { scheme, schemes } from '@/app/scheme'
import { useSession } from '@/app/session'
import FormField from '@/components/FormField.vue'
import PageHeader from '@/components/PageHeader.vue'
import PagePanel from '@/components/PagePanel.vue'
import ShareLinks from '@/components/ShareLinks.vue'

const session = useSession()
const router = useRouter()
const ask = useConfirmAction()
const notice = useNotice()
const passkeys = ref<Passkey[]>([])
const code = ref<SetupCode | null>(null)
const here = location.hostname
const id = useId()

// The library's search, for finding a paper there; one it cannot search with says so.
const libraryProblem = computed(() =>
  librarySearch.value.trim() && !librarySearchLink(librarySearch.value, 'test')
    ? 'Paste the EBSCO search page your library opens (research.ebsco.com/c/…), or an https search link with {query} where the words go.'
    : '',
)

async function load() {
  const { data, error } = await api.GET('/passkeys')
  if (error) return notice.failed(error)
  passkeys.value = data.passkeys ?? []
}
onMounted(load)

async function addHere() {
  try {
    await session.addPasskey()
    notice.done('Passkey added for this device')
    await load()
  } catch (e) {
    notice.failed(e)
  }
}

async function makeCode() {
  const { data, error } = await api.POST('/setup-codes')
  if (error) return notice.failed(error)
  code.value = data
}

function remove(p: Passkey) {
  ask(
    {
      header: `Remove the passkey “${p.name}”?`,
      message: `That device will no longer sign in on ${p.site}. It can be set up again with a code.`,
      accept: 'Remove the passkey',
      danger: true,
    },
    async () => {
      const { error } = await api.DELETE('/passkeys/{id}', { params: { path: { id: p.id } } })
      if (error) return notice.failed(error)
      await load()
    },
  )
}

async function signOut() {
  await session.signOut()
  void router.push('/sign-in')
}

const until = (s: string) =>
  new Date(s).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
</script>

<template>
  <div class="mx-auto max-w-record">
    <PageHeader title="Settings">
      <Button type="button" severity="secondary" @click="signOut">
        <FontAwesomeIcon :icon="faRightFromBracket" /> Sign out
      </Button>
    </PageHeader>

    <PagePanel title="Appearance">
      <FormField
        label="Colors"
        :for="`${id}-scheme`"
        hint="Kept on this device. Matching it follows its light or dark setting as that changes."
      >
        <SelectButton
          :id="`${id}-scheme`"
          v-model="scheme"
          :options="[...schemes]"
          option-label="label"
          option-value="value"
          :allow-empty="false"
          :aria-labelledby="`${id}-scheme-label`"
        />
      </FormField>
    </PagePanel>

    <PagePanel title="Your library" class="mt-6">
      <FormField
        label="Library search"
        :for="`${id}-library`"
        hint="Kept on this device. The page your library's databases open on, as EBSCO's does: research.ebsco.com/c/…/search. A paper's Find a copy searches it, and you sign in there with your card."
      >
        <InputText
          :id="`${id}-library`"
          v-model="librarySearch"
          type="url"
          inputmode="url"
          autocomplete="off"
          :aria-invalid="!!libraryProblem"
          :aria-describedby="libraryProblem ? `${id}-library-problem` : undefined"
        />
        <p
          v-if="libraryProblem"
          :id="`${id}-library-problem`"
          class="prose-text mt-1 text-sm text-status-danger"
          role="alert"
        >
          {{ libraryProblem }}
        </p>
      </FormField>
    </PagePanel>

    <PagePanel title="Passkeys" class="mt-6">
      <p class="prose-text mb-4 text-sm text-ink-muted">
        A passkey works on the address it was made on, and the library answers on two, so it takes
        one passkey for each. A password manager that syncs passkeys (1Password, Google Password
        Manager, iCloud Keychain) puts each on all your devices; without one, every device makes its
        own. This page is on <span class="font-mono">{{ here }}</span
        >.
      </p>
      <DataTable :value="passkeys" data-key="id" class="mb-4">
        <template #empty><p class="text-ink-muted">No passkeys.</p></template>
        <Column header="Device">
          <template #body="{ data }">
            {{ (data as Passkey).name }}
            <!-- On a phone the address folds under the device, as the author does in the library. -->
            <div class="font-mono text-sm break-all text-ink-muted md:hidden">
              {{ (data as Passkey).site }}
            </div>
          </template>
        </Column>
        <Column
          field="site"
          header="Address"
          header-class="max-md:hidden"
          body-class="max-md:hidden font-mono"
        />
        <Column header="Last used">
          <template #body="{ data }">{{ day((data as Passkey).lastUsedAt) }}</template>
        </Column>
        <Column header="Remove" header-class="column-unlabelled">
          <template #body="{ data }">
            <Button
              type="button"
              text
              severity="danger"
              label="Remove"
              :aria-label="`Remove the passkey ${(data as Passkey).name} on ${(data as Passkey).site}`"
              @click="remove(data as Passkey)"
            />
          </template>
        </Column>
      </DataTable>
      <div class="flex flex-wrap gap-2">
        <Button type="button" @click="addHere"
          ><FontAwesomeIcon :icon="faKey" /> Add a passkey on this device</Button
        >
        <Button
          type="button"
          severity="secondary"
          label="Set up another device or address"
          @click="makeCode"
        />
      </div>
      <div v-if="code" class="mt-4" role="status">
        <p class="prose-text text-sm">
          On the other device, open the library's address and go to
          <span class="font-mono">/setup</span>. Type this code there before
          {{ until(code.expiresAt) }}:
        </p>
        <p class="mt-2 font-mono text-3xl tracking-widest">{{ code.code }}</p>
      </div>
    </PagePanel>

    <ShareLinks class="mt-6" />

    <PagePanel title="Export" class="mt-6">
      <p class="prose-text mb-4 text-sm text-ink-muted">
        Everything you have entered, to keep elsewhere or take to another app. The CSV has a row for
        each copy, for a spreadsheet. The JSON file holds the whole library: books, copies, loans,
        courses, papers' details, and shelves. Covers and papers' files are not in either; the
        backups keep them.
      </p>
      <div class="flex flex-wrap gap-2">
        <Button as="a" :href="apiURL('/export/books.csv')" download
          ><FontAwesomeIcon :icon="faFileCsv" /> Download the CSV</Button
        >
        <Button as="a" severity="secondary" :href="apiURL('/export/library.json')" download
          ><FontAwesomeIcon :icon="faFileCode" /> Download the JSON</Button
        >
      </div>
    </PagePanel>
  </div>
</template>
