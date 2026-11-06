/**
 * Copies text to the clipboard, with HTML beside it where the browser takes both, so a word
 * processor keeps the italics. It answers whether the copy was made.
 */
export async function copyText(text: string, html?: string): Promise<boolean> {
  try {
    if (html && typeof ClipboardItem === 'function' && navigator.clipboard.write) {
      await navigator.clipboard.write([
        new ClipboardItem({
          'text/plain': new Blob([text], { type: 'text/plain' }),
          'text/html': new Blob([html], { type: 'text/html' }),
        }),
      ])
    } else {
      await navigator.clipboard.writeText(text)
    }
    return true
  } catch {
    return false
  }
}
