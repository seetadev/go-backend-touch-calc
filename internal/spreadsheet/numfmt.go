package spreadsheet

import (
	"math"
	"regexp"
	"strings"
	"time"
)

// builtinFormats are the number formats Excel numbers 0-49 stand for. Locale
// dependent currency formats (5-8, 41-44) use a dollar sign as Excel does for en-US.
var builtinFormats = map[int]string{
	1: "0", 2: "0.00", 3: "#,##0", 4: "#,##0.00",
	5: `"$"#,##0;("$"#,##0)`, 6: `"$"#,##0;[Red]("$"#,##0)`,
	7: `"$"#,##0.00;("$"#,##0.00)`, 8: `"$"#,##0.00;[Red]("$"#,##0.00)`,
	9: "0%", 10: "0.00%", 11: "0.00E+00", 12: "# ?/?", 13: "# ??/??",
	14: "m/d/yyyy", 15: "d-mmm-yy", 16: "d-mmm", 17: "mmm-yy",
	18: "h:mm AM/PM", 19: "h:mm:ss AM/PM", 20: "h:mm", 21: "h:mm:ss", 22: "m/d/yyyy h:mm",
	37: "#,##0;(#,##0)", 38: "#,##0;[Red](#,##0)", 39: "#,##0.00;(#,##0.00)", 40: "#,##0.00;[Red](#,##0.00)",
	45: "mm:ss", 46: "[h]:mm:ss", 47: "mm:ss.0", 48: "##0.0E+0", 49: "@",
}

// builtinFormat returns the format code for a built-in number format id ("" for General).
func builtinFormat(id int) string {
	if f, ok := builtinFormats[id]; ok {
		return f
	}
	// East Asian date/time formats: show them as plain dates.
	if (id >= 27 && id <= 36) || (id >= 50 && id <= 58) {
		return "m/d/yyyy"
	}
	return ""
}

var (
	quotedOrEscaped = regexp.MustCompile(`"[^"]*"|\\.|_.|\*.`)
	bracketed       = regexp.MustCompile(`\[[^\]]*\]`)
	currencyBracket = regexp.MustCompile(`\[\$([^\]\-]*)(?:-[^\]]*)?\]`)
	elapsedBracket  = regexp.MustCompile(`(?i)\[(h+|m+|s+)\]`)
)

// isDateFormat reports whether an Excel number format code displays a date or time.
func isDateFormat(code string) bool {
	if code == "" || strings.EqualFold(code, "general") {
		return false
	}
	s := quotedOrEscaped.ReplaceAllString(code, "")
	// keep elapsed-time markers such as [h], drop other bracket tokens ([Red], [$-409])
	s = elapsedBracket.ReplaceAllString(s, "$1")
	s = bracketed.ReplaceAllString(s, "")
	if strings.ContainsAny(s, "Ee") && strings.ContainsAny(s, "+-") && strings.ContainsAny(s, "0#") {
		return false // scientific notation
	}
	return strings.ContainsAny(strings.ToLower(s), "dmyhs")
}

// socialCalcFormat turns an Excel format code into one SocialCalc understands:
// currency/locale tokens become quoted text, other bracket tokens and column
// padding are dropped. It returns "" for General.
func socialCalcFormat(code string) string {
	if code == "" || strings.EqualFold(code, "general") {
		return ""
	}
	out := currencyBracket.ReplaceAllString(code, `"$1"`)
	out = elapsedBracket.ReplaceAllString(out, "[$1]")
	// drop remaining bracket tokens ([Red], [$-409]) but keep elapsed markers
	var b strings.Builder
	for i := 0; i < len(out); {
		if out[i] == '[' {
			if j := strings.IndexByte(out[i:], ']'); j > 0 {
				tok := strings.ToLower(out[i+1 : i+j])
				if strings.Trim(tok, "hms") == "" && tok != "" {
					b.WriteString(out[i : i+j+1])
				}
				i += j + 1
				continue
			}
		}
		if (out[i] == '_' || out[i] == '*') && i+1 < len(out) {
			i += 2
			continue
		}
		b.WriteByte(out[i])
		i++
	}
	res := strings.TrimSpace(b.String())
	if strings.EqualFold(res, "general") {
		return ""
	}
	return res
}

var excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// serialToTime converts an Excel serial date number to a time (UTC, to the second).
func serialToTime(serial float64) time.Time {
	secs := math.Round(serial * 86400)
	return excelEpoch.Add(time.Duration(secs) * time.Second)
}

// timeToSerial converts a time to an Excel serial date number.
func timeToSerial(t time.Time) float64 {
	return t.UTC().Sub(excelEpoch).Hours() / 24
}
