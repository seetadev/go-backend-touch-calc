package spreadsheet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf16"

	"github.com/richardlehane/mscfb"
)

// BIFF8 record ids used by the reader.
const (
	recBOF        = 0x0809
	recEOF        = 0x000A
	recContinue   = 0x003C
	recFilePass   = 0x002F
	recDate1904   = 0x0022
	recFormat     = 0x041E
	recXF         = 0x00E0
	recBoundSheet = 0x0085
	recSST        = 0x00FC
	recLabelSST   = 0x00FD
	recLabel      = 0x0204
	recNumber     = 0x0203
	recRK         = 0x027E
	recMulRK      = 0x00BD
	recFormula    = 0x0006
	recString     = 0x0207
	recBoolErr    = 0x0205
	recMergeCells = 0x00E5
)

var errBadXLS = errors.New("not a valid xls file")

type biffRecord struct {
	id     uint16
	offset int
	chunks [][]byte // data, then the data of any CONTINUE records that follow
}

func (r *biffRecord) data() []byte { return r.chunks[0] }

// ReadXLS parses a legacy Excel 97-2003 (BIFF8) workbook: every sheet, text,
// numbers, cached formula results, number formats and merged ranges. Formulas
// are imported as their last calculated value; the formula text is not decoded.
func ReadXLS(r io.Reader) (w *Workbook, err error) {
	raw, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxFileBytes {
		return nil, ErrTooLarge
	}
	// The container and record parsers check lengths, but a corrupt file must
	// never take the server down, so turn any remaining panic into an error.
	defer func() {
		if p := recover(); p != nil {
			w, err = nil, fmt.Errorf("%w: %v", errBadXLS, p)
		}
	}()

	stream, err := workbookStream(raw)
	if err != nil {
		return nil, err
	}
	return parseBIFF8(stream)
}

func workbookStream(raw []byte) ([]byte, error) {
	doc, err := mscfb.New(bytes.NewReader(raw))
	if err != nil {
		return nil, errBadXLS
	}
	hasOld := false
	for entry, err := doc.Next(); err == nil && entry != nil; entry, err = doc.Next() {
		switch entry.Name {
		case "Workbook":
			b, err := io.ReadAll(io.LimitReader(doc, MaxFileBytes*2))
			if err != nil {
				return nil, errBadXLS
			}
			return b, nil
		case "Book":
			hasOld = true
		}
	}
	if hasOld {
		return nil, errors.New("Excel 5.0/95 files are not supported; save the file as .xlsx or .xls (97-2003) and try again")
	}
	return nil, errBadXLS
}

func splitRecords(stream []byte) []biffRecord {
	var recs []biffRecord
	for pos := 0; pos+4 <= len(stream); {
		id := binary.LittleEndian.Uint16(stream[pos:])
		n := int(binary.LittleEndian.Uint16(stream[pos+2:]))
		if pos+4+n > len(stream) {
			break
		}
		data := stream[pos+4 : pos+4+n]
		if id == recContinue && len(recs) > 0 {
			last := &recs[len(recs)-1]
			last.chunks = append(last.chunks, data)
		} else {
			recs = append(recs, biffRecord{id: id, offset: pos, chunks: [][]byte{data}})
		}
		pos += 4 + n
	}
	return recs
}

type sheetRef struct {
	name   string
	offset int
}

func parseBIFF8(stream []byte) (*Workbook, error) {
	recs := splitRecords(stream)
	if len(recs) == 0 || recs[0].id != recBOF || len(recs[0].data()) < 2 {
		return nil, errBadXLS
	}
	if v := binary.LittleEndian.Uint16(recs[0].data()); v != 0x0600 {
		return nil, errors.New("Excel 5.0/95 files are not supported; save the file as .xlsx or .xls (97-2003) and try again")
	}

	var (
		sst      []string
		xfFormat []int
		formats  = map[int]string{}
		sheets   []sheetRef
		date1904 bool
	)

	// workbook globals: everything up to the first EOF
	for _, rec := range recs {
		if rec.id == recEOF {
			break
		}
		d := rec.data()
		switch rec.id {
		case recFilePass:
			return nil, errors.New("the file is password protected")
		case recDate1904:
			date1904 = len(d) >= 2 && binary.LittleEndian.Uint16(d) == 1
		case recXF:
			if len(d) >= 4 {
				xfFormat = append(xfFormat, int(binary.LittleEndian.Uint16(d[2:])))
			}
		case recFormat:
			if len(d) >= 2 {
				cr := &chunkReader{chunks: rec.chunks}
				id := int(cr.u16())
				formats[id] = cr.unicodeString16()
			}
		case recBoundSheet:
			if len(d) >= 8 {
				cr := &chunkReader{chunks: [][]byte{d}}
				off := int(cr.u32())
				cr.u8() // visibility
				kind := cr.u8()
				name := cr.unicodeString8()
				if kind == 0 { // worksheet (not chart/macro/VB module)
					sheets = append(sheets, sheetRef{name, off})
				}
			}
		case recSST:
			sst = parseSST(rec.chunks)
		}
	}
	if len(sheets) == 0 {
		return nil, errors.New("workbook has no worksheets")
	}
	if len(sheets) > MaxSheets {
		return nil, ErrTooLarge
	}

	formatFor := func(xf int) string {
		if xf < 0 || xf >= len(xfFormat) {
			return ""
		}
		id := xfFormat[xf]
		code, ok := formats[id]
		if !ok {
			code = builtinFormat(id)
		}
		return socialCalcFormat(code)
	}

	byOffset := map[int]int{}
	for i, r := range recs {
		byOffset[r.offset] = i
	}

	wb := &Workbook{}
	used := map[string]bool{}
	for i, ref := range sheets {
		start, ok := byOffset[ref.offset]
		if !ok {
			return nil, errBadXLS
		}
		s := newSheet(uniqueSheetName(ref.name, used, i))
		if err := parseBIFFSheet(recs[start:], s, sst, formatFor, date1904); err != nil {
			return nil, err
		}
		wb.Sheets = append(wb.Sheets, s)
		if wb.cellCount() > MaxCells {
			return nil, ErrTooLarge
		}
	}
	return wb, nil
}

func parseBIFFSheet(recs []biffRecord, s *Sheet, sst []string, formatFor func(int) string, date1904 bool) error {
	set := func(row, col, xf int, c *Cell) error {
		if row < 0 || col < 0 || row >= 1<<20 || col >= 1<<14 {
			return nil
		}
		if c.IsNumber {
			c.Format = formatFor(xf)
			if date1904 && isDateFormat(c.Format) {
				c.Num += 1462
			}
		}
		s.Cells[Coord{row + 1, col + 1}] = c
		if len(s.Cells) > MaxCells {
			return ErrTooLarge
		}
		return nil
	}
	num := func(row, col, xf int, v float64) error {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil
		}
		return set(row, col, xf, &Cell{IsNumber: true, Num: v})
	}

	depth := 0
	var pendingFormula *Coord // formula cell waiting for its STRING record
	for _, rec := range recs {
		d := rec.data()
		switch rec.id {
		case recBOF:
			depth++
		case recEOF:
			depth--
			if depth <= 0 {
				return nil
			}
		}
		if depth != 1 {
			continue // embedded chart/other substream
		}
		switch rec.id {
		case recLabelSST:
			if len(d) >= 10 {
				idx := int(binary.LittleEndian.Uint32(d[6:]))
				if idx < len(sst) {
					if err := set(int(le16(d, 0)), int(le16(d, 2)), int(le16(d, 4)), &Cell{Text: sst[idx]}); err != nil {
						return err
					}
				}
			}
		case recLabel:
			if len(d) >= 8 {
				cr := &chunkReader{chunks: rec.chunks, pos: 6}
				if err := set(int(le16(d, 0)), int(le16(d, 2)), int(le16(d, 4)), &Cell{Text: cr.unicodeString16()}); err != nil {
					return err
				}
			}
		case recNumber:
			if len(d) >= 14 {
				v := math.Float64frombits(binary.LittleEndian.Uint64(d[6:]))
				if err := num(int(le16(d, 0)), int(le16(d, 2)), int(le16(d, 4)), v); err != nil {
					return err
				}
			}
		case recRK:
			if len(d) >= 10 {
				if err := num(int(le16(d, 0)), int(le16(d, 2)), int(le16(d, 4)), decodeRK(binary.LittleEndian.Uint32(d[6:]))); err != nil {
					return err
				}
			}
		case recMulRK:
			if len(d) >= 6 {
				row, first := int(le16(d, 0)), int(le16(d, 2))
				for i, off := 0, 4; off+6 <= len(d)-2; i, off = i+1, off+6 {
					if err := num(row, first+i, int(le16(d, off)), decodeRK(binary.LittleEndian.Uint32(d[off+2:]))); err != nil {
						return err
					}
				}
			}
		case recBoolErr:
			if len(d) >= 8 {
				text := "FALSE"
				if d[7] == 0 {
					if d[6] != 0 {
						text = "TRUE"
					}
				} else {
					text = biffError(d[6])
				}
				if err := set(int(le16(d, 0)), int(le16(d, 2)), int(le16(d, 4)), &Cell{Text: text}); err != nil {
					return err
				}
			}
		case recFormula:
			pendingFormula = nil
			if len(d) >= 14 {
				row, col, xf := int(le16(d, 0)), int(le16(d, 2)), int(le16(d, 4))
				if d[12] == 0xFF && d[13] == 0xFF { // not a number
					switch d[6] {
					case 0: // string, in the STRING record that follows
						pendingFormula = &Coord{row + 1, col + 1}
						if err := set(row, col, xf, &Cell{}); err != nil {
							return err
						}
					case 1:
						if err := set(row, col, xf, &Cell{Text: map[bool]string{true: "TRUE", false: "FALSE"}[d[8] != 0]}); err != nil {
							return err
						}
					case 2:
						if err := set(row, col, xf, &Cell{Text: biffError(d[8])}); err != nil {
							return err
						}
					}
				} else if err := num(row, col, xf, math.Float64frombits(binary.LittleEndian.Uint64(d[6:]))); err != nil {
					return err
				}
			}
		case recString:
			if pendingFormula != nil {
				cr := &chunkReader{chunks: rec.chunks}
				if c := s.Cells[*pendingFormula]; c != nil {
					c.Text = cr.unicodeString16()
				}
				pendingFormula = nil
			}
		case recMergeCells:
			if len(d) >= 2 {
				n := int(le16(d, 0))
				for i := 0; i < n && 2+(i+1)*8 <= len(d); i++ {
					o := 2 + i*8
					r1, r2, c1, c2 := int(le16(d, o)), int(le16(d, o+2)), int(le16(d, o+4)), int(le16(d, o+6))
					if r2 < r1 || c2 < c1 || (r1 == r2 && c1 == c2) {
						continue
					}
					co := Coord{r1 + 1, c1 + 1}
					cell := s.Cells[co]
					if cell == nil {
						cell = &Cell{}
						s.Cells[co] = cell
					}
					cell.ColSpan, cell.RowSpan = c2-c1+1, r2-r1+1
				}
			}
		}
	}
	return nil
}

func le16(b []byte, off int) uint16 { return binary.LittleEndian.Uint16(b[off:]) }

func decodeRK(rk uint32) float64 {
	var v float64
	if rk&2 != 0 {
		v = float64(int32(rk) >> 2)
	} else {
		v = math.Float64frombits(uint64(rk&0xFFFFFFFC) << 32)
	}
	if rk&1 != 0 {
		v /= 100
	}
	return v
}

func biffError(code byte) string {
	switch code {
	case 0x00:
		return "#NULL!"
	case 0x07:
		return "#DIV/0!"
	case 0x0F:
		return "#VALUE!"
	case 0x17:
		return "#REF!"
	case 0x1D:
		return "#NAME?"
	case 0x24:
		return "#NUM!"
	case 0x2A:
		return "#N/A"
	}
	return "#ERROR!"
}

// parseSST reads the shared string table; strings may continue across CONTINUE records.
func parseSST(chunks [][]byte) []string {
	cr := &chunkReader{chunks: chunks}
	cr.skip(4) // total count
	unique := int(cr.u32())
	if unique < 0 || unique > MaxCells {
		return nil
	}
	out := make([]string, 0, min(unique, 1<<16))
	for i := 0; i < unique && !cr.eof(); i++ {
		out = append(out, cr.unicodeStringExt())
	}
	return out
}

// chunkReader reads little-endian values and BIFF strings from a record and
// its CONTINUE chunks. Every read is bounds checked; past the end it yields zeros.
type chunkReader struct {
	chunks [][]byte
	ci     int
	pos    int
}

func (r *chunkReader) eof() bool {
	for r.ci < len(r.chunks) && r.pos >= len(r.chunks[r.ci]) {
		r.ci++
		r.pos = 0
	}
	return r.ci >= len(r.chunks)
}

func (r *chunkReader) u8() byte {
	if r.eof() {
		return 0
	}
	b := r.chunks[r.ci][r.pos]
	r.pos++
	return b
}

func (r *chunkReader) u16() uint16 { return uint16(r.u8()) | uint16(r.u8())<<8 }
func (r *chunkReader) u32() uint32 { return uint32(r.u16()) | uint32(r.u16())<<16 }

func (r *chunkReader) skip(n int) {
	for ; n > 0 && !r.eof(); n-- {
		r.pos++
	}
}

// chars reads cch characters; at a chunk boundary a new flags byte gives the width.
func (r *chunkReader) chars(cch int, wide bool) string {
	var units []uint16
	var sb strings.Builder
	for cch > 0 {
		if r.pos >= len(r.chunks[min(r.ci, len(r.chunks)-1)]) || r.ci >= len(r.chunks) {
			if r.ci+1 >= len(r.chunks) {
				break
			}
			r.ci++
			r.pos = 0
			wide = r.u8()&1 != 0 // CONTINUE repeats the flags byte of a split string
			continue
		}
		chunk := r.chunks[r.ci]
		if wide {
			if r.pos+2 > len(chunk) {
				r.pos = len(chunk)
				continue
			}
			units = append(units, binary.LittleEndian.Uint16(chunk[r.pos:]))
			r.pos += 2
		} else {
			sb.WriteRune(rune(chunk[r.pos])) // compressed: Latin-1 code points
			r.pos++
		}
		cch--
	}
	if len(units) > 0 {
		return string(utf16.Decode(units))
	}
	return sb.String()
}

// unicodeString16 reads an XLUnicodeString: 2-byte length, flags, characters.
func (r *chunkReader) unicodeString16() string {
	cch := int(r.u16())
	flags := r.u8()
	return r.chars(cch, flags&1 != 0)
}

// unicodeString8 reads a ShortXLUnicodeString: 1-byte length, flags, characters.
func (r *chunkReader) unicodeString8() string {
	cch := int(r.u8())
	flags := r.u8()
	return r.chars(cch, flags&1 != 0)
}

// unicodeStringExt reads an XLUnicodeRichExtendedString (an SST entry).
func (r *chunkReader) unicodeStringExt() string {
	cch := int(r.u16())
	flags := r.u8()
	runs, ext := 0, 0
	if flags&0x08 != 0 {
		runs = int(r.u16())
	}
	if flags&0x04 != 0 {
		ext = int(r.u32())
	}
	s := r.chars(cch, flags&1 != 0)
	r.skip(4*runs + ext)
	return s
}
