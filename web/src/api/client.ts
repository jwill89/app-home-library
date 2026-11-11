import createClient from 'openapi-fetch'
import type { components, operations, paths } from './schema'

// The one API client. Types come from the generated schema; never hand-write them. A list in
// a query is sent as huma reads one: comma-separated in a single parameter.
export const api = createClient<paths>({
  baseUrl: '/api/v1',
  credentials: 'same-origin',
  querySerializer: { array: { style: 'form', explode: false } },
})

// With no connection, every request answers as the server would with a problem it could not
// help, marked so a page that can do without the server (the library list, Scan) knows.
api.use({
  onError() {
    const problem = {
      status: 503,
      title: 'No connection',
      detail: 'There is no connection. This works again when the device is back online.',
    }
    return new Response(JSON.stringify(problem), {
      status: 503,
      headers: { 'Content-Type': 'application/problem+json', 'X-Offline': '1' },
    })
  },
})

/** Whether an answer is the device's own, made for want of a connection. */
export const isOffline = (response?: Response) => response?.headers.get('X-Offline') === '1'

export type Cover = components['schemas']['Cover']
export type CoverChoice = components['schemas']['CoverChoice']
export type CalendarLink = components['schemas']['CalendarLink']
export type QueueCounts = components['schemas']['QueueCounts']
export type Change = components['schemas']['Change']
export type SetupCode = components['schemas']['SetupCodeOutputBody']
export type NotOwned = NonNullable<
  NonNullable<operations['create-import']['parameters']['query']>['notOwned']
>
export type BookGaps = components['schemas']['BookGaps']
export type Reading = components['schemas']['Reading']
export type FinishedRead = components['schemas']['FinishedRead']
export type ReadingStats = components['schemas']['ReadingStats']
export type Series = components['schemas']['Series']
export type OpenCopy = components['schemas']['OpenCopy']
export type DocumentFill = components['schemas']['DocumentFill']
export type DocProposal = components['schemas']['DocProposal']
export type DocumentSuggestions = components['schemas']['DocumentSuggestions']
export type GoneInput = components['schemas']['GoneInput']
export type ErrorModel = components['schemas']['ErrorModel']
export type Book = components['schemas']['Book']
export type Suggestions = components['schemas']['Suggestions']
export type SuggestedBook = components['schemas']['SuggestedBook']
export type ReadStatusInput = components['schemas']['ReadStatusInput']
export type ReadStatus = ReadStatusInput['readStatus']
export type BookRow = components['schemas']['BookRow']
export type BookInput = components['schemas']['BookInput']
export type CopyInput = components['schemas']['CopyInput']
export type Copy = components['schemas']['Copy']
export type Contributor = components['schemas']['Contributor']
export type Proposal = components['schemas']['Proposal']
export type Candidate = components['schemas']['Candidate']
export type Choices = components['schemas']['Choices']
export type ImportReport = components['schemas']['ImportReport']
export type Passkey = components['schemas']['Passkey']
export type Scan = components['schemas']['Scan']
export type ShareLink = components['schemas']['ShareLink']
export type ShareScope = components['schemas']['Scope']
export type Sees = components['schemas']['Sees']
export type Me = components['schemas']['MeOutputBody']
export type Role = Me['role']
export type Loan = components['schemas']['Loan']
export type LoanInput = components['schemas']['LoanInput']
export type Borrower = components['schemas']['Borrower']
export type BorrowerInput = components['schemas']['BorrowerInput']
export type Course = components['schemas']['Course']
export type CourseInput = components['schemas']['CourseInput']
export type CourseDetail = components['schemas']['CourseDetail']
export type CourseBook = components['schemas']['CourseBook']
export type CourseUse = components['schemas']['CourseUse']
export type QueuedLabel = components['schemas']['QueuedLabel']
export type LabelSheet = components['schemas']['LabelSheet']
export type ShelvingRule = components['schemas']['ShelvingRule']
export type Document = components['schemas']['Document']
export type DocumentInput = components['schemas']['DocumentInput']
export type DocumentRow = components['schemas']['DocumentRow']
export type DocumentFile = components['schemas']['DocumentFile']
export type Bookcase = components['schemas']['Bookcase']
export type Shelf = components['schemas']['Shelf']
export type ShelfInput = components['schemas']['ShelfInput']
export type ShelfPlan = components['schemas']['ShelfPlan']
export type ShelfBook = components['schemas']['ShelfBook']
export type Layout = components['schemas']['Layout']
export type ShelfPlace = components['schemas']['ShelfPlace']

/** The address of one of a document's files, which the browser shows or downloads. */
export const fileURL = (documentId: number, fileId: number) =>
  `/api/v1/documents/${documentId}/files/${fileId}`

/** The address of a file the server makes: a PDF to print from the browser's viewer, an export to save. */
export function apiURL(path: string, copies?: number[]): string {
  const query = copies?.length ? `?copies=${copies.join(',')}` : ''
  return `/api/v1${path}${query}`
}

/** A multipart body for an upload operation: openapi-fetch sends the FormData as it is. */
export function formData(
  file: Blob,
  filename = file instanceof File ? file.name : 'cover.jpg',
): { body: never; bodySerializer: () => FormData } {
  const form = new FormData()
  form.append('file', file, filename)
  return { body: {} as never, bodySerializer: () => form }
}
