import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { api, isOffline, type BookRow, type ReadStatus } from '@/api/client'
import { listen } from './feed'
import { today } from './format'
import { messageOf } from './problem'
import { savedCatalog, saveCatalog } from './offline'
import { maxRefresh, refreshRows } from './rows'
import { useSession } from './session'

/** The filter value that filters nothing: a real value, so the select shows its label. */
export const ALL = 'all'

// The library list and how it is being looked at. It lives in a store so the search and
// filters are still there when the owner comes back from a book.
export const useLibrary = defineStore('library', () => {
  const rows = ref<BookRow[]>([])
  const loaded = ref(false)
  const search = ref('')
  const section = ref(ALL)
  const status = ref(ALL)
  const format = ref(ALL)
  const category = ref(ALL)
  // Where in the list the owner was, so going back to it from a book lands on the same page.
  const first = ref(0)
  const pageRows = ref(50)
  const sortField = ref('title')
  const sortOrder = ref<1 | -1 | 0>(1)
  // A new search or filter is a new list, read from its start.
  watch([search, section, status, format, category], () => (first.value = 0))

  // When the list came from this device, for want of a connection: the day it was kept.
  const offlineSince = ref('')
  // Why the list could not be read, when the server answered with an error.
  const failed = ref('')

  async function load() {
    const { data, error, response } = await api.GET('/books')
    if (isOffline(response)) {
      const saved = await savedCatalog()
      rows.value = saved?.rows ?? []
      offlineSince.value = saved?.savedAt ?? ''
      failed.value = ''
    } else if (error) {
      // An error is not an empty library: the list there is kept, and the copy kept on this
      // device for scanning without a connection is not overwritten.
      failed.value = messageOf(error)
    } else {
      rows.value = data.books ?? []
      offlineSince.value = ''
      failed.value = ''
      if (useSession().isOwner) {
        void saveCatalog(rows.value)
        follow()
      }
    }
    loaded.value = true
  }

  // Once loaded, the list keeps up with the change feed for as long as the app is open: a book
  // changed here, on another device, or by the background lookup is fetched again, alone.
  let following = false
  function follow() {
    if (following) {
      return
    }
    following = true
    listen(['book'], (refs, all) => refresh(refs, all))
  }

  async function refresh(refs: ReadonlySet<number>, all: boolean) {
    if (all || refs.size > maxRefresh) {
      return load()
    }
    const { data, error } = await api.GET('/books', { params: { query: { ids: [...refs] } } })
    if (error) {
      return
    }
    if (refreshRows(rows.value, refs, data.books ?? [], (b) => b.id)) {
      void saveCatalog(rows.value)
    }
  }

  /**
   * The one way a book's reading status changes: begun on this device's day where the book has
   * none, finished on no day the owner did not give. The list keeps up without loading again.
   */
  async function setStatus(id: number, readStatus: ReadStatus) {
    const result = await api.PUT('/books/{id}/read-status', {
      params: { path: { id } },
      body: { readStatus, on: today() },
    })
    const row = rows.value.find((r) => r.id === id)
    if (result.data && row) {
      row.readStatus = result.data.readStatus
    }
    return result
  }

  const sections = computed(() =>
    [...new Set(rows.value.flatMap((r) => r.sections.split(',').filter(Boolean)))].sort(),
  )

  // The books in the house, and the ones wanted: a wanted book has no copy yet.
  const books = computed(() => rows.value.filter((r) => !r.wantedOn))
  const wishlist = computed(() => rows.value.filter((r) => r.wantedOn))

  const filtered = computed(() => {
    const found = matching(search.value)
    return books.value.filter((r) => {
      if (section.value !== ALL && !r.sections.split(',').includes(section.value)) {
        return false
      }
      if (status.value !== ALL && r.readStatus !== status.value) {
        return false
      }
      if (format.value !== ALL && r.format !== format.value) {
        return false
      }
      if (category.value !== ALL && r.category !== category.value) {
        return false
      }
      return found(r)
    })
  })

  return {
    rows,
    offlineSince,
    failed,
    books,
    wishlist,
    loaded,
    search,
    section,
    status,
    format,
    category,
    first,
    pageRows,
    sortField,
    sortOrder,
    sections,
    filtered,
    load,
    setStatus,
  }
})

/**
 * The one search over the library: every word somewhere in what a row is known by, from its
 * title and people to its subjects, genres, and class numbers.
 */
export function matching(query: string): (r: BookRow) => boolean {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  return (r) => {
    if (!words.length) {
      return true
    }
    const hay = [
      r.title,
      r.subtitle,
      r.authors,
      r.credits,
      r.series,
      r.isbn13,
      r.ddc,
      r.lcc,
      r.publisher,
      r.barcode,
      r.sections,
      r.genres,
      ...(r.subjects ?? []),
    ]
      .join(' ')
      .toLowerCase()
    return words.every((w) => hay.includes(w))
  }
}
