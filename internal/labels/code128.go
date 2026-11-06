// Package labels draws the library's label sheets as PDF: Code 128 barcode labels on Avery
// 5167 and spine labels on Avery 5195, for the Brother HL-L2460DW's manual feed slot.
package labels

import (
	"errors"
	"strings"
)

// code128 holds each symbol's bar and space widths in modules, bar first: values 0 to 102,
// then Start A, Start B, Start C, and Stop.
var code128 = [...]string{
	"212222", "222122", "222221", "121223", "121322", "131222", "122213", "122312", "132212", "221213",
	"221312", "231212", "112232", "122132", "122231", "113222", "123122", "123221", "223211", "221132",
	"221231", "213212", "223112", "312131", "311222", "321122", "321221", "312212", "322112", "322211",
	"212123", "212321", "232121", "111323", "131123", "131321", "112313", "132113", "132311", "211313",
	"231113", "231311", "112133", "112331", "132131", "113123", "113321", "133121", "313121", "211331",
	"231131", "213113", "213311", "213131", "311123", "311321", "331121", "312113", "312311", "332111",
	"314111", "221411", "431111", "111224", "111422", "121124", "121421", "141122", "141221", "112214",
	"112412", "122114", "122411", "142112", "142211", "241211", "221114", "413111", "241112", "134111",
	"111242", "121142", "121241", "114212", "124112", "124211", "411212", "421112", "421211", "212141",
	"214121", "412121", "111143", "111341", "131141", "114113", "114311", "411113", "411311", "113141",
	"114131", "311141", "411131", "211412", "211214", "211232", "2331112",
}

const (
	startB = 104
	stop   = 106
	// quiet is the clear space a reader needs on each side, in modules.
	quiet = 10
)

// Code128 encodes text in code set B, which carries every printable ASCII character, and
// returns the widths of its bars and spaces in modules, bar first, with the check symbol and
// the stop. It does not include the quiet zones.
func Code128(text string) ([]int, error) {
	if text == "" {
		return nil, errors.New("code 128: nothing to encode")
	}
	symbols := []int{startB}
	sum := startB
	for i, r := range text {
		if r < 32 || r > 126 {
			return nil, errors.New("code 128: " + string(r) + " is not printable ASCII")
		}
		v := int(r - 32)
		symbols = append(symbols, v)
		sum += v * (i + 1)
	}
	symbols = append(symbols, sum%103, stop)
	var widths []int
	for _, s := range symbols {
		for _, w := range code128[s] {
			widths = append(widths, int(w-'0'))
		}
	}
	return widths, nil
}

// modules is the number of modules widths take up.
func modules(widths []int) int {
	n := 0
	for _, w := range widths {
		n += w
	}
	return n
}

// printable keeps what Code 128 set B can carry.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r > 126 {
			return -1
		}
		return r
	}, s)
}
