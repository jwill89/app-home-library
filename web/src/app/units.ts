import { defineStore } from 'pinia'
import { computed, ref, type WritableComputedRef } from 'vue'
import { fromUnit, inUnit, type Unit } from './format'
import { useRemembered } from './remember'

// The unit the owner measures in, imperial or metric: one choice for every measurement typed
// or shown (shelves, braces, a book's size), remembered on this device where storage allows.
// Lengths are always kept in millimetres.

export const useUnits = defineStore('units', () => {
  const unit = useRemembered<Unit>('hl.unit', ['in', 'cm'], 'in')
  return { unit }
})

export const unitOptions: { value: Unit; label: string }[] = [
  { value: 'in', label: 'Inches' },
  { value: 'cm', label: 'Centimetres' },
]

/**
 * A length kept in millimetres, shown and typed in the owner's unit. What was typed stays
 * shown while it still means the same millimetres, so a field never changes under the cursor.
 */
export function useLength(
  get: () => number,
  set: (mm: number) => void,
): WritableComputedRef<number | null> {
  const units = useUnits()
  const typed = ref<number | null>(null)
  return computed({
    get: () => {
      const mm = get()
      if (!mm) {
        return null
      }
      if (typed.value && fromUnit(typed.value, units.unit) === mm) {
        return typed.value
      }
      return inUnit(mm, units.unit)
    },
    set: (v) => {
      typed.value = v
      set(v ? fromUnit(v, units.unit) : 0)
    },
  })
}
