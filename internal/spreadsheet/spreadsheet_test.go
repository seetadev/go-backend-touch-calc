package spreadsheet

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func open(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() })
	return f
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return string(b)
}

func sheetByName(t *testing.T, w *Workbook, name string) *Sheet {
	t.Helper()
	for _, s := range w.Sheets {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no sheet %q in %v", name, sheetNames(w))
	return nil
}

func sheetNames(w *Workbook) []string {
	var names []string
	for _, s := range w.Sheets {
		names = append(names, s.Name)
	}
	return names
}

func cell(t *testing.T, s *Sheet, name string) *Cell {
	t.Helper()
	co, ok := ParseCoord(name)
	require.True(t, ok, name)
	c := s.Cells[co]
	require.NotNil(t, c, "%s!%s is empty", s.Name, name)
	return c
}

// assertInvoice checks what both invoice fixtures (xlsx and xls) contain.
func assertInvoice(t *testing.T, w *Workbook, formulas bool) {
	t.Helper()
	require.Equal(t, []string{"Invoice", "Payments", "Summary"}, sheetNames(w))

	inv := sheetByName(t, w, "Invoice")
	require.Equal(t, "Aspiring Traders Pvt Ltd - Tax Invoice", cell(t, inv, "A1").Text)
	require.Equal(t, "Ramesh Kumar (रमेश कुमार)", cell(t, inv, "B3").Text)
	require.Equal(t, "Rate (₹)", cell(t, inv, "C5").Text)
	require.Equal(t, "Amount (₹)", cell(t, inv, "D5").Text)
	require.Equal(t, "GST %", cell(t, inv, "E5").Text)
	require.Equal(t, "Notebook (हिंदी)", cell(t, inv, "A10").Text)
	require.Equal(t, "Thank you for your business / धन्यवाद", cell(t, inv, "A16").Text)

	require.True(t, cell(t, inv, "B6").IsNumber)
	require.Equal(t, 10.0, cell(t, inv, "B6").Num)
	require.Equal(t, 125.5, cell(t, inv, "C6").Num)
	require.Equal(t, 18.0, cell(t, inv, "E6").Num)
	require.InDelta(t, 1255.0, cell(t, inv, "D6").Num, 1e-9)
	require.InDelta(t, 2699.97, cell(t, inv, "D7").Num, 1e-9)
	require.InDelta(t, 8306.22, cell(t, inv, "D12").Num, 1e-9)
	require.InDelta(t, 9374.6771, cell(t, inv, "D14").Num, 1e-9)
	require.Equal(t, "Large", cell(t, inv, "F6").Text)
	require.Equal(t, "Small", cell(t, inv, "F10").Text)

	// a date stays a number with a date format
	date := cell(t, inv, "E2")
	require.True(t, date.IsNumber)
	require.Equal(t, 46280.0, date.Num)
	require.True(t, isDateFormat(date.Format), "format %q", date.Format)
	require.Equal(t, "2026-09-15", serialToTime(date.Num).Format("2006-01-02"))

	// merged ranges: A1:F1, B3:C3, A16:F16
	require.Equal(t, [2]int{6, 1}, [2]int{cell(t, inv, "A1").ColSpan, max(cell(t, inv, "A1").RowSpan, 1)})
	require.Equal(t, 2, cell(t, inv, "B3").ColSpan)
	require.Equal(t, 6, cell(t, inv, "A16").ColSpan)

	pay := sheetByName(t, w, "Payments")
	require.Equal(t, "UPI", cell(t, pay, "B2").Text)
	require.Equal(t, "NEFT", cell(t, pay, "B5").Text)
	require.Equal(t, 1412.0, cell(t, pay, "C2").Num)
	require.Equal(t, 19790.0, cell(t, pay, "D9").Num)
	require.True(t, isDateFormat(cell(t, pay, "A2").Format))
	require.Equal(t, "PAID", cell(t, pay, "G1").Text)

	sum := sheetByName(t, w, "Summary")
	require.Equal(t, "Summary", cell(t, sum, "A1").Text)
	require.InDelta(t, 9374.6771, cell(t, sum, "B2").Num, 1e-9)
	require.InDelta(t, -10415.3229, cell(t, sum, "B4").Num, 1e-9)
	require.Equal(t, 3, cell(t, sum, "A1").ColSpan)

	if formulas {
		require.Equal(t, "B6*C6", cell(t, inv, "D6").Formula)
		require.Equal(t, "SUM(D6:D10)", cell(t, inv, "D12").Formula)
		require.Equal(t, `IF(D6>1000,"Large","Small")`, cell(t, inv, "F6").Formula)
		require.Equal(t, "Invoice!D14", cell(t, sum, "B2").Formula)
		require.Equal(t, "SUM(Payments!C2:C9)", cell(t, sum, "B3").Formula)
	}
}

func TestReadXLSXFixture(t *testing.T) {
	w, err := ReadXLSX(open(t, "invoice.xlsx"))
	require.NoError(t, err)
	assertInvoice(t, w, true)
}

func TestReadXLSFixture(t *testing.T) {
	w, err := ReadXLS(open(t, "invoice.xls"))
	require.NoError(t, err)
	assertInvoice(t, w, false)
}

func TestReadCSVFixture(t *testing.T) {
	w, err := ReadCSV(open(t, "customers.csv"), "customers")
	require.NoError(t, err)
	require.Len(t, w.Sheets, 1)
	s := w.Sheets[0]
	require.Equal(t, "id", cell(t, s, "A1").Text)
	require.Equal(t, 1.0, cell(t, s, "A2").Num)
	require.Equal(t, "Ramesh Kumar", cell(t, s, "B2").Text)
	require.Equal(t, "₹1,25,000 total", cell(t, s, "E2").Text)
	require.Equal(t, "Zoë Müller", cell(t, s, "B3").Text)
	require.Equal(t, "दिल्ली", cell(t, s, "C3").Text)
	require.Equal(t, "line1\nline2", cell(t, s, "E3").Text, "embedded newline survives")
	require.Equal(t, "2025-01-02", cell(t, s, "F2").Text)
}

// The converted values must equal the output of the Tornado backend's converter.
func TestMatchesTornadoOutput(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		read    func() (*Workbook, error)
	}{
		{"invoice.xlsx.sc", func() (*Workbook, error) { return ReadXLSX(open(t, "invoice.xlsx")) }},
		{"invoice.xls.sc", func() (*Workbook, error) { return ReadXLS(open(t, "invoice.xls")) }},
		{"customers.csv.sc", func() (*Workbook, error) { return ReadCSV(open(t, "customers.csv"), "Sheet1") }},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			want, err := Decode(readFile(t, tc.fixture))
			require.NoError(t, err)
			got, err := tc.read()
			require.NoError(t, err)

			require.Equal(t, sheetNames(want), sheetNames(got))
			for i, ws := range want.Sheets {
				gs := got.Sheets[i]
				for co, wc := range ws.Cells {
					gc := gs.Cells[co]
					require.NotNil(t, gc, "%s!%s missing", ws.Name, co.Name())
					switch {
					case wc.IsNumber:
						require.True(t, gc.IsNumber, "%s!%s", ws.Name, co.Name())
						require.InDelta(t, wc.Num, gc.Num, 1e-9, "%s!%s", ws.Name, co.Name())
					case gc.IsNumber && isDateFormat(gc.Format):
						// the old converter wrote dates as text
						require.Equal(t, wc.Text, serialToTime(gc.Num).Format("2006-01-02 15:04:05"), "%s!%s", ws.Name, co.Name())
					default:
						require.Equal(t, wc.Text, gc.Text, "%s!%s", ws.Name, co.Name())
					}
				}
			}
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	src, err := ReadXLSX(open(t, "invoice.xlsx"))
	require.NoError(t, err)

	saved, err := Encode(src)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(saved, `{"numsheets":3,"currentid":"sheet1","currentname":"Invoice"`), saved[:80])
	require.Contains(t, saved, `"sheetArr":{"sheet1":{"name":"Invoice"`)

	back, err := Decode(saved)
	require.NoError(t, err)
	assertInvoice(t, back, true)
}

func TestSaveFormatEscaping(t *testing.T) {
	s := newSheet("S")
	s.Cells[Coord{1, 1}] = &Cell{Text: "a:b\\c\nd"}
	s.Cells[Coord{1, 2}] = &Cell{IsNumber: true, Num: 1234.5, Format: "#,##0.00"}
	s.Cells[Coord{2, 1}] = &Cell{Text: "x", ColSpan: 3, RowSpan: 2}
	out := EncodeSheet(s)
	require.Equal(t, "version:1.5\n"+
		`cell:A1:t:a\cb\bc\nd`+"\n"+
		"cell:B1:v:1234.5:ntvf:1\n"+
		"cell:A2:t:x:colspan:3:rowspan:2\n"+
		`valueformat:1:#,##0.00`+"\n", out)

	w, err := Decode(out)
	require.NoError(t, err)
	got := w.Sheets[0]
	require.Equal(t, "a:b\\c\nd", cell(t, got, "A1").Text)
	require.Equal(t, "#,##0.00", cell(t, got, "B1").Format)
	require.Equal(t, 3, cell(t, got, "A2").ColSpan)
	require.Equal(t, 2, cell(t, got, "A2").RowSpan)
}

func TestWriteXLSXRoundTrip(t *testing.T) {
	src, err := ReadXLSX(open(t, "invoice.xlsx"))
	require.NoError(t, err)

	data, err := WriteXLSX(src)
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(data, []byte("PK")), "xlsx is a zip file")

	back, err := ReadXLSX(bytes.NewReader(data))
	require.NoError(t, err)
	assertInvoice(t, back, true)
}

func TestWriteXLSXKeepsTextThatLooksLikeNumbersOrFormulas(t *testing.T) {
	s := newSheet("S")
	s.Cells[Coord{1, 1}] = &Cell{Text: "007"}
	s.Cells[Coord{1, 2}] = &Cell{Text: "=1+1"}
	s.Cells[Coord{1, 3}] = &Cell{Text: "+91 98765"}
	data, err := WriteXLSX(&Workbook{Sheets: []*Sheet{s}})
	require.NoError(t, err)

	back, err := ReadXLSX(bytes.NewReader(data))
	require.NoError(t, err)
	got := back.Sheets[0]
	for name, want := range map[string]string{"A1": "007", "B1": "=1+1", "C1": "+91 98765"} {
		c := cell(t, got, name)
		require.False(t, c.IsNumber, name)
		require.Empty(t, c.Formula, name)
		require.Equal(t, want, c.Text, name)
	}
}

func TestWriteXLSXSheetNames(t *testing.T) {
	w := &Workbook{Sheets: []*Sheet{newSheet("Q1/Q2: [draft]"), newSheet("Q1/Q2: [draft]"), newSheet(strings.Repeat("é", 40))}}
	for _, s := range w.Sheets {
		s.Cells[Coord{1, 1}] = &Cell{Text: "x"}
	}
	data, err := WriteXLSX(w)
	require.NoError(t, err)
	back, err := ReadXLSX(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, []string{"Q1_Q2_ _draft_", "Q1_Q2_ _draft__2", strings.Repeat("é", 31)}, sheetNames(back))
}

func TestWriteCSV(t *testing.T) {
	src, err := ReadXLSX(open(t, "invoice.xlsx"))
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, WriteCSV(&buf, sheetByName(t, src, "Invoice")))
	out := buf.String()
	require.True(t, strings.HasPrefix(out, "\xEF\xBB\xBF"), "UTF-8 byte order mark for Excel")
	require.Contains(t, out, "Ramesh Kumar (रमेश कुमार)")
	require.Contains(t, out, "Rate (₹)")
	require.Contains(t, out, "2026-09-15", "dates are written as ISO dates")
	require.Contains(t, out, "9374.6771")
	require.Contains(t, out, "Widget A,10,125.5,1255,18,Large")
}

func TestCSVRoundTrip(t *testing.T) {
	src, err := ReadCSV(open(t, "customers.csv"), "customers")
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, WriteCSV(&buf, src.Sheets[0]))
	back, err := ReadCSV(&buf, "customers")
	require.NoError(t, err)

	require.Equal(t, len(src.Sheets[0].Cells), len(back.Sheets[0].Cells))
	for co, want := range src.Sheets[0].Cells {
		got := back.Sheets[0].Cells[co]
		require.NotNil(t, got, co.Name())
		require.Equal(t, want.IsNumber, got.IsNumber, co.Name())
		require.Equal(t, want.Text, got.Text, co.Name())
		require.Equal(t, want.Num, got.Num, co.Name())
	}
}

func TestWriteCSVNeutralisesFormulaText(t *testing.T) {
	s := newSheet("S")
	s.Cells[Coord{1, 1}] = &Cell{Text: "=HYPERLINK(\"http://example.com\")"}
	s.Cells[Coord{1, 2}] = &Cell{IsNumber: true, Num: -5}
	s.Cells[Coord{1, 3}] = &Cell{Text: "plain"}
	var buf bytes.Buffer
	require.NoError(t, WriteCSV(&buf, s))
	require.Equal(t, "\xEF\xBB\xBF\"'=HYPERLINK(\"\"http://example.com\"\")\",-5,plain\n", buf.String())
}

func TestReadCSVEncodingsAndDelimiters(t *testing.T) {
	cases := map[string]struct {
		in   []byte
		want []string
	}{
		"utf8 with BOM":   {[]byte("\xEF\xBB\xBFa,b\n₹1,2\n"), []string{"a", "b", "₹1", "2"}},
		"semicolons":      {[]byte("a;b\n1,5;x\n"), []string{"a", "b", "1,5", "x"}},
		"tabs":            {[]byte("a\tb\nc\td\n"), []string{"a", "b", "c", "d"}},
		"windows-1252":    {[]byte("caf\xe9;\x80\n"), []string{"café", "€"}},
		"utf-16 with BOM": {[]byte{0xFF, 0xFE, 'a', 0, ',', 0, 0xB9, 0x20, 0x0A, 0}, []string{"a", "₹"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w, err := ReadCSV(bytes.NewReader(tc.in), "")
			require.NoError(t, err)
			var got []string
			s := w.Sheets[0]
			for _, co := range s.sortedCoords() {
				c := s.Cells[co]
				if c.IsNumber {
					got = append(got, formatNum(c.Num))
				} else {
					got = append(got, c.Text)
				}
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func TestReadCSVKeepsNumbersThatAreNotNumbers(t *testing.T) {
	w, err := ReadCSV(strings.NewReader("007,+91,1e3,NaN,inf,0x10,1_000,12.50,-3\n"), "")
	require.NoError(t, err)
	s := w.Sheets[0]
	for i, want := range []struct {
		isNum bool
		text  string
		num   float64
	}{
		{false, "007", 0}, {false, "+91", 0}, {true, "", 1000}, {false, "NaN", 0},
		{false, "inf", 0}, {false, "0x10", 0}, {false, "1_000", 0}, {true, "", 12.5}, {true, "", -3},
	} {
		c := s.Cells[Coord{1, i + 1}]
		require.Equal(t, want.isNum, c.IsNumber, "col %d", i+1)
		require.Equal(t, want.text, c.Text, "col %d", i+1)
		require.Equal(t, want.num, c.Num, "col %d", i+1)
	}
}

func TestInvalidInputIsAnErrorNotAPanic(t *testing.T) {
	good, err := os.ReadFile(filepath.Join("testdata", "invoice.xls"))
	require.NoError(t, err)
	garbage := bytes.Repeat([]byte{0xD0, 0xCF, 0x11, 0xE0, 0x01, 0x02}, 500)
	truncated := good[:len(good)/2]

	for name, in := range map[string][]byte{"empty": nil, "text": []byte("hello"), "garbage": garbage, "truncated xls": truncated} {
		t.Run("xls/"+name, func(t *testing.T) {
			_, err := ReadXLS(bytes.NewReader(in))
			require.Error(t, err)
		})
		t.Run("xlsx/"+name, func(t *testing.T) {
			_, err := ReadXLSX(bytes.NewReader(in))
			require.Error(t, err)
		})
	}

	// Bytes flipped throughout a good file must never panic.
	for i := 0; i < len(good); i += 7 {
		mutated := append([]byte(nil), good...)
		mutated[i] ^= 0xFF
		_, _ = ReadXLS(bytes.NewReader(mutated))
	}
}

func TestDecodePlainCoordinateLines(t *testing.T) {
	w, err := Decode("A1:Welcome to TouchCalc\nB1:Hello a@b.com\n\nA2:Start editing here\n")
	require.NoError(t, err)
	s := w.Sheets[0]
	require.Equal(t, "Welcome to TouchCalc", cell(t, s, "A1").Text)
	require.Equal(t, "Hello a@b.com", cell(t, s, "B1").Text)
	require.Equal(t, "Start editing here", cell(t, s, "A2").Text)
}

func TestDecodeRejectsNonWorkbooks(t *testing.T) {
	for _, in := range []string{"", "   ", "just some text", `{"hello":"world"}`, `{"numsheets":1,"sheetArr":{}}`, `{not json`} {
		_, err := Decode(in)
		require.Error(t, err, in)
	}
}

func TestDateFormats(t *testing.T) {
	for code, want := range map[string]bool{
		"yyyy-mm-dd": true, "dd-mmm-yy": true, "h:mm:ss": true, "[h]:mm:ss": true, "m/d/yyyy h:mm": true,
		"General": false, "": false, "0.00": false, "#,##0.00": false, `"day" 0`: false, "0.00E+00": false,
		`[$₹-4009]#,##0.00`: false, "0%": false, `\d0`: false,
	} {
		require.Equal(t, want, isDateFormat(code), code)
	}
}

func TestSocialCalcFormat(t *testing.T) {
	for in, want := range map[string]string{
		"General":                    "",
		"0.00":                       "0.00",
		`[$₹-4009]#,##0.00`:          `"₹"#,##0.00`,
		`#,##0.00_);[Red](#,##0.00)`: `#,##0.00;(#,##0.00)`,
		"[h]:mm:ss":                  "[h]:mm:ss",
		"yyyy-mm-dd":                 "yyyy-mm-dd",
	} {
		require.Equal(t, want, socialCalcFormat(in), in)
	}
}

func TestCoords(t *testing.T) {
	for name, want := range map[string]Coord{"A1": {1, 1}, "z9": {9, 26}, "AA10": {10, 27}, "XFD1048576": {1048576, 16384}} {
		got, ok := ParseCoord(name)
		require.True(t, ok, name)
		require.Equal(t, want, got)
		require.Equal(t, strings.ToUpper(name), got.Name())
	}
	for _, bad := range []string{"", "A", "1", "A0", "1A", "A-1", "A1B"} {
		_, ok := ParseCoord(bad)
		require.False(t, ok, bad)
	}
}
