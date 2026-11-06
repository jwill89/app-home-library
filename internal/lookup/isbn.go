package lookup

import "strings"

// NormalizeISBN reads an ISBN-10 or ISBN-13, with or without hyphens and spaces, and returns
// the ISBN-13 when its check digit holds. A scanned EAN-13 that is not Bookland (978/979)
// is refused: a mass-market paperback's UPC is not its ISBN.
func NormalizeISBN(s string) (string, bool) {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		switch {
		case r >= '0' && r <= '9', r == 'X':
			b.WriteRune(r)
		case r == '-' || r == ' ':
		default:
			return "", false
		}
	}
	d := b.String()
	switch len(d) {
	case 10:
		if !valid10(d) {
			return "", false
		}
		return to13(d), true
	case 13:
		if !strings.HasPrefix(d, "978") && !strings.HasPrefix(d, "979") {
			return "", false
		}
		if !valid13(d) {
			return "", false
		}
		return d, true
	}
	return "", false
}

// IsRetailCode reports whether a scanned code is a store's product barcode and not an ISBN:
// a 12-digit UPC-A, an 8-digit UPC-E, or an EAN-13 outside Bookland. A mass-market paperback
// carries a UPC on the back and prints its ISBN inside the front cover.
func IsRetailCode(code string) bool {
	code = strings.TrimSpace(code)
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	switch len(code) {
	case 8, 12:
		return true
	case 13:
		return !strings.HasPrefix(code, "978") && !strings.HasPrefix(code, "979")
	}
	return false
}

// ISBN10 is the ten-digit form of a 978 ISBN-13, and empty for a 979 one, which has none.
func ISBN10(isbn13 string) string {
	if len(isbn13) != 13 || !strings.HasPrefix(isbn13, "978") {
		return ""
	}
	core := isbn13[3:12]
	sum := 0
	for i, r := range core {
		sum += (10 - i) * int(r-'0')
	}
	check := (11 - sum%11) % 11
	if check == 10 {
		return core + "X"
	}
	return core + string(rune('0'+check))
}

func valid10(d string) bool {
	sum := 0
	for i, r := range d {
		v := int(r - '0')
		if r == 'X' {
			if i != 9 {
				return false
			}
			v = 10
		}
		sum += (10 - i) * v
	}
	return sum%11 == 0
}

func valid13(d string) bool {
	if strings.ContainsRune(d, 'X') {
		return false
	}
	return check13(d[:12]) == d[12]
}

func check13(first12 string) byte {
	sum := 0
	for i, r := range first12 {
		w := 1
		if i%2 == 1 {
			w = 3
		}
		sum += w * int(r-'0')
	}
	return byte('0' + (10-sum%10)%10)
}

func to13(isbn10 string) string {
	core := "978" + isbn10[:9]
	return core + string(check13(core))
}
