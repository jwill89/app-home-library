import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { recall, remember, useRemembered } from '@/composables/useRemembered'

beforeEach(() => {
  localStorage.clear()
})

/** Storage the browser refuses, as in a private window that allows none. */
function refuseStorage() {
  const refused = () => {
    throw new DOMException('The operation is insecure.', 'SecurityError')
  }
  vi.stubGlobal('localStorage', { getItem: refused, setItem: refused })
}

describe('recall and remember', () => {
  it('keeps a value under its key and gives it back', () => {
    remember('hl.unit', 'cm')
    expect(recall('hl.unit')).toBe('cm')
  })

  it('gives back nothing for a key never kept', () => {
    expect(recall('hl.unit')).toBe('')
  })

  it('carries on without storage', () => {
    refuseStorage()
    expect(() => {
      remember('hl.unit', 'cm')
    }).not.toThrow()
    expect(recall('hl.unit')).toBe('')
  })
})

describe('useRemembered', () => {
  it('starts from the choice kept, and keeps each new one', async () => {
    remember('hl.unit', 'cm')
    const unit = useRemembered('hl.unit', ['in', 'cm'], 'in')
    expect(unit.value).toBe('cm')
    unit.value = 'in'
    await nextTick()
    expect(recall('hl.unit')).toBe('in')
  })

  it('starts from the fallback when what was kept is not one of the choices', () => {
    remember('hl.unit', 'furlongs')
    expect(useRemembered('hl.unit', ['in', 'cm'], 'in').value).toBe('in')
  })

  it('starts from the fallback without storage', () => {
    refuseStorage()
    expect(useRemembered('hl.unit', ['in', 'cm'], 'cm').value).toBe('cm')
  })
})
