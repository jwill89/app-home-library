import { afterEach, describe, expect, it, vi } from 'vitest'
import { createApp, defineComponent, h, ref, type Ref } from 'vue'
import { useWidth } from '@/composables/useWidth'

// jsdom draws nothing and has no ResizeObserver: one stands in that the test tells of a change.
let changed: (() => void) | undefined
const disconnect = vi.fn()
class Observer {
  constructor(callback: () => void) {
    changed = callback
  }
  observe = vi.fn()
  disconnect = disconnect
}

afterEach(() => {
  changed = undefined
  disconnect.mockClear()
})

function mounted() {
  vi.stubGlobal('ResizeObserver', Observer)
  let width: Ref<number> | undefined
  const el = ref<HTMLElement | null>(null)
  const app = createApp(
    defineComponent(() => {
      width = useWidth(el, (e) => Number(e.dataset.width))
      return () => h('div', { ref: el, 'data-width': '320' })
    }),
  )
  const root = document.createElement('div')
  app.mount(root)
  return { app, el, width: width! }
}

describe('useWidth', () => {
  it('reads the width once drawn, and again whenever it changes size', () => {
    const { app, el, width } = mounted()
    expect(width.value).toBe(320)
    el.value?.setAttribute('data-width', '1180')
    changed?.()
    expect(width.value).toBe(1180)
    app.unmount()
    expect(disconnect).toHaveBeenCalled()
  })

  it('reads an element’s own width unless told what to measure', () => {
    vi.stubGlobal('ResizeObserver', Observer)
    let width: Ref<number> | undefined
    const app = createApp(
      defineComponent(() => {
        const el = ref<HTMLElement | null>(null)
        width = useWidth(el)
        return () => h('div', { ref: el })
      }),
    )
    app.mount(document.createElement('div'))
    expect(width?.value).toBe(0)
    app.unmount()
  })
})
