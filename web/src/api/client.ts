import createClient from 'openapi-fetch'
import type { paths } from './schema'

// The one API client, typed by the generated schema's paths; the names the app gives the API's
// types are in types/index.ts. A list in a query is sent as huma reads one: comma-separated in a
// single parameter.
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
