import { api, type Loan, type LoanInput } from '@/api/client'
import { calendarDate, count, daysLate, today } from './format'
import { useNotice } from './problem'

// A loan's shapes and words, for the book's page and the Loans page alike.

export function loanInput(l: Loan): LoanInput {
  return {
    borrower: l.borrower,
    lentOn: l.lentOn,
    dueOn: l.dueOn,
    returnedOn: l.returnedOn,
    notes: l.notes,
  }
}

/** The loan with the copy back today, or on the day it went out if that is later. */
function returnedToday(l: Loan): LoanInput {
  const now = today()
  return { ...loanInput(l), returnedOn: now < l.lentOn ? l.lentOn : now }
}

/** Marking a copy back on the shelf today, and saying so; it answers whether it is. */
export function useReturnLoan(): (l: Loan) => Promise<boolean> {
  const notice = useNotice()
  return async (l) => {
    const { error } = await api.PUT('/loans/{id}', {
      params: { path: { id: l.id } },
      body: returnedToday(l),
    })
    if (error) {
      notice.failed(error)
      return false
    }
    notice.done(`${l.barcode} is back on the shelf`)
    return true
  }
}

/** Days past due for a copy still out, on the device's day; 0 when it is not late. */
export function overdueBy(l: Loan): number {
  return l.returnedOn ? 0 : daysLate(l.dueOn, today())
}

export function lateLabel(days: number): string {
  return `${count(days, 'day')} overdue`
}

/** Where a copy is, for its owner: lent to Ana on Oct 1, 2026, due back Oct 22, 2026. */
export function loanLine(l: Loan): string {
  const parts = [`Lent to ${l.borrower} on ${calendarDate(l.lentOn)}`]
  if (l.dueOn) parts.push(`due back ${calendarDate(l.dueOn)}`)
  return parts.join(', ')
}
