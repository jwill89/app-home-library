import { ref, watch, type Ref } from 'vue'

// What the owner chose last, kept on this device where the browser allows: a preference, never
// the library's data. Without storage each choice simply starts afresh.

/** The value kept under key, or empty when there is none or storage is refused. */
export function recall(key: string): string {
  try {
    return localStorage.getItem(key) ?? ''
  } catch {
    return ''
  }
}

/** Keeps value under key, where storage allows. */
export function remember(key: string, value: string): void {
  try {
    localStorage.setItem(key, value)
  } catch {
    // Not kept: chosen again next time.
  }
}

/** One of a few choices, recalled at first and remembered whenever it changes. */
export function useRemembered<T extends string>(
  key: string,
  choices: readonly T[],
  fallback: T,
): Ref<T> {
  const kept = recall(key)
  const choice = ref((choices as readonly string[]).includes(kept) ? kept : fallback) as Ref<T>
  watch(choice, (v) => remember(key, v))
  return choice
}

/** The section of the last book added, the likeliest for the next: books come in by shelf. */
export const lastSectionKey = 'hl.lastSection'
