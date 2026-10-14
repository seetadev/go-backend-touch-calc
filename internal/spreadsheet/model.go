// Package spreadsheet converts between spreadsheet files (xls, xlsx, csv) and
// the SocialCalc multi-sheet save format used by the Tornado backend.
package spreadsheet

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Limits that keep a hostile or accidental huge upload from exhausting memory.
const (
	MaxSheets = 100
	MaxCells  = 1_000_000
	// MaxFileBytes caps the size of an uploaded file that is read into memory.
	MaxFileBytes = 25 << 20
)

// ErrTooLarge is returned when a workbook exceeds MaxSheets or MaxCells.
var ErrTooLarge = errors.New("spreadsheet is too large")

// Coord is a 1-based cell position.
type Coord struct{ Row, Col int }

// Name returns the A1-style name of the coordinate.
func (c Coord) Name() string { return ColName(c.Col) + fmt.Sprint(c.Row) }

// Cell is one non-empty cell.
type Cell struct {
	IsNumber bool
	Num      float64
	Text     string
	// Formula is the formula without the leading "=", or empty.
	Formula string
	// Format is an Excel-style number format code, or empty for General.
	Format string
	// ColSpan and RowSpan are set (>1) on the top-left cell of a merged range.
	ColSpan, RowSpan int
}

// Sheet is a named grid of cells.
type Sheet struct {
	Name  string
	Cells map[Coord]*Cell
}

// Workbook is an ordered list of sheets and the index of the active one.
type Workbook struct {
	Sheets  []*Sheet
	Current int
}

func newSheet(name string) *Sheet { return &Sheet{Name: name, Cells: map[Coord]*Cell{}} }

// sortedCoords returns the sheet's cell coordinates in row-major order.
func (s *Sheet) sortedCoords() []Coord {
	coords := make([]Coord, 0, len(s.Cells))
	for c := range s.Cells {
		coords = append(coords, c)
	}
	sort.Slice(coords, func(i, j int) bool {
		if coords[i].Row != coords[j].Row {
			return coords[i].Row < coords[j].Row
		}
		return coords[i].Col < coords[j].Col
	})
	return coords
}

// ColName converts a 1-based column number to letters (1 -> A, 27 -> AA).
func ColName(n int) string {
	var b []byte
	for n > 0 {
		n--
		b = append([]byte{byte('A' + n%26)}, b...)
		n /= 26
	}
	return string(b)
}

// ParseCoord converts an A1-style name to a Coord.
func ParseCoord(name string) (Coord, bool) {
	name = strings.ToUpper(name)
	i := 0
	col := 0
	for i < len(name) && name[i] >= 'A' && name[i] <= 'Z' {
		col = col*26 + int(name[i]-'A') + 1
		i++
		if col > 1<<20 {
			return Coord{}, false
		}
	}
	if i == 0 || i == len(name) {
		return Coord{}, false
	}
	row := 0
	for ; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			return Coord{}, false
		}
		row = row*10 + int(name[i]-'0')
		if row > 1<<24 {
			return Coord{}, false
		}
	}
	if row == 0 {
		return Coord{}, false
	}
	return Coord{Row: row, Col: col}, true
}

// cellCount returns the total number of cells in the workbook.
func (w *Workbook) cellCount() int {
	n := 0
	for _, s := range w.Sheets {
		n += len(s.Cells)
	}
	return n
}

// uniqueSheetName returns a sheet name not yet in used, based on name.
func uniqueSheetName(name string, used map[string]bool, index int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = fmt.Sprintf("Sheet%d", index+1)
	}
	base, n := name, 2
	for used[strings.ToLower(name)] {
		name = fmt.Sprintf("%s_%d", base, n)
		n++
	}
	used[strings.ToLower(name)] = true
	return name
}
