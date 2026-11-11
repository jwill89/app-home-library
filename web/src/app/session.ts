import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { startAuthentication, startRegistration } from '@simplewebauthn/browser'
import type {
  PublicKeyCredentialCreationOptionsJSON,
  PublicKeyCredentialRequestOptionsJSON,
} from '@simplewebauthn/browser'
import { api, isOffline, type Me, type Role, type Sees } from '@/api/client'
import { forgetCatalog } from './offline'
import { ProblemError } from './problem'
import { recall, remember } from './remember'

/** A name for a new passkey that says which device made it. */
export function deviceName(): string {
  const ua = navigator.userAgent
  if (/Android/i.test(ua)) {
    return 'Android phone'
  }
  if (/iPhone|iPad/i.test(ua)) {
    return 'iPhone or iPad'
  }
  if (/Windows/i.test(ua)) {
    return 'Windows computer'
  }
  if (/Mac/i.test(ua)) {
    return 'Mac'
  }
  return 'This device'
}

const everything = (): Sees => ({ books: true, wishlist: true, papers: true })

/** Who a session is, as far as an answer from the server can change it. */
export interface Asker {
  role: Role
  isVisitor: boolean
  forget: () => void
}

/**
 * Follows an answer from the server. A 401 says a sign-in has gone stale or a visitor's link
 * has ended, so the session is forgotten. A 401 from sign-in's own operations is a sign-in that
 * failed: it stays with the page that tried it, and the session is left as it was.
 *
 * @param schemaPath - the operation's path in the API document, such as `/auth/passkey/finish`
 * @returns the page that says so, or nothing when the answer changes nothing
 */
export function afterAnswer(session: Asker, status: number, schemaPath: string) {
  if (status !== 401 || schemaPath.startsWith('/auth/') || session.role === 'none') {
    return undefined
  }
  const visitor = session.isVisitor
  session.forget()
  return visitor ? '/link-ended' : '/sign-in'
}

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

  function take(me: Me | undefined) {
    role.value = me?.role ?? 'none'
    name.value = me?.name ?? ''
    sees.value = me?.sees ?? everything()
    remember('hl.role', role.value)
  }

  async function load() {
    const { data, response } = await api.GET('/me')
    if (isOffline(response)) {
      // No connection: who this device last knew, so the installed app opens. The server
      // judges again once it can be asked.
      role.value = (recall('hl.role') || 'none') as Role
    } else {
      take(data)
    }
    loaded.value = true
  }

  /** After the server has said no one is signed in: the next page asks again. */
  function forget() {
    role.value = 'none'
    name.value = ''
    sees.value = everything()
  }

  /** Signs in with a passkey this device offers for this name. */
  async function signIn() {
    const begun = await api.POST('/auth/passkey/begin')
    if (begun.error) {
      throw new ProblemError(begun.error)
    }
    const optionsJSON = begun.data.publicKey as PublicKeyCredentialRequestOptionsJSON
    const credential = await startAuthentication({ optionsJSON })
    const { data, error } = await api.POST('/auth/passkey/finish', { body: { credential } })
    if (error) {
      throw new ProblemError(error)
    }
    take(data)
    loaded.value = true
  }

  /** Makes a passkey on this device and this name, signed in or holding a redeemed code. */
  async function addPasskey(label = deviceName()) {
    const begun = await api.POST('/passkeys/begin')
    if (begun.error) {
      throw new ProblemError(begun.error)
    }
    const optionsJSON = begun.data.publicKey as PublicKeyCredentialCreationOptionsJSON
    const credential = await startRegistration({ optionsJSON })
    const { error } = await api.POST('/passkeys', { body: { name: label, credential } })
    if (error) {
      throw new ProblemError(error)
    }
    // A device that made its first passkey with a setup code is signed in by it.
    await load()
  }

  /** Spends a setup code, so this device may make a passkey for the next ten minutes. */
  async function redeem(code: string) {
    const { error } = await api.POST('/auth/setup', { body: { code } })
    if (error) {
      throw new ProblemError(error)
    }
  }

  async function signOut() {
    const { error } = await api.POST('/auth/logout')
    if (error) {
      throw new ProblemError(error)
    }
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
