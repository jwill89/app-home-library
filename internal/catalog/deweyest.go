package catalog

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// lcClassRe reads the class off an LC call number: its letters, and the number after them when
// there is one. Letters running on past three are a word (UNKNOWN, MANGA), not a class.
var lcClassRe = regexp.MustCompile(`^([A-Z]{1,3})(?:\s*(\d+(?:\.\d+)?)|[^A-Z]|$)`)

// lcRange is the Dewey number for an LC subclass, or for the numbers from `from` through `to` within it. A
// range takes in the decimals of its last number, as the schedules print ranges (PL501–889
// holds PL889.5), and a to of 0 runs to the end. A from and to of 0 is the whole subclass,
// which follows its ranges so that the narrowest entry is found first.
type lcRange struct {
	class    string
	from, to float64
	ddc      string
}

func (r lcRange) covers(n float64) bool {
	return n >= r.from && (r.to == 0 || math.Floor(n) <= r.to)
}

// lcToDewey is an LC-to-Dewey crosswalk approximation, good for shelf order only.
var lcToDewey = []lcRange{
	{"A", 0, 0, "000"},
	{"AC", 0, 0, "080"},
	{"AE", 0, 0, "030"},
	{"AG", 0, 0, "030"},
	{"AI", 0, 0, "050"},
	{"AM", 0, 0, "069"},
	{"AN", 0, 0, "070"},
	{"AP", 0, 0, "050"},
	{"AS", 0, 0, "060"},
	{"AY", 0, 0, "030"},
	{"AZ", 0, 0, "001.2"},

	{"B", 108, 708, "180"},  // ancient philosophy
	{"B", 720, 765, "189"},  // medieval
	{"B", 790, 5739, "190"}, // modern
	{"B", 0, 0, "100"},
	{"BC", 0, 0, "160"},
	{"BD", 143, 237, "121"}, // epistemology
	{"BD", 493, 708, "113"}, // cosmology
	{"BD", 0, 0, "110"},
	{"BF", 1001, 2055, "133"}, // parapsychology, the occult
	{"BF", 0, 0, "150"},
	{"BH", 0, 0, "111.85"},
	{"BJ", 1801, 2195, "395"}, // etiquette
	{"BJ", 0, 0, "170"},
	{"BL", 300, 325, "201.3"},   // mythology
	{"BL", 700, 820, "292"},     // classical religion
	{"BL", 1100, 1295, "294.5"}, // Hinduism
	{"BL", 0, 0, "200"},
	{"BM", 0, 0, "296"},
	{"BP", 0, 0, "297"},
	{"BQ", 0, 0, "294.3"},
	{"BR", 0, 0, "270"},
	{"BS", 0, 0, "220"},
	{"BT", 0, 0, "230"},
	{"BV", 0, 0, "248"},
	{"BX", 0, 0, "280"},

	{"C", 0, 0, "900"},
	{"CB", 0, 0, "909"},
	{"CC", 0, 0, "930.1"},
	{"CD", 0, 0, "027"},
	{"CE", 0, 0, "529"},
	{"CJ", 0, 0, "737"},
	{"CN", 0, 0, "411.7"},
	{"CR", 0, 0, "929.6"},
	{"CS", 0, 0, "929.1"},
	{"CT", 0, 0, "920"},

	{"D", 1, 50, "909"},       // world history
	{"D", 51, 90, "930"},      // ancient
	{"D", 111, 203, "909.07"}, // medieval
	{"D", 204, 475, "909.08"}, // modern
	{"D", 501, 680, "940.3"},  // the First World War
	{"D", 731, 838, "940.53"}, // the Second World War
	{"D", 0, 0, "940"},
	{"DA", 20, 690, "942"},    // England
	{"DA", 700, 745, "942.9"}, // Wales
	{"DA", 750, 890, "941.1"}, // Scotland
	{"DA", 900, 995, "941.5"}, // Ireland
	{"DA", 0, 0, "941"},
	{"DAW", 0, 0, "943"},
	{"DB", 901, 999, "943.9"},   // Hungary
	{"DB", 2000, 3150, "943.7"}, // Czechoslovakia
	{"DB", 0, 0, "943.6"},
	{"DC", 0, 0, "944"},
	{"DD", 0, 0, "943"},
	{"DE", 0, 0, "938"},
	{"DF", 501, 951, "949.5"}, // medieval and modern Greece
	{"DF", 0, 0, "938"},
	{"DG", 11, 365, "937"}, // ancient Rome
	{"DG", 0, 0, "945"},
	{"DH", 901, 925, "949.35"}, // Luxembourg
	{"DH", 0, 0, "949.3"},
	{"DJ", 0, 0, "949.2"},
	{"DJK", 0, 0, "947"},
	{"DK", 4010, 4800, "943.8"}, // Poland
	{"DK", 0, 0, "947"},
	{"DL", 101, 291, "948.9"},    // Denmark
	{"DL", 301, 398, "949.12"},   // Iceland
	{"DL", 401, 596, "948.1"},    // Norway
	{"DL", 601, 991, "948.5"},    // Sweden
	{"DL", 1002, 1180, "948.97"}, // Finland
	{"DL", 0, 0, "948"},
	{"DP", 501, 900, "946.9"}, // Portugal
	{"DP", 0, 0, "946"},
	{"DQ", 0, 0, "949.4"},
	{"DR", 401, 741, "956.1"}, // Turkey
	{"DR", 0, 0, "949.6"},
	{"DS", 350, 375, "958.1"},  // Afghanistan
	{"DS", 376, 392, "954.91"}, // Pakistan
	{"DS", 393, 396, "954.92"}, // Bangladesh
	{"DS", 401, 486, "954"},    // India
	{"DS", 488, 490, "954.93"}, // Sri Lanka
	{"DS", 520, 560, "959"},    // Southeast Asia
	{"DS", 701, 799, "951"},    // China
	{"DS", 801, 897, "952"},    // Japan
	{"DS", 901, 937, "951.9"},  // Korea
	{"DS", 0, 0, "950"},
	{"DT", 43, 154, "962"},    // Egypt
	{"DT", 1701, 2405, "968"}, // South Africa
	{"DT", 0, 0, "960"},
	{"DU", 80, 398, "994"},  // Australia
	{"DU", 400, 430, "993"}, // New Zealand
	{"DU", 0, 0, "990"},
	{"DX", 0, 0, "305.891497"},

	{"E", 51, 99, "970.00497"}, // American Indians
	{"E", 201, 298, "973.3"},   // the Revolution
	{"E", 456, 655, "973.7"},   // the Civil War
	{"E", 151, 909, "973"},     // the United States
	{"E", 0, 0, "970"},

	{"F", 1, 975, "973"},       // the states
	{"F", 1001, 1145, "971"},   // Canada
	{"F", 1201, 1392, "972"},   // Mexico
	{"F", 1401, 1419, "980"},   // Latin America
	{"F", 1421, 1577, "972.8"}, // Central America
	{"F", 1601, 2183, "972.9"}, // the West Indies
	{"F", 2201, 3799, "980"},   // South America
	{"F", 0, 0, "970"},

	{"G", 1000, 9980, "912"}, // atlases and maps
	{"G", 0, 0, "910"},
	{"GA", 0, 0, "526"},
	{"GB", 0, 0, "910.02"},
	{"GC", 0, 0, "551.46"},
	{"GE", 0, 0, "363.7"},
	{"GF", 0, 0, "304.2"},
	{"GN", 49, 298, "599.9"},  // physical anthropology
	{"GN", 700, 890, "930.1"}, // prehistoric archaeology
	{"GN", 0, 0, "301"},
	{"GR", 0, 0, "398"},
	{"GT", 0, 0, "390"},
	{"GV", 201, 555, "613.7"},   // physical education
	{"GV", 557, 1198, "796"},    // sports
	{"GV", 1313, 1458, "794.1"}, // chess, inside games
	{"GV", 1199, 1570, "793"},   // games
	{"GV", 1580, 1799, "792.8"}, // dance
	{"GV", 0, 0, "790"},

	{"H", 0, 0, "300"},
	{"HA", 0, 0, "310"},
	{"HB", 848, 3697, "304.6"}, // demography
	{"HB", 0, 0, "330"},
	{"HC", 0, 0, "330"},
	{"HD", 28, 70, "658"},     // management
	{"HD", 1241, 1339, "333"}, // land
	{"HD", 4801, 8943, "331"}, // labour
	{"HD", 0, 0, "338"},
	{"HE", 0, 0, "380"},
	{"HF", 1, 4050, "382"},      // trade
	{"HF", 5601, 5689, "657"},   // accounting
	{"HF", 5801, 6182, "659.1"}, // advertising
	{"HF", 0, 0, "650"},
	{"HG", 8011, 9999, "368"}, // insurance
	{"HG", 0, 0, "332"},
	{"HJ", 0, 0, "336"},
	{"HM", 0, 0, "301"},
	{"HN", 0, 0, "306"},
	{"HQ", 12, 449, "306.7"},     // sexuality
	{"HQ", 767.8, 792, "305.23"}, // children
	{"HQ", 1060, 1064, "305.26"}, // old age
	{"HQ", 1075, 1090, "305.3"},  // gender, men
	{"HQ", 1101, 2030, "305.4"},  // women
	{"HQ", 0, 0, "306.8"},
	{"HS", 0, 0, "366"},
	{"HT", 101, 395, "307.76"},  // cities
	{"HT", 601, 1445, "305.5"},  // social classes
	{"HT", 1501, 1595, "305.8"}, // races
	{"HT", 0, 0, "307"},
	{"HV", 5001, 5840, "362.29"}, // alcohol, tobacco and drugs
	{"HV", 6001, 7220, "364"},    // crime
	{"HV", 7551, 8280, "363.2"},  // police
	{"HV", 8301, 9920, "365"},    // prisons
	{"HV", 0, 0, "361"},
	{"HX", 0, 0, "335"},

	{"J", 0, 0, "320"},
	{"JA", 0, 0, "320"},
	{"JC", 0, 0, "320.1"},
	{"JF", 0, 0, "320.3"},
	{"JK", 0, 0, "320.473"},
	{"JL", 0, 0, "320.97"},
	{"JN", 0, 0, "320.94"},
	{"JQ", 0, 0, "320.95"},
	{"JS", 0, 0, "320.8"},
	{"JV", 0, 0, "325"},
	{"JX", 0, 0, "341"},
	{"JZ", 0, 0, "327"},

	{"K", 0, 0, "340"},
	{"KBM", 0, 0, "296.18"},
	{"KBP", 0, 0, "340.59"},
	{"KBR", 0, 0, "262.9"},
	{"KBU", 0, 0, "262.9"},
	{"KD", 0, 0, "349.41"},
	{"KDZ", 0, 0, "349.7"},
	{"KE", 0, 0, "349.71"},
	{"KF", 0, 0, "349.73"},
	{"KJ", 0, 0, "349.4"},
	{"KJV", 0, 0, "349.44"},
	{"KK", 0, 0, "349.43"},
	{"KZ", 0, 0, "341"},

	{"L", 0, 0, "370"},
	{"LA", 0, 0, "370.9"},
	{"LB", 1025, 1050, "371.102"}, // teaching
	{"LB", 1051, 1091, "370.15"},  // educational psychology
	{"LB", 2300, 2430, "378"},     // higher education
	{"LB", 0, 0, "370.1"},
	{"LC", 3950, 4806, "371.9"}, // special education
	{"LC", 0, 0, "370"},
	{"LD", 0, 0, "378.73"},
	{"LE", 0, 0, "378.7"},
	{"LF", 0, 0, "378.4"},
	{"LG", 0, 0, "378.5"},
	{"LH", 0, 0, "378.1"},
	{"LJ", 0, 0, "371.85"},
	{"LT", 0, 0, "371.32"},

	{"M", 0, 0, "780"},
	{"ML", 3469, 3541, "781.64"}, // popular music
	{"ML", 0, 0, "780.9"},
	{"MT", 0, 0, "780.7"},

	{"N", 0, 0, "700"},
	{"NA", 9000, 9428, "711"}, // city planning
	{"NA", 0, 0, "720"},
	{"NB", 0, 0, "730"},
	{"NC", 1300, 1766, "741.5"}, // cartoons and comic art
	{"NC", 0, 0, "741"},
	{"ND", 0, 0, "750"},
	{"NE", 0, 0, "760"},
	{"NK", 1700, 3505, "747"}, // interior decoration
	{"NK", 0, 0, "745"},
	{"NX", 0, 0, "700"},

	{"P", 87, 96, "302.23"}, // communication, mass media
	{"P", 0, 0, "410"},
	{"PA", 201, 1179, "480"},  // the Greek language
	{"PA", 2001, 2995, "470"}, // the Latin language
	{"PA", 6000, 6971, "870"}, // Latin literature
	{"PA", 0, 0, "880"},
	{"PB", 0, 0, "491.6"},
	{"PC", 601, 872, "459"},   // Romanian
	{"PC", 1001, 1984, "450"}, // Italian
	{"PC", 4001, 4977, "460"}, // Spanish
	{"PC", 5001, 5498, "469"}, // Portuguese
	{"PC", 0, 0, "440"},
	{"PD", 5001, 5929, "439.5"}, // Scandinavian
	{"PD", 0, 0, "430"},
	{"PE", 0, 0, "420"},
	{"PF", 1, 979, "439.31"}, // Dutch
	{"PF", 0, 0, "430"},
	{"PG", 2001, 2850, "491.7"},  // the Russian language
	{"PG", 7001, 7446, "891.85"}, // Polish literature
	{"PG", 0, 0, "891.7"},
	{"PH", 0, 0, "894.5"},
	{"PJ", 5001, 5192, "892.4"}, // Hebrew literature
	{"PJ", 7501, 8517, "892.7"}, // Arabic literature
	{"PJ", 0, 0, "892"},
	{"PK", 6001, 6996, "891.5"}, // Persian
	{"PK", 0, 0, "891"},
	{"PL", 501, 889, "895.6"},   // Japanese
	{"PL", 901, 998, "895.7"},   // Korean
	{"PL", 1001, 3208, "895.1"}, // Chinese
	{"PL", 4601, 4961, "894.8"}, // Dravidian
	{"PL", 5001, 7511, "899.2"}, // Austronesian
	{"PL", 8000, 8844, "896"},   // African
	{"PL", 0, 0, "895"},
	{"PM", 8001, 9021, "499.99"}, // artificial languages
	{"PM", 0, 0, "497"},
	{"PN", 101, 249, "808.02"},   // authorship
	{"PN", 1991, 1991, "791.44"}, // radio
	{"PN", 1992, 1992, "791.45"}, // television
	{"PN", 1993, 1999, "791.43"}, // film
	{"PN", 2000, 3307, "792"},    // theatre
	{"PN", 4699, 5650, "070"},    // journalism
	{"PN", 6010, 6146, "808.8"},  // collections
	{"PN", 6147, 6231, "808.87"}, // humour
	{"PN", 6700, 6790, "741.5"},  // comics
	{"PN", 0, 0, "809"},
	{"PQ", 4001, 5999, "850"}, // Italian
	{"PQ", 6001, 8929, "860"}, // Spanish and Spanish American
	{"PQ", 9000, 9999, "869"}, // Portuguese and Brazilian
	{"PQ", 0, 0, "840"},
	{"PR", 0, 0, "820"},
	{"PS", 0, 0, "810"},
	{"PT", 5001, 6593, "839.3"},  // Dutch, Flemish, Afrikaans
	{"PT", 7001, 7099, "839.5"},  // Scandinavian
	{"PT", 7101, 7550, "839.6"},  // Old Norse, Icelandic
	{"PT", 7601, 8260, "839.81"}, // Danish
	{"PT", 8301, 9155, "839.82"}, // Norwegian
	{"PT", 9201, 9999, "839.7"},  // Swedish
	{"PT", 0, 0, "830"},
	{"PZ", 0, 0, "813"}, // juvenile fiction, shelved as fiction

	{"Q", 334, 342, "006.3"}, // artificial intelligence, inside cybernetics
	{"Q", 300, 390, "003.5"}, // cybernetics
	{"Q", 0, 0, "500"},
	{"QA", 75, 76, "004"},     // computing
	{"QA", 150, 272, "512"},   // algebra
	{"QA", 273, 274, "519.2"}, // probability
	{"QA", 276, 280, "519.5"}, // statistics
	{"QA", 299, 433, "515"},   // analysis
	{"QA", 440, 699, "516"},   // geometry
	{"QA", 801, 939, "531"},   // mechanics
	{"QA", 0, 0, "510"},
	{"QB", 0, 0, "520"},
	{"QC", 851, 999, "551.5"}, // meteorology
	{"QC", 0, 0, "530"},
	{"QD", 0, 0, "540"},
	{"QE", 701, 996, "560"}, // paleontology
	{"QE", 0, 0, "550"},
	{"QH", 1, 278, "508"},     // natural history
	{"QH", 426, 470, "576.5"}, // genetics
	{"QH", 540, 549, "577"},   // ecology
	{"QH", 0, 0, "570"},
	{"QK", 0, 0, "580"},
	{"QL", 461, 599, "595.7"}, // insects
	{"QL", 671, 699, "598"},   // birds
	{"QL", 700, 739, "599"},   // mammals
	{"QL", 0, 0, "590"},
	{"QM", 0, 0, "611"},
	{"QP", 351, 495, "612.8"}, // the nervous system, the senses
	{"QP", 0, 0, "612"},
	{"QR", 0, 0, "579"},

	{"R", 0, 0, "610"},
	{"RA", 421, 772, "614"},     // public health
	{"RA", 773, 790, "613"},     // personal health
	{"RA", 1001, 1171, "614.1"}, // forensic medicine
	{"RA", 1190, 1270, "615.9"}, // toxicology
	{"RA", 0, 0, "362.1"},
	{"RB", 0, 0, "616.07"},
	{"RC", 435, 571, "616.89"}, // psychiatry
	{"RC", 0, 0, "616"},
	{"RD", 0, 0, "617"},
	{"RE", 0, 0, "617.7"},
	{"RF", 0, 0, "617.51"},
	{"RG", 0, 0, "618"},
	{"RJ", 0, 0, "618.92"},
	{"RK", 0, 0, "617.6"},
	{"RL", 0, 0, "616.5"},
	{"RM", 0, 0, "615"},
	{"RS", 0, 0, "615.1"},
	{"RT", 0, 0, "610.73"},
	{"RV", 0, 0, "615.32"},
	{"RX", 0, 0, "615.532"},
	{"RZ", 0, 0, "615.5"},

	{"S", 0, 0, "630"},
	{"SB", 469, 476, "712"}, // landscape gardening
	{"SB", 0, 0, "635"},
	{"SD", 0, 0, "634.9"},
	{"SF", 600, 1100, "636.089"}, // veterinary medicine
	{"SF", 0, 0, "636"},
	{"SH", 401, 691, "799.1"}, // angling
	{"SH", 0, 0, "639.2"},
	{"SK", 0, 0, "799.2"},

	{"T", 385, 385, "006.6"}, // computer graphics
	{"T", 0, 0, "600"},
	{"TA", 0, 0, "620"},
	{"TC", 0, 0, "627"},
	{"TD", 0, 0, "628"},
	{"TE", 0, 0, "625.7"},
	{"TF", 0, 0, "625.1"},
	{"TG", 0, 0, "624.2"},
	{"TH", 0, 0, "690"},
	{"TJ", 210, 211, "629.892"}, // robotics
	{"TJ", 0, 0, "621"},
	{"TK", 5101, 6720, "621.382"}, // telecommunication
	{"TK", 7885, 7895, "621.39"},  // computer engineering, inside electronics
	{"TK", 7800, 8360, "621.381"}, // electronics
	{"TK", 9001, 9401, "621.48"},  // nuclear engineering
	{"TK", 0, 0, "621.3"},
	{"TL", 1, 484, "629.2"},    // motor vehicles
	{"TL", 787, 4050, "629.4"}, // astronautics
	{"TL", 0, 0, "629.1"},
	{"TN", 0, 0, "622"},
	{"TP", 500, 660, "663"}, // fermentation, beverages
	{"TP", 0, 0, "660"},
	{"TR", 845, 899, "777"}, // cinematography, video
	{"TR", 0, 0, "770"},
	{"TS", 0, 0, "670"},
	{"TT", 490, 695, "646.4"}, // clothing, sewing
	{"TT", 0, 0, "745.5"},
	{"TX", 642, 840, "641.5"}, // cooking
	{"TX", 0, 0, "641"},

	{"U", 0, 0, "355"},
	{"UA", 0, 0, "355.3"},
	{"UB", 0, 0, "355.6"},
	{"UC", 0, 0, "355.8"},
	{"UD", 0, 0, "356"},
	{"UE", 0, 0, "357"},
	{"UF", 0, 0, "358.1"},
	{"UG", 622, 1435, "358.4"}, // air forces
	{"UG", 0, 0, "623"},
	{"UH", 0, 0, "355.34"},

	{"V", 0, 0, "359"},
	{"VA", 0, 0, "359.3"},
	{"VK", 0, 0, "623.89"},
	{"VM", 0, 0, "623.8"},

	{"Z", 4, 8, "002"},       // the history of books
	{"Z", 116, 265, "686.2"}, // printing
	{"Z", 278, 549, "070.5"}, // publishing
	{"Z", 1001, 0, "016"},    // bibliographies
	{"Z", 0, 0, "020"},
	{"ZA", 0, 0, "025.04"},
}

// estimateDewey is a Dewey number worked out from a Library of Congress call number's class,
// for a book that has none of its own: the subclass's nearest Dewey number, as a library's
// crosswalk gives it. ok is false for a class the table does not know, or no class at all.
func estimateDewey(lcc string) (ddc string, ok bool) {
	m := lcClassRe.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(lcc)))
	if m == nil {
		return "", false
	}
	letters, number := m[1], m[2]
	n := 0.0
	if number != "" {
		var err error
		if n, err = strconv.ParseFloat(number, 64); err != nil {
			return "", false
		}
	}
	// Longest letters first, QA before Q. A subclass the table lacks falls back to its main
	// class only when a number follows, so a bare shelf prefix such as FIC is not read as F.
	for l := len(letters); l > 0 && (l == len(letters) || number != ""); l-- {
		for _, r := range lcToDewey {
			if r.class == letters[:l] && r.covers(n) {
				return r.ddc, true
			}
		}
	}
	return "", false
}
