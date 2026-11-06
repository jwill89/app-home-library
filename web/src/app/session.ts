import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { startAuthentication, startRegistration } from '@simplewebauthn/browser'
import type {
  PublicKeyCredentialCreationOptionsJSON,
  PublicKeyCredentialRequestOptionsJSON,
} from '@simplewebauthn/browser'
import { api, isOffline, type Role, type Sees } from '@/api/client'
import { forgetCatalog } from './offline'
import { recall, remember } from './remember'

// The passkey ceremonies live under /auth, outside the API: their bodies are WebAuthn's own.
async function authPost<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method: 'POST',
    credentials: 'same-origin',
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!res.ok) {
    const problem = (await res.json().catch(() => ({}))) as { detail?: string }
    throw new Error(problem.detail ?? `Sign-in failed (${res.status}).`)
  }
  return (res.status === 204 ? undefined : await res.json()) as T
}

/** A name for a new passkey that says which device made it. */
export function deviceName(): string {
  const ua = navigator.userAgent
  if (/Android/i.test(ua)) return 'Android phone'
  if (/iPhone|iPad/i.test(ua)) return 'iPhone or iPad'
  if (/Windows/i.test(ua)) return 'Windows computer'
  if (/Mac/i.test(ua)) return 'Mac'
  return 'This device'
}

const everything = (): Sees => ({ books: true, wishlist: true, papers: true })

export const useSession = defineStore('session', () => {
  const loaded = ref(false)
  const role = ref<Role>('none')
  // The owner's name, for the owner and for a visitor, who sees whose library it is.
  const name = ref('')
  const isOwner = computed(() => role.value === 'owner')
  const isVisitor = computed(() => role.value === 'visitor')
  // Which of the library's pages have anything for the asker: a visitor's link may show books
  // without papers, or papers alone. The page a visitor starts on is the first it has.
  const sees = ref<Sees>(everything())
  const home = computed(() => (sees.value.books ? '/' : '/papers'))

  async function load() {
    const { data, response } = await api.GET('/me')
    if (isOffline(response)) {
      // No connection: who this device last knew, so the installed app opens. The server
      // judges again once it can be asked.
      role.value = (recall('hl.role') as Role) || 'none'
    } else {
      role.value = data?.role ?? 'none'
      name.value = data?.name ?? ''
      sees.value = data?.sees ?? everything()
      remember('hl.role', role.value)
    }
    loaded.value = true
  }

  /** After the server has said no one is signed in: the next page asks again. */
  function forget() {
    role.value = 'none'
    name.value = ''
    sees.value = everything()
  }

  async function signIn() {
    const options = await authPost<{ publicKey: PublicKeyCredentialRequestOptionsJSON }>(
      '/auth/passkey/login/begin',
    )
    const assertion = await startAuthentication({ optionsJSON: options.publicKey })
    await authPost('/auth/passkey/login/finish', assertion)
    await load()
  }

  /** Makes a passkey on this device and this name, signed in or holding a redeemed code. */
  async function addPasskey(label = deviceName()) {
    const options = await authPost<{ publicKey: PublicKeyCredentialCreationOptionsJSON }>(
      '/auth/passkey/register/begin',
    )
    const attestation = await startRegistration({ optionsJSON: options.publicKey })
    await authPost(`/auth/passkey/register/finish?name=${encodeURIComponent(label)}`, attestation)
    await load()
  }

  async function redeem(code: string) {
    await authPost('/auth/setup', { code })
  }

  async function signOut() {
    await authPost('/auth/logout')
    forget()
    remember('hl.role', 'none')
    await forgetCatalog()
  }

  return {
    loaded,
    role,
    name,
    isOwner,
    isVisitor,
    sees,
    home,
    load,
    forget,
    signIn,
    addPasskey,
    redeem,
    signOut,
  }
})
