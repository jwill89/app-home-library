import { useConfirm } from 'primevue/useconfirm'
import { messageOf } from '@/app/problem'
import type { ErrorModel } from '@/types'
import { useNotice } from './useNotice'

/** A record a form saves, stamped with when it last changed. */
interface Stamped {
  updatedAt?: string
}

/** What a form does with each answer to a save refused because its record moved. */
export interface StaleChoices<R extends Stamped> {
  /** The record as the server has it now: no data and no error when it is gone. */
  latest: () => Promise<{ data?: R; error?: unknown }>
  /** Puts the latest record into the form, in place of what was typed. */
  load: (latest: R) => unknown
  /** Saves the form's values again, carrying the latest record's stamp, so they go over it. */
  overwrite: (updatedAt: string | undefined) => unknown
}

/** Whether a failed save was refused because its record changed after the form loaded it. */
export function isStale(error: unknown): boolean {
  if (!error || typeof error !== 'object') {
    return false
  }
  const problem = error as ErrorModel
  return (
    problem.status === 409 && (problem.errors ?? []).some((e) => e.location === 'body.updatedAt')
  )
}

/**
 * The one way a form answers a save refused because its record changed on another device or
 * page since the form loaded it: the server's words, and the owner's choice between loading
 * the latest into the form (what was typed is dropped) and saving the form over it. Closing
 * the question leaves the form as it is, unsaved. It answers whether the failure was that
 * refusal; any other is the caller's to tell.
 */
export function useStaleSave() {
  const confirm = useConfirm()
  const notice = useNotice()
  return <R extends Stamped>(error: unknown, choices: StaleChoices<R>): boolean => {
    if (!isStale(error)) {
      return false
    }
    const latest = async (): Promise<R | undefined> => {
      const answer = await choices.latest()
      if (answer.error) {
        notice.failed(answer.error)
        return undefined
      }
      if (!answer.data) {
        notice.failed(new Error('It is no longer here: it was removed on another device or page.'))
        return undefined
      }
      return answer.data
    }
    confirm.require({
      header: 'Changed since you opened it',
      message: messageOf(error),
      rejectProps: { label: 'Load the latest', severity: 'secondary' },
      acceptProps: { label: 'Save mine over it' },
      // Loading the latest loses only what was typed here; saving over it loses the other change.
      defaultFocus: 'reject',
      reject: () => {
        void latest().then((r) => r && choices.load(r))
      },
      accept: () => {
        void latest().then((r) => r && choices.overwrite(r.updatedAt))
      },
    })
    return true
  }
}
