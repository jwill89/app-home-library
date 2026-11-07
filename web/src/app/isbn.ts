/**
 * A typed or scanned code as an ISBN-13, the way the server reads one: an ISBN-10 (with an X
 * for ten) becomes its 978 form, and either must check out. Empty for anything else, such as a
 * store's product code or one of the library's labels. The server is the judge; this is for
 * when it cannot be asked.
 */
export function isbn13(code: string): string {
  const s = code.replace(/[\s-]/g, '').toUpperCase()
  if (/^\d{9}[\dX]$/.test(s)) {
    const sum = Array.from(s).reduce((n, c, i) => n + (c === 'X' ? 10 : Number(c)) * (10 - i), 0)
    if (sum % 11 !== 0) {
      return ''
    }
    const twelve = `978${s.slice(0, 9)}`
    return twelve + checkDigit(twelve)
  }
  if (/^(978|979)\d{10}$/.test(s) && checkDigit(s.slice(0, 12)) === s[12]) {
    return s
  }
  return ''
}

function checkDigit(twelve: string): string {
  const sum = Array.from(twelve).reduce((n, c, i) => n + Number(c) * (i % 2 ? 3 : 1), 0)
  return String((10 - (sum % 10)) % 10)
}
