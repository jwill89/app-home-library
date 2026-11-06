import { api, formData, type Document, type DocumentInput } from '@/api/client'
import type { CiteWork } from './cite'

/** The files a document takes, for a file picker's accept. */
export const documentFileTypes = 'application/pdf,image/jpeg,image/png,image/webp'

/** The one way a file joins a document: the document comes back with it, its text to be read. */
export function addDocumentFile(id: number, file: Blob, name: string) {
  return api.POST('/documents/{id}/files', { params: { path: { id } }, ...formData(file, name) })
}

// A document's shapes: a blank one, a saved one back into the form, and the work a citation
// is made from.

export function emptyDocument(courseId = 0): DocumentInput {
  return {
    kind: 'other',
    title: '',
    authors: [],
    editors: [],
    container: '',
    volume: '',
    issue: '',
    pages: '',
    published: '',
    publisher: '',
    doi: '',
    url: '',
    courseId,
    abstract: '',
    notes: '',
    subjects: [],
  }
}

export function documentToInput(d: Document): DocumentInput {
  return {
    kind: d.kind,
    title: d.title,
    authors: [...(d.authors ?? [])],
    editors: [...(d.editors ?? [])],
    container: d.container,
    volume: d.volume,
    issue: d.issue,
    pages: d.pages,
    published: d.published,
    publisher: d.publisher,
    doi: d.doi,
    url: d.url,
    courseId: d.courseId,
    abstract: d.abstract,
    notes: d.notes,
    subjects: [...(d.subjects ?? [])],
  }
}

export function documentCite(d: Document): CiteWork {
  return {
    // A book review is cited as the article it is.
    kind:
      d.kind === 'review'
        ? 'article'
        : d.kind === 'article' || d.kind === 'chapter'
          ? d.kind
          : 'document',
    authors: d.authors ?? [],
    editors: d.editors ?? [],
    title: d.title,
    container: d.container,
    volume: d.volume,
    issue: d.issue,
    pages: d.pages,
    published: d.published,
    publisher: d.publisher,
    doi: d.doi,
    url: d.url,
  }
}
