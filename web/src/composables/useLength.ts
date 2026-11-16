import { computed, ref, type WritableComputedRef } from 'vue'
import { fromUnit, inUnit } from '@/app/format'
import { useUnits } from '@/stores/units'

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
