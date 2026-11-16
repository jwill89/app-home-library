import { api } from '@/api/client'
import { today } from '@/app/format'
import { loanInput } from '@/app/loans'
import type { Loan, LoanInput } from '@/types'
import { useNotice } from './useNotice'
import { useStaleSave } from './useStaleSave'

/** The loan with the copy back today, or on the day it went out if that is later. */
function returnedToday(l: Loan): LoanInput {
  const now = today()
  return { ...loanInput(l), returnedOn: now < l.lentOn ? l.lentOn : now }
}

/**
 * Marking a copy back on the shelf today, and saying so; it answers whether it is. `changed`
 * runs once the loan is as the owner chose: marked back, or, when it had changed on another
 * device first, loaded as it is now or marked back over that change.
 */
export function useReturnLoan(changed: () => unknown): (l: Loan) => Promise<boolean> {
  const notice = useNotice()
  const stale = useStaleSave()
  const back = async (l: Loan): Promise<boolean> => {
    const { error } = await api.PUT('/loans/{id}', {
      params: { path: { id: l.id } },
      body: returnedToday(l),
    })
    if (error) {
      const refused = stale(error, {
        latest: async () => {
          const { data, error: failed } = await api.GET('/loans')
          return { data: data?.loans?.find((x) => x.id === l.id), error: failed }
        },
        load: changed,
        overwrite: (updatedAt) => back({ ...l, updatedAt }),
      })
      if (!refused) {
        notice.failed(error)
      }
      return false
    }
    notice.done(`${l.barcode} is back on the shelf`)
    await changed()
    return true
  }
  return back
}
