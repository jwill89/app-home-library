import { nextTick, ref, type ComponentPublicInstance } from 'vue'

// Each paginator is a navigation landmark, so the two are named for where they stand (axe's
// landmark-unique). The table marks its paginator above with data-p-top, which PrimeVue hands a
// pass-through function as the paginator's attrs.
const pt = {
  pcPaginator: {
    paginatorContainer: ({ attrs }: { attrs: Record<string, unknown> }) => ({
      'aria-label': attrs['data-p-top'] ? 'Pages, above the list' : 'Pages, below the list',
    }),
  },
}

/**
 * A paged table, with its paginator above the rows as well as below them:
 *
 *     const paged = usePagedTable()
 *     <DataTable :ref="paged.table" :pt="paged.pt" paginator paginator-position="both"
 *       @page="paged.turned">
 *
 * A page turned brings the paginator above back into view, so the new page reads from its first
 * row rather than from its last. Turned from the paginator below, the focus goes to the same
 * control in the one above (Next for Next), so the keyboard carries on from there; a page turned
 * from above leaves both where they are.
 */
export function usePagedTable() {
  const table = ref<ComponentPublicInstance | null>(null)

  async function turned() {
    const root: unknown = table.value?.$el
    if (!(root instanceof HTMLElement)) {
      return
    }
    // Taken now: the new page can disable the control it was turned with (Next, on the last).
    const from = document.activeElement
    const below = from instanceof HTMLElement && !!from.closest('.p-datatable-paginator-bottom')
    const label = below ? from.getAttribute('aria-label') : null
    await nextTick()
    const above = root.querySelector<HTMLElement>('.p-datatable-paginator-top')
    if (!above) {
      return
    }
    // Nearest: no move while it is in view, and below the app's bar when it is not (the
    // page's scroll-padding-top).
    above.scrollIntoView({ block: 'nearest' })
    if (!below) {
      return
    }
    const same = [...above.querySelectorAll<HTMLElement>('[aria-label]')].find(
      (el) => el.getAttribute('aria-label') === label && !el.hasAttribute('disabled'),
    )
    ;(same ?? above.querySelector<HTMLElement>('[aria-current="page"]'))?.focus({
      preventScroll: true,
    })
  }

  return { table, pt, turned }
}
