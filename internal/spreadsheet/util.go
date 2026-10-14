package spreadsheet

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
)

func sortStrings(s []string) { sort.Strings(s) }

// sortSheetIDs orders ids like "sheet2" before "sheet10".
func sortSheetIDs(ids []string) {
	num := func(id string) (int, bool) {
		i := len(id)
		for i > 0 && unicode.IsDigit(rune(id[i-1])) {
			i--
		}
		n, err := strconv.Atoi(id[i:])
		return n, err == nil
	}
	sort.SliceStable(ids, func(a, b int) bool {
		na, oka := num(ids[a])
		nb, okb := num(ids[b])
		switch {
		case oka && okb && na != nb:
			return na < nb
		case oka != okb:
			return oka
		}
		return strings.ToLower(ids[a]) < strings.ToLower(ids[b])
	})
}
