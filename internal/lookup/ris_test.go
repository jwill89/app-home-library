package lookup

import (
	"slices"
	"testing"
)

func TestParseRIS(t *testing.T) {
	// As EBSCO exports a record: CRLF lines, a byte order mark, the abstract run onto a second
	// line, subject terms with their first word in capitals, and a date with slashes.
	body := []byte("\xef\xbb\xbfTY  - JOUR\r\nAU  - Ryan, Anthony\r\nAU  - Doe, Jane Q.\r\nT1  - On Theory Choice.\r\n" +
		"JO  - Philosophy of Science\r\nY1  - 2019/03/01/\r\nVL  - 86\r\nIS  - 2\r\nSP  - 201\r\nEP  - 223\r\n" +
		"PB  - Cambridge University Press\r\nDO  - https://doi.org/10.1086/701234\r\nKW  - SCIENTIFIC revolutions\r\n" +
		"KW  - PHILOSOPHY\r\nKW  - Kuhn, Thomas S.\r\nAB  - How scientists choose\r\nbetween theories.\r\n" +
		"UR  - https://research.ebsco.com/linkprocessor/plink?id=abc\r\nER  - \r\nTY  - BOOK\r\nT1  - Another\r\nER  - \r\n")
	p := ParseRIS(body)
	if p == nil {
		t.Fatal("no record")
	}
	if p.Kind != "article" || p.Title != "On Theory Choice" || p.Container != "Philosophy of Science" ||
		p.Published != "2019-03-01" || p.Volume != "86" || p.Issue != "2" || p.Pages != "201-223" ||
		p.DOI != "10.1086/701234" || p.Abstract != "How scientists choose between theories." {
		t.Errorf("proposal %+v", p)
	}
	if !slices.Equal(p.Authors, []string{"Anthony Ryan", "Jane Q. Doe"}) {
		t.Errorf("authors %v", p.Authors)
	}
	if !slices.Equal(p.Subjects, []string{"Scientific revolutions", "Philosophy", "Kuhn, Thomas S"}) {
		t.Errorf("subjects %v", p.Subjects)
	}
	if ParseRIS([]byte("not a citation\n")) != nil {
		t.Error("a file without a record gave one")
	}
	if c := ParseRIS([]byte("TY  - CHAP\nTI  - A Chapter\nT2  - The Book\nA2  - Price, Huw\nED  - Corry, Richard\nPY  - 2004\nER  -\n")); c == nil || c.Kind != "chapter" ||
		c.Container != "The Book" || c.Published != "2004" || !slices.Equal(c.Editors, []string{"Huw Price", "Richard Corry"}) {
		t.Errorf("chapter %+v", c)
	}
	if a := ParseRIS([]byte("TY  - JOUR\nTI  - An Article\nA2  - Itor, Ed\nER  -\n")); a == nil || len(a.Editors) != 0 {
		t.Errorf("article editors %+v", a)
	}
}

func TestParseOpenCopy(t *testing.T) {
	c, err := ParseOpenCopy([]byte(`{"best_oa_location":{"pdf_url":"https://arxiv.org/pdf/1234.pdf","landing_page_url":"https://arxiv.org/abs/1234","license":"cc-by","version":"acceptedVersion","source":{"display_name":"arXiv"}}}`))
	if err != nil || c == nil || c.PDFURL == "" || c.License != "cc-by" || c.Version != "acceptedVersion" || c.Host != "arXiv" {
		t.Errorf("copy %+v, err %v", c, err)
	}
	if c, _ := ParseOpenCopy([]byte(`{"best_oa_location":null}`)); c != nil {
		t.Errorf("no open copy gave %+v", c)
	}
	if c, _ := ParseOpenCopy([]byte(`{"best_oa_location":{"pdf_url":null,"landing_page_url":"https://example.org/a","license":null,"version":null,"source":null}}`)); c == nil || c.PDFURL != "" || c.PageURL == "" {
		t.Errorf("a page without its PDF: %+v", c)
	}
}
