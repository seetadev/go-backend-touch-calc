package spreadsheet

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

var plainNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// ReadCSV parses CSV data into a single-sheet workbook. It accepts UTF-8 (with or
// without a BOM), UTF-16 with a BOM, and falls back to Windows-1252 for other
// bytes, and picks the delimiter among comma, semicolon and tab. Fields that are
// plain decimal numbers become numbers; everything else stays text.
func ReadCSV(r io.Reader, sheetName string) (*Workbook, error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxFileBytes {
		return nil, ErrTooLarge
	}
	text, err := decodeText(raw)
	if err != nil {
		return nil, err
	}

	cr := csv.NewReader(strings.NewReader(text))
	cr.Comma = sniffDelimiter(text)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true

	if sheetName == "" {
		sheetName = "Sheet1"
	}
	s := newSheet(sheetName)
	for row := 1; ; row++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		for i, field := range rec {
			if field == "" {
				continue
			}
			c := &Cell{}
			if plainNumber.MatchString(strings.TrimSpace(field)) {
				if f, err := strconv.ParseFloat(strings.TrimSpace(field), 64); err == nil {
					c.IsNumber, c.Num = true, f
				}
			}
			if !c.IsNumber {
				c.Text = field
			}
			s.Cells[Coord{row, i + 1}] = c
			if len(s.Cells) > MaxCells {
				return nil, ErrTooLarge
			}
		}
	}
	return &Workbook{Sheets: []*Sheet{s}}, nil
}

func decodeText(raw []byte) (string, error) {
	switch {
	case bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}):
		raw = raw[3:]
	case bytes.HasPrefix(raw, []byte{0xFF, 0xFE}), bytes.HasPrefix(raw, []byte{0xFE, 0xFF}):
		out, err := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder().Bytes(raw)
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	if utf8.Valid(raw) {
		return string(raw), nil
	}
	out, err := charmap.Windows1252.NewDecoder().Bytes(raw)
	if err != nil {
		return "", errors.New("unsupported text encoding")
	}
	return string(out), nil
}

// sniffDelimiter picks the most frequent of , ; and tab in the first line outside quotes.
func sniffDelimiter(text string) rune {
	counts := map[rune]int{}
	inQuote := false
	for _, ch := range text {
		if ch == '"' {
			inQuote = !inQuote
		}
		if ch == '\n' && !inQuote {
			break
		}
		if !inQuote && (ch == ',' || ch == ';' || ch == '\t') {
			counts[ch]++
		}
	}
	best, n := ',', 0
	for _, d := range []rune{',', ';', '\t'} {
		if counts[d] > n {
			best, n = d, counts[d]
		}
	}
	return best
}

// WriteCSV writes one sheet as UTF-8 CSV with a byte order mark so Excel
// recognises the encoding. Numbers are written as raw values, date-formatted
// numbers as ISO dates. Text that a spreadsheet program would run as a formula
// (starting with = + - @ or a control character) is prefixed with an apostrophe.
func WriteCSV(w io.Writer, s *Sheet) error {
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	maxRow, maxCol := 0, 0
	for co := range s.Cells {
		if co.Row > maxRow {
			maxRow = co.Row
		}
		if co.Col > maxCol {
			maxCol = co.Col
		}
	}
	for r := 1; r <= maxRow; r++ {
		rec := make([]string, maxCol)
		for c := 1; c <= maxCol; c++ {
			if cell := s.Cells[Coord{r, c}]; cell != nil {
				rec[c-1] = csvValue(cell)
			}
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func csvValue(c *Cell) string {
	if c.IsNumber {
		if isDateFormat(c.Format) {
			t := serialToTime(c.Num)
			switch {
			case c.Num < 1:
				return t.Format("15:04:05")
			case c.Num == float64(int64(c.Num)):
				return t.Format("2006-01-02")
			default:
				return t.Format("2006-01-02 15:04:05")
			}
		}
		return strconv.FormatFloat(c.Num, 'f', -1, 64)
	}
	if c.Text != "" && strings.ContainsRune("=+-@\t\r", rune(c.Text[0])) {
		return "'" + c.Text
	}
	return c.Text
}
