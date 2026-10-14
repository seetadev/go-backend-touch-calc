package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
	"github.com/c4gt/tornado-nginx-go-backend/internal/spreadsheet"
	"github.com/gin-gonic/gin"
)

const (
	xlsxContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	csvContentType  = "text/csv; charset=utf-8"

	// uploadOverhead is room for the multipart framing around a file of MaxFileBytes.
	uploadOverhead = 1 << 20
)

// HandleImportPost handles POST requests to /import. xls, xlsx and csv uploads
// are converted to the SocialCalc multi-sheet save format; .msc/.msce/.sc files
// are already in that format and are used as they are.
func (h *WebAppHandler) HandleImportPost(c *gin.Context) {
	session, _ := c.Cookie("session")
	user := h.getCurrentUser(c)

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, spreadsheet.MaxFileBytes+uploadOverhead)
	file, err := c.FormFile("upload")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			importError(c, http.StatusRequestEntityTooLarge, "The file is too large")
			return
		}
		importError(c, http.StatusBadRequest, "No file uploaded")
		return
	}
	if file.Size > spreadsheet.MaxFileBytes {
		importError(c, http.StatusRequestEntityTooLarge, "The file is too large")
		return
	}

	fname := uploadedName(file.Filename)
	src, err := file.Open()
	if err != nil {
		importError(c, http.StatusInternalServerError, "Failed to read file")
		return
	}
	defer src.Close()

	var wbook string // the SocialCalc multi-sheet save string
	ext := strings.ToLower(path.Ext(fname))
	switch ext {
	case ".msc", ".msce", ".sc":
		raw := new(bytes.Buffer)
		if _, err := raw.ReadFrom(src); err != nil {
			importError(c, http.StatusInternalServerError, "Failed to read file")
			return
		}
		wbook = raw.String()
	case ".xlsx", ".xlsm", ".xls", ".csv", ".tsv", ".txt":
		var wb *spreadsheet.Workbook
		switch ext {
		case ".xlsx", ".xlsm":
			wb, err = spreadsheet.ReadXLSX(src)
		case ".xls":
			wb, err = spreadsheet.ReadXLS(src)
		default:
			wb, err = spreadsheet.ReadCSV(src, "Sheet1")
		}
		if err != nil {
			status := http.StatusUnprocessableEntity
			if errors.Is(err, spreadsheet.ErrTooLarge) {
				status = http.StatusRequestEntityTooLarge
			}
			log.Printf("import %q for %s failed: %v", fname, user, err)
			importError(c, status, "Could not read "+fname+": "+err.Error())
			return
		}
		if wbook, err = spreadsheet.Encode(wb); err != nil {
			importError(c, http.StatusInternalServerError, "Failed to convert file")
			return
		}
	default:
		importError(c, http.StatusUnsupportedMediaType, "Unsupported file type. Upload an .xlsx, .xls, .csv or SocialCalc (.msc) file.")
		return
	}

	// If the user is logged in, keep the imported file in their account.
	if user != "" {
		baseName := storedName(strings.TrimSuffix(fname, path.Ext(fname)))
		if err := h.saveUserFile(user, baseName, wbook, true); err != nil {
			log.Printf("import: could not store %q for %s: %v", baseName, user, err)
		}
	}

	c.HTML(http.StatusOK, "importcollabload.html", gin.H{
		"entry": map[string]interface{}{
			"fname":        fname,
			"sheetmscestr": wbook,
			"sheetstr":     editorSheet(wbook),
			"session":      session,
		},
		"user": user,
	})
}

// HandleDownloadFile handles file download requests: format=xlsx or csv convert the
// stored sheet; format=msc returns the stored SocialCalc data unchanged.
func (h *WebAppHandler) HandleDownloadFile(c *gin.Context) {
	user := h.getCurrentUser(c)
	if user == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"result": "fail",
			"data":   "usererror",
		})
		return
	}

	fname := c.PostForm("fname")
	format := strings.ToLower(strings.TrimSpace(c.PostForm("format")))
	if fname == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"result": "fail",
			"data":   "missing filename",
		})
		return
	}
	if format != "csv" && format != "xlsx" && format != "msc" {
		c.JSON(http.StatusBadRequest, gin.H{
			"result": "fail",
			"data":   "unsupported format: use csv, xlsx or msc",
		})
		return
	}

	item, err := h.handler.Storage.GetFile([]string{"home", user, fname})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"result": "fail",
			"data":   "file not found",
		})
		return
	}
	content := storedContent(item)

	var (
		body        []byte
		contentType string
	)
	switch format {
	case "msc":
		body, contentType = []byte(content), "application/octet-stream"
	default:
		wb, err := spreadsheet.Decode(content)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"result": "fail",
				"data":   "file is not a spreadsheet",
			})
			return
		}
		if format == "xlsx" {
			body, err = spreadsheet.WriteXLSX(wb)
			contentType = xlsxContentType
		} else {
			sheet := pickSheet(wb, c.PostForm("sheet"))
			var buf bytes.Buffer
			err = spreadsheet.WriteCSV(&buf, sheet)
			body, contentType = buf.Bytes(), csvContentType
		}
		if err != nil {
			log.Printf("export %q as %s for %s failed: %v", fname, format, user, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"result": "fail",
				"data":   "export failed",
			})
			return
		}
	}

	c.Header("Content-Disposition", contentDisposition(downloadName(fname)+"."+format))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, contentType, body)
}

func importError(c *gin.Context, status int, msg string) {
	c.HTML(status, "importerror.html", gin.H{"error": msg})
}

// saveUserFile stores data as the user's file name, creating or replacing it.
func (h *WebAppHandler) saveUserFile(user, name, data string, imported bool) error {
	fileData := map[string]interface{}{
		"user":      user,
		"fname":     name,
		"data":      data,
		"imported":  imported,
		"timestamp": time.Now().Unix(),
	}
	dataJSON, err := json.Marshal(fileData)
	if err != nil {
		return err
	}

	p := []string{"home", user, name}
	if _, err := h.handler.Storage.GetFile(p); err == nil {
		return h.handler.Storage.UpdateFile(p, string(dataJSON))
	}
	// New file: its parent directory must exist (it may not for a user who has
	// not opened the file list yet).
	if _, err := h.handler.Storage.GetFile([]string{"home", user}); err != nil {
		_ = h.handler.Storage.CreateDir([]string{"home", user})
	}
	return h.handler.Storage.CreateFile(p, string(dataJSON))
}

// storedContent returns the sheet text of a stored file, whether it was saved as
// the {"data": ...} record written by /save and /import, or as plain text.
func storedContent(item *models.StorageItem) string {
	dataStr, ok := item.Data.(string)
	if !ok {
		b, _ := json.Marshal(item.Data)
		return string(b)
	}
	var rec map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &rec); err == nil {
		for _, key := range []string{"data", "content"} {
			if s, ok := rec[key].(string); ok {
				return s
			}
		}
	}
	return dataStr
}

// editorSheet is what the single-sheet editor page can load: the sheet's save
// string when the workbook has one sheet, otherwise the workbook as it is.
func editorSheet(wbook string) string {
	wb, err := spreadsheet.Decode(wbook)
	if err != nil || len(wb.Sheets) != 1 {
		return wbook
	}
	return spreadsheet.EncodeSheet(wb.Sheets[0])
}

// pickSheet returns the sheet named name, else the workbook's current sheet.
func pickSheet(wb *spreadsheet.Workbook, name string) *spreadsheet.Sheet {
	for _, s := range wb.Sheets {
		if name != "" && s.Name == name {
			return s
		}
	}
	if wb.Current >= 0 && wb.Current < len(wb.Sheets) {
		return wb.Sheets[wb.Current]
	}
	return wb.Sheets[0]
}

// uploadedName reduces an uploaded file name to its last path element.
func uploadedName(name string) string {
	name = strings.ReplaceAll(name, `\`, "/")
	return path.Base(name)
}

// storedName makes name safe as one storage path element.
func storedName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '/' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		name = "imported"
	}
	if strings.EqualFold(name, "securestore") {
		name += "-import" // would clash with the per-user app store directory
	}
	return name
}

// downloadName is the file name, without extension, offered to the browser.
func downloadName(fname string) string {
	return storedName(path.Base(strings.ReplaceAll(fname, `\`, "/")))
}

// contentDisposition builds an attachment header that is safe for any file name
// and carries the real (possibly non-ASCII) name in the RFC 6266 filename* form.
func contentDisposition(name string) string {
	var ascii strings.Builder
	for _, r := range name {
		switch {
		case r < 0x20 || r > 0x7e || r == '"' || r == '\\' || r == '%' || r == ';':
			ascii.WriteByte('_')
		default:
			ascii.WriteRune(r)
		}
	}
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii.String(), rfc5987Escape(name))
}

// rfc5987Escape percent-encodes s for the filename* parameter.
func rfc5987Escape(s string) string {
	const attrChars = "!#$&+-.^_`|~"
	var b strings.Builder
	for _, c := range []byte(s) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', strings.IndexByte(attrChars, c) >= 0:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
