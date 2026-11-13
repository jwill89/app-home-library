package catalog

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestEstimateDewey(t *testing.T) {
	for _, c := range []struct {
		lcc, want string // want "" when there is no estimate
	}{
		{"QA76.73.G63 D66 2016", "004"},
		{"PS3622.A4258", "810"},
		{"PZ7.M6843377 Hi 2020", "813"},
		{"PN6728.G28 W38 2006", "741.5"},
		{"BF441 .K238 2013", "150"},
		{"  hq 1075 .s5", "305.3"},
		{"", ""},
		{"Unknown", ""},
		{"123.4", ""},

		// Ranges inside a subclass.
		{"QA9.A5", "510"},
		{"QA77", "510"},
		{"QA300", "515"},
		{"PN1995", "791.43"},
		{"PN1993.5.U6", "791.43"},
		{"PL876", "895.6"},
		{"PL2658.E1", "895.1"},
		{"PL989", "895.7"},
		{"DS835 .S6", "952"},
		{"DS1", "950"},
		{"PQ2605", "840"},
		{"PQ4302", "850"},
		{"PQ6352", "860"},
		{"PT2621.A26", "830"},
		{"PT5881", "839.3"},
		{"B2430.S34", "190"},
		{"E468", "973.7"},
		{"E185.97", "973"},
		{"Z6", "002"},
		{"Z665", "020"},
		{"Z1035", "016"},
		{"GV1446", "794.1"},
		{"GV1200", "793"},
		{"TK7885", "621.39"},
		{"TK7871", "621.381"},
		{"HQ767.8", "305.23"},
		{"HQ767", "306.8"},

		// The same class however it is written.
		{"qa 76.9 .d3", "004"},
		{"ps3622.a4258", "810"},
		{"PN 6790.J3", "741.5"},
		{"PS3545I345", "810"},

		// Classes of one, two and three letters, and a subclass the table lacks.
		{"Q175 .K8", "500"},
		{"CT275", "920"},
		{"JZ1305", "327"},
		{"DAW1031", "943"},
		{"KJV4", "349.44"},
		{"KJW123", "349.4"},
		{"QA", "510"},

		// Not a class.
		{"FIC", ""},
		{"MANGA", ""},
		{"ISBN 9780756404741", ""},
		{"I1", ""},
	} {
		got, ok := estimateDewey(c.lcc)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("estimateDewey(%q) = %q, %v; want %q", c.lcc, got, ok, c.want)
		}
	}
}

func TestEstimateDeweyTable(t *testing.T) {
	stored := regexp.MustCompile(`^\d{3}(\.\d+)?$`)
	whole := map[string]bool{}
	for _, r := range lcToDewey {
		if !stored.MatchString(r.ddc) {
			t.Errorf("%s: %q is not a stored Dewey number", r.class, r.ddc)
		}
		if r.from == 0 && r.to == 0 {
			if whole[r.class] {
				t.Errorf("%s: a second entry for the whole subclass", r.class)
			}
			whole[r.class] = true
			continue
		}
		if whole[r.class] {
			t.Errorf("%s %v–%v: after the whole subclass, so never reached", r.class, r.from, r.to)
		}
		if r.from <= 0 || r.to != math.Trunc(r.to) || (r.to != 0 && r.to < r.from) {
			t.Errorf("%s %v–%v: not a range", r.class, r.from, r.to)
		}
		// Each end of a range gives its own number, so no entry before it hides it.
		for _, n := range []float64{r.from, r.to} {
			if n == 0 {
				continue
			}
			lcc := r.class + strconv.FormatFloat(n, 'f', -1, 64)
			if got, _ := estimateDewey(lcc); got != r.ddc {
				t.Errorf("%s gives %q, want %q", lcc, got, r.ddc)
			}
		}
	}
	for _, r := range lcToDewey {
		if !whole[r.class] {
			t.Errorf("%s: no entry for the whole subclass", r.class)
		}
	}
}

func TestEstimateDeweyClasses(t *testing.T) {
	// LC uses 21 letters; I, O, W, X and Y are no class.
	for _, l := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		_, ok := estimateDewey(string(l) + "1")
		if want := !strings.ContainsRune("IOWXY", l); ok != want {
			t.Errorf("class %c: ok %v, want %v", l, ok, want)
		}
	}
}
