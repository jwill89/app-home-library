package lookup

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
	"unicode"
)

// risLineRe is one line of a RIS file: a two-character tag, then "  - ", then the value.
var risLineRe = regexp.MustCompile(`^([A-Z][A-Z0-9])\s{1,2}-\s?(.*)$`)

// ParseRIS reads the first record of a RIS file, the format library databases (EBSCO's among
// them) export a citation in, into a document's details. nil when the file holds no record.
func ParseRIS(body []byte) *DocProposal {
	body = bytes.TrimPrefix(body, []byte{0xef, 0xbb, 0xbf}) // a byte order mark, as Windows writes one
	fields := map[string][]string{}
	var last string
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r ")
		m := risLineRe.FindStringSubmatch(line)
		switch {
		case m != nil && m[1] == "ER":
			if len(fields) > 0 {
				return risProposal(fields)
			}
		case m != nil:
			last = m[1]
			fields[last] = append(fields[last], strings.TrimSpace(m[2]))
		case last != "" && strings.TrimSpace(line) != "":
			// A value carried onto the next line: an abstract, as a rule.
			v := fields[last]
			v[len(v)-1] += " " + strings.TrimSpace(line)
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return risProposal(fields)
}

func risProposal(f map[string][]string) *DocProposal {
	first := func(tags ...string) string {
		for _, t := range tags {
			for _, v := range f[t] {
				if v = strings.TrimSpace(v); v != "" {
					return v
				}
			}
		}
		return ""
	}
	p := &DocProposal{
		Kind:      risKind(first("TY")),
		Title:     cleanField(first("TI", "T1")),
		Container: cleanField(first("T2", "JF", "JO", "JA", "BT")),
		Volume:    first("VL"),
		Issue:     first("IS"),
		Published: NormalizeDate(strings.TrimRight(strings.ReplaceAll(first("PY", "Y1", "DA"), "/", "-"), "-")),
		Publisher: cleanField(first("PB")),
		URL:       first("UR"),
		Abstract:  first("AB", "N2"),
		Authors:   []string{},
		Editors:   []string{},
		Subjects:  []string{},
	}
	if p.Title == "" && p.Kind == "" {
		return nil
	}
	if p.Kind == "" {
		p.Kind = "other"
	}
	if sp, ep := first("SP"), first("EP"); sp != "" && ep != "" && ep != sp {
		p.Pages = sp + "-" + ep
	} else {
		p.Pages = sp
	}
	if d, ok := NormalizeDOI(first("DO")); ok {
		p.DOI = d
	}
	for _, t := range []string{"AU", "A1"} {
		for _, a := range f[t] {
			if name := InvertName(a); name != "" {
				p.Authors = appendUnique(p.Authors, name)
			}
		}
	}
	// A chapter's A2 (or ED) is its book's editors; an article's A2 is not cited.
	if p.Kind == "chapter" {
		for _, t := range []string{"A2", "ED"} {
			for _, a := range f[t] {
				if name := InvertName(a); name != "" {
					p.Editors = appendUnique(p.Editors, name)
				}
			}
		}
	}
	for _, k := range f["KW"] {
		for _, s := range strings.Split(k, ";") {
			if s = subjectCase(cleanField(s)); s != "" {
				p.Subjects = appendUnique(p.Subjects, s)
			}
		}
	}
	return p
}

// risKind reads a RIS type as a document's kind.
func risKind(ty string) string {
	switch strings.ToUpper(strings.TrimSpace(ty)) {
	case "":
		return ""
	case "JOUR", "JFULL", "EJOUR", "MGZN", "NEWS", "ABST", "INPR":
		return "article"
	case "CHAP", "ECHAP":
		return "chapter"
	}
	return "other"
}

// subjectCase writes a database's subject term as a heading: EBSCO capitalizes its first word
// ("SCIENTIFIC revolutions", "PHILOSOPHY"), which reads as shouting beside the rest.
func subjectCase(s string) string {
	word, rest, _ := strings.Cut(s, " ")
	letters := strings.IndexFunc(word, unicode.IsLetter) >= 0
	if !letters || strings.ToUpper(word) != word || len([]rune(word)) < 2 {
		return s
	}
	r := []rune(strings.ToLower(word))
	r[0] = unicode.ToUpper(r[0])
	if rest == "" {
		return string(r)
	}
	return string(r) + " " + rest
}
