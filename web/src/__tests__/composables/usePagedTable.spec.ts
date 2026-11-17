import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ComponentPublicInstance } from 'vue'
import { usePagedTable } from '@/composables/usePagedTable'

// A table as PrimeVue draws it with a paginator above and below: only what the composable reads.
function drawn(page: number, last: number) {
  const paginator = (where: string) => `
    <div class="p-datatable-paginator-${where}">
      <button aria-label="Previous Page" ${page === 1 ? 'disabled' : ''}>‹</button>
      <button aria-label="Page 1" ${page === 1 ? 'aria-current="page"' : ''}>1</button>
      <button aria-label="Page 2" ${page === 2 ? 'aria-current="page"' : ''}>2</button>
      <button aria-label="Next Page" ${page === last ? 'disabled' : ''}>›</button>
    </div>`
  return `${paginator('top')}<table></table>${paginator('bottom')}`
}

describe('usePagedTable', () => {
  let root: HTMLElement
  const scrolled = vi.fn()

  function paged() {
    const p = usePagedTable()
    p.table.value = { $el: root } as unknown as ComponentPublicInstance
    return p
  }
  const button = (where: string, label: string) => {
    const el = root.querySelector<HTMLElement>(
      `.p-datatable-paginator-${where} [aria-label="${label}"]`,
    )
    if (!el) {
      throw new Error(`no ${label} ${where}`)
    }
    return el
  }

  beforeEach(() => {
    root = document.createElement('div')
    root.innerHTML = drawn(1, 3)
    document.body.append(root)
    Element.prototype.scrollIntoView = scrolled
  })
  afterEach(() => {
    root.remove()
    scrolled.mockReset()
  })

  it('brings the paginator above into view, and the focus to its same control', async () => {
    const p = paged()
    button('bottom', 'Next Page').focus()
    const turning = p.turned()
    root.innerHTML = drawn(2, 3) // the new page, drawn before the composable looks again
    await turning
    expect(scrolled).toHaveBeenCalledWith({ block: 'nearest' })
    expect(scrolled.mock.contexts[0]).toBe(root.querySelector('.p-datatable-paginator-top'))
    expect(document.activeElement).toBe(button('top', 'Next Page'))
  })

  it('gives the focus to the page shown when the same control is now disabled', async () => {
    root.innerHTML = drawn(1, 2)
    const p = paged()
    button('bottom', 'Next Page').focus()
    const turning = p.turned()
    root.innerHTML = drawn(2, 2)
    await turning
    expect(document.activeElement).toBe(button('top', 'Page 2'))
  })

  it('leaves the focus where it was for a page turned from above', async () => {
    const p = paged()
    const next = button('top', 'Next Page')
    next.focus()
    await p.turned()
    expect(scrolled).toHaveBeenCalledOnce()
    expect(document.activeElement).toBe(next)
  })

  it('names each paginator for where it stands', () => {
    const { paginatorContainer } = usePagedTable().pt.pcPaginator
    expect(paginatorContainer({ attrs: { 'data-p-top': true } })).toEqual({
      'aria-label': 'Pages, above the list',
    })
    expect(paginatorContainer({ attrs: { 'data-p-bottom': true } })).toEqual({
      'aria-label': 'Pages, below the list',
    })
  })

  it('does nothing for a table not drawn', async () => {
    const p = usePagedTable()
    await p.turned()
    expect(scrolled).not.toHaveBeenCalled()
  })
})
