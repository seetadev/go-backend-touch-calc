package spreadsheet

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Encode serialises w as the SocialCalc multi-sheet save string:
//
//	{"numsheets":N,"currentid":"sheet1","currentname":"...","sheetArr":{"sheet1":{"name":"...","hidden":"0","sheetstr":{"savestr":"version:1.5\ncell:..."}}}}
func Encode(w *Workbook) (string, error) {
	if len(w.Sheets) == 0 {
		return "", errors.New("workbook has no sheets")
	}
	cur := w.Current
	if cur < 0 || cur >= len(w.Sheets) {
		cur = 0
	}

	type savestr struct {
		Savestr string `json:"savestr"`
	}
	type sheetOut struct {
		Name     string  `json:"name"`
		Hidden   string  `json:"hidden"`
		Sheetstr savestr `json:"sheetstr"`
	}
	out := struct {
		NumSheets     int                 `json:"numsheets"`
		CurrentID     string              `json:"currentid"`
		CurrentName   string              `json:"currentname"`
		SheetArr      map[string]sheetOut `json:"sheetArr"`
		EditableCells map[string]any      `json:"EditableCells"`
	}{
		NumSheets:     len(w.Sheets),
		CurrentID:     fmt.Sprintf("sheet%d", cur+1),
		CurrentName:   w.Sheets[cur].Name,
		SheetArr:      map[string]sheetOut{},
		EditableCells: map[string]any{"allow": false, "cells": map[string]any{}, "constraints": map[string]any{}},
	}
	for i, s := range w.Sheets {
		out.SheetArr[fmt.Sprintf("sheet%d", i+1)] = sheetOut{Name: s.Name, Hidden: "0", Sheetstr: savestr{encodeSheet(s)}}
	}
	b, err := json.Marshal(out)
	return string(b), err
}

// EncodeSheet serialises a single sheet as a SocialCalc sheet save string.
func EncodeSheet(s *Sheet) string { return encodeSheet(s) }

func encodeSheet(s *Sheet) string {
	var b strings.Builder
	b.WriteString("version:1.5\n")

	// Number formats are shared per sheet: valueformat:<n>:<code>
	formatIDs := map[string]int{}
	var formats []string
	for _, c := range s.Cells {
		if c.Format != "" {
			if _, ok := formatIDs[c.Format]; !ok {
				formatIDs[c.Format] = 0
				formats = append(formats, c.Format)
			}
		}
	}
	sortStrings(formats)
	for i, f := range formats {
		formatIDs[f] = i + 1
	}

	for _, co := range s.sortedCoords() {
		c := s.Cells[co]
		b.WriteString("cell:" + co.Name() + ":")
		switch {
		case c.Formula != "" && c.IsNumber:
			b.WriteString("vtf:n:" + formatNum(c.Num) + ":" + esc(c.Formula))
		case c.Formula != "":
			b.WriteString("vtf:t:" + esc(c.Text) + ":" + esc(c.Formula))
		case c.IsNumber:
			b.WriteString("v:" + formatNum(c.Num))
		default:
			b.WriteString("t:" + esc(c.Text))
		}
		if id := formatIDs[c.Format]; c.Format != "" && c.IsNumber {
			b.WriteString(":ntvf:" + strconv.Itoa(id))
		}
		if c.ColSpan > 1 {
			b.WriteString(":colspan:" + strconv.Itoa(c.ColSpan))
		}
		if c.RowSpan > 1 {
			b.WriteString(":rowspan:" + strconv.Itoa(c.RowSpan))
		}
		b.WriteString("\n")
	}
	for i, f := range formats {
		b.WriteString("valueformat:" + strconv.Itoa(i+1) + ":" + esc(f) + "\n")
	}
	return b.String()
}

// Decode parses a stored SocialCalc file: either the multi-sheet JSON envelope
// or a single-sheet save string ("version:1.5\ncell:...").
func Decode(data string) (*Workbook, error) {
	trimmed := strings.TrimSpace(data)
	if strings.HasPrefix(trimmed, "{") {
		return decodeMulti(trimmed)
	}
	if trimmed == "" {
		return nil, errors.New("empty spreadsheet")
	}
	if strings.HasPrefix(trimmed, "version:") || strings.Contains(data, "\ncell:") || strings.HasPrefix(trimmed, "cell:") {
		s, err := decodeSheet("Sheet1", data)
		if err != nil {
			return nil, err
		}
		return &Workbook{Sheets: []*Sheet{s}}, nil
	}
	if s := decodeCoordLines("Sheet1", data); s != nil {
		return &Workbook{Sheets: []*Sheet{s}}, nil
	}
	return nil, errors.New("not a SocialCalc spreadsheet")
}

// decodeCoordLines reads the plain "A1:text" lines the editor page accepts (and
// the default new file uses). It returns nil if no line has that form.
func decodeCoordLines(name, data string) *Sheet {
	s := newSheet(name)
	for _, line := range strings.Split(data, "\n") {
		coord, text, ok := strings.Cut(strings.TrimSuffix(line, "\r"), ":")
		if !ok {
			continue
		}
		if co, ok := ParseCoord(strings.TrimSpace(coord)); ok && strings.TrimSpace(text) != "" {
			s.Cells[co] = &Cell{Text: strings.TrimSpace(text)}
		}
	}
	if len(s.Cells) == 0 {
		return nil
	}
	return s
}

func decodeMulti(data string) (*Workbook, error) {
	var in struct {
		CurrentID   string `json:"currentid"`
		CurrentName string `json:"currentname"`
		SheetArr    map[string]struct {
			Name     string `json:"name"`
			Sheetstr struct {
				Savestr string `json:"savestr"`
			} `json:"sheetstr"`
		} `json:"sheetArr"`
	}
	if err := json.Unmarshal([]byte(data), &in); err != nil {
		return nil, fmt.Errorf("not a SocialCalc workbook: %w", err)
	}
	if len(in.SheetArr) == 0 {
		return nil, errors.New("not a SocialCalc workbook: no sheets")
	}
	if len(in.SheetArr) > MaxSheets {
		return nil, ErrTooLarge
	}

	// Order sheets by the number in their id (sheet1, sheet2, ...); ids that do
	// not end in a number keep a stable alphabetical order after those.
	ids := make([]string, 0, len(in.SheetArr))
	for id := range in.SheetArr {
		ids = append(ids, id)
	}
	sortSheetIDs(ids)

	w := &Workbook{}
	used := map[string]bool{}
	for i, id := range ids {
		raw := in.SheetArr[id]
		name := uniqueSheetName(raw.Name, used, i)
		s, err := decodeSheet(name, raw.Sheetstr.Savestr)
		if err != nil {
			return nil, fmt.Errorf("sheet %q: %w", name, err)
		}
		w.Sheets = append(w.Sheets, s)
		if strings.EqualFold(id, in.CurrentID) || (in.CurrentID == "" && name == in.CurrentName) {
			w.Current = i
		}
		if w.cellCount() > MaxCells {
			return nil, ErrTooLarge
		}
	}
	return w, nil
}

func decodeSheet(name, savestr string) (*Sheet, error) {
	s := newSheet(name)
	formats := map[int]string{}
	type pending struct {
		co Coord
		id int
	}
	var withFormat []pending

	for _, line := range strings.Split(savestr, "\n") {
		line = strings.TrimSuffix(line, "\r")
		parts := strings.Split(line, ":")
		switch parts[0] {
		case "valueformat":
			if len(parts) >= 3 {
				if id, err := strconv.Atoi(parts[1]); err == nil {
					formats[id] = unesc(parts[2])
				}
			}
		case "cell":
			if len(parts) < 3 {
				continue
			}
			co, ok := ParseCoord(parts[1])
			if !ok {
				continue
			}
			c := &Cell{}
			fmtID := 0
			for j := 2; j < len(parts); {
				key := parts[j]
				j++
				take := func() string {
					if j < len(parts) {
						v := parts[j]
						j++
						return v
					}
					return ""
				}
				switch key {
				case "v":
					c.IsNumber, c.Num = true, parseNum(unesc(take()))
				case "t":
					c.Text = unesc(take())
				case "vt":
					vt := take()
					val := unesc(take())
					if strings.HasPrefix(vt, "n") {
						c.IsNumber, c.Num = true, parseNum(val)
					} else {
						c.Text = val
					}
				case "vtf", "vtc":
					vt := take()
					val := unesc(take())
					f := unesc(take())
					if strings.HasPrefix(vt, "n") {
						c.IsNumber, c.Num = true, parseNum(val)
					} else {
						c.Text = val
					}
					if key == "vtf" {
						c.Formula = f
					}
				case "ntvf":
					fmtID, _ = strconv.Atoi(take())
				case "colspan":
					c.ColSpan, _ = strconv.Atoi(take())
				case "rowspan":
					c.RowSpan, _ = strconv.Atoi(take())
				case "b": // borders: four values
					take()
					take()
					take()
					take()
				case "e", "tvf", "l", "f", "c", "bg", "cf", "cssc", "csss", "mod", "comment":
					take()
				}
			}
			s.Cells[co] = c
			if fmtID > 0 {
				withFormat = append(withFormat, pending{co, fmtID})
			}
			if len(s.Cells) > MaxCells {
				return nil, ErrTooLarge
			}
		}
	}
	for _, p := range withFormat {
		if f := formats[p.id]; f != "" && s.Cells[p.co].IsNumber {
			s.Cells[p.co].Format = f
		}
	}
	return s, nil
}

func parseNum(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func formatNum(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

// esc is SocialCalc's encodeForSave.
func esc(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, `\`, `\b`)
	s = strings.ReplaceAll(s, ":", `\c`)
	return strings.ReplaceAll(s, "\n", `\n`)
}

// unesc is SocialCalc's decodeFromSave.
func unesc(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	s = strings.ReplaceAll(s, `\c`, ":")
	s = strings.ReplaceAll(s, `\n`, "\n")
	return strings.ReplaceAll(s, `\b`, `\`)
}
