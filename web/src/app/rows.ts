/** Past this many books named at once, a list is fetched whole rather than book by book. */
export const maxRefresh = 200

/**
 * Brings a list up to date with the records the change feed named, fetched again: one asked for
 * and not in the answer is no longer listed, and one not in the list yet is new. A row is
 * replaced only when it differs, and in place, so the table draws that row alone and a menu open
 * on another stays open. Says whether anything changed.
 */
export function refreshRows<T>(
  list: T[],
  refs: ReadonlySet<number>,
  answer: readonly T[],
  id: (row: T) => number,
): boolean {
  const fresh = new Map(answer.map((r) => [id(r), r]))
  let changed = false
  for (const ref of refs) {
    const at = list.findIndex((r) => id(r) === ref)
    const now = fresh.get(ref)
    if (at < 0 && now) {
      list.push(now)
    } else if (at >= 0 && !now) {
      list.splice(at, 1)
    } else if (at >= 0 && now && JSON.stringify(list[at]) !== JSON.stringify(now)) {
      list[at] = now
    } else {
      continue
    }
    changed = true
  }
  return changed
}
