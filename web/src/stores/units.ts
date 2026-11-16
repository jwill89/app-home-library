import { defineStore } from 'pinia'
import type { Unit } from '@/app/format'
import { useRemembered } from '@/composables/useRemembered'

// The unit the owner measures in, imperial or metric: one choice for every measurement typed
// or shown (shelves, braces, a book's size), remembered on this device where storage allows.
// Lengths are always kept in millimetres.

export const useUnits = defineStore('units', () => {
  const unit = useRemembered<Unit>('hl.unit', ['in', 'cm'], 'in')
  return { unit }
})
