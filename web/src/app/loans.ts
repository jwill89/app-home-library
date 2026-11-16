import type { Loan, LoanInput } from '@/types'
import { calendarDate, count, daysLate, today } from '@/app/format'

// A loan's shapes and words, for the book's page and the Loans page alike.

export function loanInput(l: Loan): LoanInput {
  return {
    borrower: l.borrower,
    lentOn: l.lentOn,
    dueOn: l.dueOn,
    returnedOn: l.returnedOn,
    notes: l.notes,
    updatedAt: l.updatedAt,
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
  if (l.dueOn) {
    parts.push(`due back ${calendarDate(l.dueOn)}`)
  }
  return parts.join(', ')
}
