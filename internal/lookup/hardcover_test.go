package lookup

import (
	"slices"
	"testing"
)

func TestParseHardcover(t *testing.T) {
	body := []byte(`{"data":{"editions":[{"image":{"url":"https://assets.hardcover.app/edition/1/a.jpg"},"book":{"image":{"url":"https://assets.hardcover.app/book/9/b.jpg"},"book_series":[{"position":1,"series":{"name":"Raven's Shadow"}}]}}]}}`)
	r, err := ParseHardcover(body)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.Covers, []string{"https://assets.hardcover.app/edition/1/a.jpg", "https://assets.hardcover.app/book/9/b.jpg"}) || r.Series != "Raven's Shadow" || r.SeriesNumber != "1" {
		t.Errorf("record %+v", r)
	}

	// A series named in another script is left out; the cover still comes.
	r, err = ParseHardcover([]byte(`{"data":{"editions":[{"image":null,"book":{"image":{"url":"u"},"book_series":[{"position":7,"series":{"name":"魔導具師ダリヤはうつむかない (Comic)"}}]}}]}}`))
	if err != nil || r.Series != "" || r.SeriesNumber != "" || !slices.Equal(r.Covers, []string{"u"}) {
		t.Errorf("record %+v, err %v", r, err)
	}

	if r, err := ParseHardcover([]byte(`{"data":{"editions":[]}}`)); r != nil || err != nil {
		t.Errorf("an unknown ISBN: %+v, %v", r, err)
	}
	if _, err := ParseHardcover([]byte(`{"errors":[{"message":"invalid token"}]}`)); err == nil {
		t.Error("an error answer was taken as a book")
	}
}

func TestEnglishSeries(t *testing.T) {
	for in, want := range map[string]string{
		"ヲタクに恋は難しい / Wotakoi: Love is Hard for Otaku":                                                    "Wotakoi: Love is Hard for Otaku",
		"漫画 超人高校生たちは異世界でも餘裕で生き抜くようです！ / High School Prodigies Have It Easy Even in Another World! Manga": "High School Prodigies Have It Easy Even in Another World! Manga",
		"Raven's Shadow": "Raven's Shadow",
		"Either / Or":    "Either / Or",
		"漫画 理想のヒモ生活":     "漫画 理想のヒモ生活",
	} {
		if got := englishSeries(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}
