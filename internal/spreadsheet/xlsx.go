package spreadsheet

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

const (
	// unzipLimit bounds the total uncompressed size of an xlsx (zip bomb guard).
	unzipLimit = 200 << 20
	// unzipXMLLimit bounds a single XML part.
	unzipXMLLimit = 64 << 20
)

// ReadXLSX parses an .xlsx workbook: every sheet, text and numbers, formulas
// with their cached results, number formats and merged ranges.
func ReadXLSX(r io.Reader) (*Workbook, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxFileBytes {
		return nil, ErrTooLarge
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw), excelize.Options{
		UnzipSizeLimit:    unzipLimit,
		UnzipXMLSizeLimit: unzipXMLLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("not a valid xlsx file: %w", err)
	}
	defer f.Close()

	names := f.GetSheetList()
	if len(names) == 0 {
		return nil, fmt.Errorf("workbook has no sheets")
	}
	if len(names) > MaxSheets {
		return nil, ErrTooLarge
	}
	date1904 := f.WorkBook != nil && f.WorkBook.WorkbookPr != nil && f.WorkBook.WorkbookPr.Date1904

	w := &Workbook{Current: f.GetActiveSheetIndex()}
	if w.Current < 0 || w.Current >= len(names) {
		w.Current = 0
	}
	used := map[string]bool{}
	styles := map[int]string{}
	for i, name := range names {
		s := newSheet(uniqueSheetName(name, used, i))
		if err := readXLSXSheet(f, name, s, styles, date1904); err != nil {
			return nil, err
		}
		w.Sheets = append(w.Sheets, s)
		if w.cellCount() > MaxCells {
			return nil, ErrTooLarge
		}
	}
	return w, nil
}

func readXLSXSheet(f *excelize.File, name string, s *Sheet, styles map[int]string, date1904 bool) error {
	// The declared <dimension> is not reliable (some writers never update it),
	// so walk the rows as they are stored. A row is as long as its last cell,
	// including formula cells that have no cached value.
	data, err := f.GetRows(name, excelize.Options{RawCellValue: true})
	if err != nil {
		return err
	}
	for ri, row := range data {
		for ci, raw := range row {
			axis, err := excelize.CoordinatesToCellName(ci+1, ri+1)
			if err != nil {
				return err
			}
			formula, _ := f.GetCellFormula(name, axis)
			if raw == "" && formula == "" {
				continue
			}
			row, col := ri+1, ci+1
			cell := &Cell{Formula: strings.TrimPrefix(formula, "=")}
			typ, _ := f.GetCellType(name, axis)
			format := ""
			if styleID, err := f.GetCellStyle(name, axis); err == nil && styleID != 0 {
				format = xlsxFormat(f, styleID, styles)
			}
			switch typ {
			case excelize.CellTypeSharedString, excelize.CellTypeInlineString, excelize.CellTypeFormula:
				cell.Text = raw
			case excelize.CellTypeBool:
				cell.Text = map[bool]string{true: "TRUE", false: "FALSE"}[raw == "1" || strings.EqualFold(raw, "true")]
			case excelize.CellTypeError:
				cell.Text = raw
			case excelize.CellTypeDate:
				if t, err := time.Parse(time.RFC3339, raw); err == nil {
					cell.IsNumber, cell.Num, format = true, timeToSerial(t), "yyyy-mm-dd"
				} else {
					cell.Text = raw
				}
			default: // number, or unset (a formula without a cached value)
				if v, err := strconv.ParseFloat(raw, 64); err == nil {
					cell.IsNumber, cell.Num = true, v
					if date1904 && isDateFormat(format) {
						cell.Num += 1462
					}
				} else {
					cell.Text = raw
				}
			}
			if cell.IsNumber {
				cell.Format = format
			}
			s.Cells[Coord{row, col}] = cell
			if len(s.Cells) > MaxCells {
				return ErrTooLarge
			}
		}
	}

	merges, _ := f.GetMergeCells(name)
	for _, m := range merges {
		c1, r1, err1 := excelize.CellNameToCoordinates(m.GetStartAxis())
		c2, r2, err2 := excelize.CellNameToCoordinates(m.GetEndAxis())
		if err1 != nil || err2 != nil || c2 < c1 || r2 < r1 {
			continue
		}
		cell := s.Cells[Coord{r1, c1}]
		if cell == nil {
			cell = &Cell{}
			s.Cells[Coord{r1, c1}] = cell
		}
		cell.ColSpan, cell.RowSpan = c2-c1+1, r2-r1+1
	}
	return nil
}

// xlsxFormat returns the SocialCalc-ready number format of a cell style, cached by style id.
func xlsxFormat(f *excelize.File, styleID int, cache map[int]string) string {
	if v, ok := cache[styleID]; ok {
		return v
	}
	format := ""
	if st, err := f.GetStyle(styleID); err == nil && st != nil {
		if st.CustomNumFmt != nil {
			format = *st.CustomNumFmt
		} else {
			format = builtinFormat(st.NumFmt)
		}
	}
	format = socialCalcFormat(format)
	cache[styleID] = format
	return format
}

// WriteXLSX renders the workbook as an .xlsx file.
func WriteXLSX(w *Workbook) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	used := map[string]bool{}
	formats := map[string]int{}
	formulas := make([]map[string]*Cell, len(w.Sheets)) // per sheet: formula cells by name
	for i, s := range w.Sheets {
		formulas[i] = map[string]*Cell{}
		name := uniqueSheetName(sanitizeSheetName(s.Name), used, i)
		if i == 0 {
			if err := f.SetSheetName("Sheet1", name); err != nil {
				return nil, err
			}
		} else if _, err := f.NewSheet(name); err != nil {
			return nil, err
		}
		for _, co := range s.sortedCoords() {
			c := s.Cells[co]
			if err := writeXLSXCell(f, name, co, c, formats); err != nil {
				return nil, err
			}
			if c.Formula != "" {
				formulas[i][co.Name()] = c
			}
		}
	}
	if w.Current >= 0 && w.Current < len(w.Sheets) {
		f.SetActiveSheet(w.Current)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return addCachedFormulaResults(buf.Bytes(), formulas)
}

// excelize writes a formula cell without its calculated value. Put the value we
// have into each formula cell so readers that do not recalculate (previews,
// pandas, openpyxl) see it: a number as a plain <v>, text as t="str".
func addCachedFormulaResults(data []byte, formulas []map[string]*Cell) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, zf := range zr.File {
		rc, err := zf.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		if m := worksheetPart.FindStringSubmatch(zf.Name); m != nil {
			if n, _ := strconv.Atoi(m[1]); n >= 1 && n <= len(formulas) && len(formulas[n-1]) > 0 {
				cells := formulas[n-1]
				content = formulaCell.ReplaceAllFunc(content, func(cell []byte) []byte {
					parts := formulaCell.FindSubmatch(cell)
					c := cells[string(parts[1])]
					if c == nil {
						return cell
					}
					var b bytes.Buffer
					b.WriteString(`<c r="` + string(parts[1]) + `"` + string(parts[2]))
					if c.IsNumber {
						b.WriteString(`><f>` + string(parts[3]) + `</f><v>` + formatNum(c.Num) + `</v></c>`)
					} else {
						b.WriteString(` t="str"><f>` + string(parts[3]) + `</f><v>`)
						_ = xml.EscapeText(&b, []byte(c.Text))
						b.WriteString(`</v></c>`)
					}
					return b.Bytes()
				})
			}
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: zf.Name, Method: zip.Deflate, Modified: zf.Modified})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(content); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

var (
	worksheetPart = regexp.MustCompile(`^xl/worksheets/sheet([0-9]+)\.xml$`)
	formulaCell   = regexp.MustCompile(`<c r="([A-Z]+[0-9]+)"((?: [A-Za-z:]+="[^"]*")*?) t="str"><f>(.*?)</f></c>`)
)

func writeXLSXCell(f *excelize.File, sheet string, co Coord, c *Cell, formats map[string]int) error {
	axis := co.Name()
	var err error
	switch {
	case c.Formula != "":
		err = f.SetCellFormula(sheet, axis, c.Formula) // cached value is added afterwards
	case c.IsNumber:
		err = f.SetCellFloat(sheet, axis, c.Num, -1, 64)
	default:
		err = f.SetCellStr(sheet, axis, c.Text)
	}
	if err != nil {
		return err
	}
	if c.Format != "" {
		id, ok := formats[c.Format]
		if !ok {
			code := c.Format
			if id, err = f.NewStyle(&excelize.Style{CustomNumFmt: &code}); err != nil {
				return err
			}
			formats[c.Format] = id
		}
		if err := f.SetCellStyle(sheet, axis, axis, id); err != nil {
			return err
		}
	}
	if c.ColSpan > 1 || c.RowSpan > 1 {
		cs, rs := max(c.ColSpan, 1), max(c.RowSpan, 1)
		end := Coord{co.Row + rs - 1, co.Col + cs - 1}.Name()
		if err := f.MergeCell(sheet, axis, end); err != nil {
			return err
		}
	}
	return nil
}

// sanitizeSheetName makes name valid as an Excel sheet name: at most 31
// characters, none of []:*?/\ and not starting or ending with an apostrophe.
func sanitizeSheetName(name string) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`[]:*?/\`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, "'")
	if r := []rune(name); len(r) > 31 {
		name = string(r[:31])
	}
	return name
}
