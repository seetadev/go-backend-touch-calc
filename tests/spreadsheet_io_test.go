package tests

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/c4gt/tornado-nginx-go-backend/internal/auth"
	"github.com/c4gt/tornado-nginx-go-backend/internal/handlers"
	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
	"github.com/c4gt/tornado-nginx-go-backend/internal/storage"
	"github.com/c4gt/tornado-nginx-go-backend/tests/testutils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// memStore is an in-memory storage.Storage with the semantics of the S3/MinIO
// backend: a file needs its parent directory and creating twice fails.
type memStore struct {
	mu    sync.Mutex
	items map[string]string
}

func newMemStore() *memStore { return &memStore{items: map[string]string{}} }

func (m *memStore) put(path []string, typ string, data interface{}) error {
	j, err := models.NewStorageItem(path, typ, data).ToJSON()
	if err != nil {
		return err
	}
	m.items[strings.Join(path, "/")] = j
	return nil
}

func (m *memStore) CreateDir(path []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.items[strings.Join(path, "/")]; ok {
		return fmt.Errorf("directory already exists")
	}
	return m.put(path, "dir", []string{})
}
func (m *memStore) DeleteDir(path []string) error { return nil }
func (m *memStore) CreateFile(path []string, data string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.Join(path, "/")
	if _, ok := m.items[strings.Join(path[:len(path)-1], "/")]; !ok {
		return fmt.Errorf("parent directory does not exist")
	}
	if _, ok := m.items[key]; ok {
		return fmt.Errorf("file already exists")
	}
	return m.put(path, "file", data)
}
func (m *memStore) GetFile(path []string) (*models.StorageItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.items[strings.Join(path, "/")]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return models.StorageItemFromJSON(j)
}
func (m *memStore) UpdateFile(path []string, data string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.items[strings.Join(path, "/")]; !ok {
		return storage.ErrNotFound
	}
	return m.put(path, "file", data)
}
func (m *memStore) DeleteFile(path []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, strings.Join(path, "/"))
	return nil
}
func (m *memStore) PutItem(path, data string, _ ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[path] = data
	return nil
}
func (m *memStore) GetItem(path string, _ ...string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.items[path]; ok {
		return v, nil
	}
	return "", storage.ErrNotFound
}
func (m *memStore) ExistsItem(path string, _ ...string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.items[path]
	return ok, nil
}
func (m *memStore) DeleteItem(path string, _ ...string) error { return nil }

// ioApp is the app with its real templates, backed by memStore.
type ioApp struct {
	router  *gin.Engine
	store   *memStore
	cookies []*http.Cookie
}

func newIOApp(t *testing.T) *ioApp {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router, h := testutils.SetupTestServer(t)
	store := newMemStore()
	h.Storage = store
	h.Auth = handlers.NewAuthHandler(h, auth.NewService(store))
	router.LoadHTMLGlob("../web/templates/*")

	router.POST("/register", h.Auth.HandleRegister)
	router.POST("/login", h.Auth.HandleLogin)
	router.POST("/logout", h.Auth.HandleLogout)
	router.POST("/import", h.WebApp.HandleImportPost)
	router.POST("/save", h.WebApp.HandleSave)
	router.POST("/usersheet", h.WebApp.HandleUserSheet)
	router.POST("/downloadfile", h.WebApp.HandleDownloadFile)
	return &ioApp{router: router, store: store}
}

func (a *ioApp) do(req *http.Request) *httptest.ResponseRecorder {
	for _, c := range a.cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, req)
	if cs := w.Result().Cookies(); len(cs) > 0 {
		a.cookies = cs
	}
	return w
}

func (a *ioApp) postJSON(path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return a.do(req)
}

func (a *ioApp) postForm(path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return a.do(req)
}

func (a *ioApp) upload(filename string, content []byte) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, _ := mw.CreateFormFile("upload", filename)
	part.Write(content)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/import", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return a.do(req)
}

func (a *ioApp) registerAndLogin(t *testing.T) {
	t.Helper()
	creds := `{"email":"alice@example.com","password":"s3cret-pass"}`
	require.Equal(t, 200, a.postJSON("/register", creds).Code)
	a.cookies = nil
	require.Equal(t, 200, a.postJSON("/login", creds).Code)
	require.NotEmpty(t, a.cookies, "login sets a cookie")
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "internal", "spreadsheet", "testdata", name))
	require.NoError(t, err)
	return b
}

// storedWorkbook returns the SocialCalc workbook stored for alice's file.
func (a *ioApp) storedWorkbook(t *testing.T, name string) map[string]interface{} {
	t.Helper()
	item, err := a.store.GetFile([]string{"home", "alice@example.com", name})
	require.NoError(t, err, "stored file %q", name)
	var rec struct{ Data string }
	require.NoError(t, json.Unmarshal([]byte(item.Data.(string)), &rec))
	var wb map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(rec.Data), &wb), "stored data is a SocialCalc workbook")
	return wb
}

func sheetNamesOf(wb map[string]interface{}) []string {
	arr := wb["sheetArr"].(map[string]interface{})
	var names []string
	for i := 1; i <= len(arr); i++ {
		names = append(names, arr[fmt.Sprintf("sheet%d", i)].(map[string]interface{})["name"].(string))
	}
	return names
}

func TestImportXLSXConvertsToSocialCalc(t *testing.T) {
	app := newIOApp(t)
	app.registerAndLogin(t)

	w := app.upload("invoice.xlsx", fixture(t, "invoice.xlsx"))
	require.Equal(t, 200, w.Code, w.Body.String())
	page := w.Body.String()
	require.NotContains(t, page, "PK\x03\x04", "raw xlsx bytes must not be put in the sheet")
	require.Contains(t, page, "रमेश कुमार")
	require.Contains(t, page, "Rate (₹)")

	wb := app.storedWorkbook(t, "invoice")
	require.EqualValues(t, 3, wb["numsheets"])
	require.Equal(t, []string{"Invoice", "Payments", "Summary"}, sheetNamesOf(wb))
	invoice := wb["sheetArr"].(map[string]interface{})["sheet1"].(map[string]interface{})["sheetstr"].(map[string]interface{})["savestr"].(string)
	require.Contains(t, invoice, `cell:B3:t:Ramesh Kumar (रमेश कुमार)`)
	require.Contains(t, invoice, `cell:D6:vtf:n:1255:B6*C6`)
	require.Contains(t, invoice, `cell:A1:t:Aspiring Traders Pvt Ltd - Tax Invoice:colspan:6`)
}

func TestImportXLSAndCSV(t *testing.T) {
	app := newIOApp(t)
	app.registerAndLogin(t)

	require.Equal(t, 200, app.upload("old.xls", fixture(t, "invoice.xls")).Code)
	require.Equal(t, []string{"Invoice", "Payments", "Summary"}, sheetNamesOf(app.storedWorkbook(t, "old")))

	w := app.upload("customers.csv", fixture(t, "customers.csv"))
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), "Zoë Müller")
	require.Contains(t, w.Body.String(), "दिल्ली")
	require.EqualValues(t, 1, app.storedWorkbook(t, "customers")["numsheets"])
}

func TestImportRejectsBadFiles(t *testing.T) {
	app := newIOApp(t)
	app.registerAndLogin(t)

	for name, tc := range map[string]struct {
		file    string
		content []byte
		status  int
	}{
		"corrupt xlsx":     {"broken.xlsx", []byte("this is not a zip file"), http.StatusUnprocessableEntity},
		"corrupt xls":      {"broken.xls", []byte("this is not an OLE file"), http.StatusUnprocessableEntity},
		"unsupported type": {"page.html", []byte("<html></html>"), http.StatusUnsupportedMediaType},
	} {
		t.Run(name, func(t *testing.T) {
			w := app.upload(tc.file, tc.content)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			_, err := app.store.GetFile([]string{"home", "alice@example.com", strings.TrimSuffix(tc.file, filepath.Ext(tc.file))})
			require.Error(t, err, "nothing is stored for a rejected upload")
		})
	}
}

func TestImportUsesOnlyTheBaseNameOfTheUpload(t *testing.T) {
	app := newIOApp(t)
	app.registerAndLogin(t)

	require.Equal(t, 200, app.upload(`../../users/evil.csv`, []byte("a,b\n")).Code)
	_, err := app.store.GetFile([]string{"home", "alice@example.com", "evil"})
	require.NoError(t, err)
}

func TestImportAgainReplacesTheFile(t *testing.T) {
	app := newIOApp(t)
	app.registerAndLogin(t)

	require.Equal(t, 200, app.upload("data.csv", []byte("a\n1\n")).Code)
	require.Equal(t, 200, app.upload("data.csv", []byte("a\n2\n")).Code)
	wb := app.storedWorkbook(t, "data")
	savestr := wb["sheetArr"].(map[string]interface{})["sheet1"].(map[string]interface{})["sheetstr"].(map[string]interface{})["savestr"].(string)
	require.Contains(t, savestr, "cell:A2:v:2")
}

func exportFile(t *testing.T, app *ioApp, fname, format string) *httptest.ResponseRecorder {
	t.Helper()
	return app.postForm("/downloadfile", url.Values{"fname": {fname}, "format": {format}})
}

func TestExportXLSX(t *testing.T) {
	app := newIOApp(t)
	app.registerAndLogin(t)
	require.Equal(t, 200, app.upload("invoice.xlsx", fixture(t, "invoice.xlsx")).Code)

	w := exportFile(t, app, "invoice", "xlsx")
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", w.Header().Get("Content-Type"))
	disp, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
	require.NoError(t, err)
	require.Equal(t, "attachment", disp)
	require.Equal(t, "invoice.xlsx", params["filename"])

	// a real xlsx package that holds the sheets and the text, not SocialCalc JSON
	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	require.NoError(t, err, "body is a zip package")
	parts := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		parts[f.Name] = string(b)
	}
	for _, name := range []string{"xl/worksheets/sheet1.xml", "xl/worksheets/sheet2.xml", "xl/worksheets/sheet3.xml"} {
		require.Contains(t, parts, name)
	}
	for _, name := range []string{"Invoice", "Payments", "Summary"} {
		require.Contains(t, parts["xl/workbook.xml"], `name="`+name+`"`)
	}
	require.Contains(t, parts["xl/sharedStrings.xml"], "Ramesh Kumar (रमेश कुमार)")
	require.Contains(t, parts["xl/sharedStrings.xml"], "Rate (₹)")
	require.Contains(t, parts["xl/worksheets/sheet1.xml"], "<mergeCell ref=\"A1:F1\"")
	require.Contains(t, parts["xl/worksheets/sheet1.xml"], "<f>B6*C6</f>")
	require.NotContains(t, w.Body.String(), "numsheets")
}

func TestExportCSV(t *testing.T) {
	app := newIOApp(t)
	app.registerAndLogin(t)
	require.Equal(t, 200, app.upload("customers.csv", fixture(t, "customers.csv")).Code)

	w := exportFile(t, app, "customers", "csv")
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
	_, params, err := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
	require.NoError(t, err)
	require.Equal(t, "customers.csv", params["filename"])

	body := strings.TrimPrefix(w.Body.String(), "\xEF\xBB\xBF")
	rows, err := csv.NewReader(strings.NewReader(body)).ReadAll()
	require.NoError(t, err, "body is CSV")
	require.Equal(t, []string{"id", "name", "city", "amount", "note", "joined"}, rows[0])
	require.Equal(t, []string{"1", "Ramesh Kumar", "Chennai", "19891.58", "₹1,25,000 total", "2025-01-02"}, rows[1])
	require.Equal(t, []string{"2", "Zoë Müller", "दिल्ली", "44925.96", "line1\nline2", "2025-01-03"}, rows[2])
	require.NotContains(t, body, "numsheets")
}

func TestExportOfAnEditorSavedSheet(t *testing.T) {
	app := newIOApp(t)
	app.registerAndLogin(t)

	// what the editor page posts: a single-sheet save string
	data := "version:1.5\ncell:A1:t:नाम\ncell:B1:v:42\n"
	require.Equal(t, 200, app.postForm("/save", url.Values{"fname": {"संख्या 1"}, "data": {data}}).Code)

	w := exportFile(t, app, "संख्या 1", "csv")
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Equal(t, "\xEF\xBB\xBFनाम,42\n", w.Body.String())

	disp := w.Header().Get("Content-Disposition")
	require.Contains(t, disp, `filename="_`) // ASCII fallback
	require.Contains(t, disp, "filename*=UTF-8''%E0%A4%B8")
	_, params, err := mime.ParseMediaType(disp)
	require.NoError(t, err)
	require.Equal(t, "संख्या 1.csv", params["filename"])
}

func TestExportErrors(t *testing.T) {
	app := newIOApp(t)
	require.Equal(t, 401, exportFile(t, app, "x", "csv").Code, "needs a login")

	app.registerAndLogin(t)
	require.Equal(t, 404, exportFile(t, app, "missing", "csv").Code)

	require.Equal(t, 200, app.postForm("/save", url.Values{"fname": {"plain"}, "data": {"just some text"}}).Code)
	require.Equal(t, 422, exportFile(t, app, "plain", "xlsx").Code, "stored text is not a spreadsheet")
	require.Equal(t, 400, exportFile(t, app, "plain", "pdf").Code, "unsupported format")
	require.Equal(t, 400, exportFile(t, app, "plain", "").Code)
}

func TestExportMSCReturnsTheStoredWorkbook(t *testing.T) {
	app := newIOApp(t)
	app.registerAndLogin(t)
	require.Equal(t, 200, app.upload("customers.csv", fixture(t, "customers.csv")).Code)

	w := exportFile(t, app, "customers", "msc")
	require.Equal(t, 200, w.Code)
	var wb map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wb))
	require.EqualValues(t, 1, wb["numsheets"])
}

// The whole round trip a user performs: register, log in, import, save, open,
// export, log out. Values, sheets and UTF-8 survive.
func TestImportSaveLoadExportFlow(t *testing.T) {
	app := newIOApp(t)
	app.registerAndLogin(t)

	// import
	w := app.upload("invoice.xlsx", fixture(t, "invoice.xlsx"))
	require.Equal(t, 200, w.Code)

	// save under another name, as the workbook the import produced
	item, err := app.store.GetFile([]string{"home", "alice@example.com", "invoice"})
	require.NoError(t, err)
	var rec struct{ Data string }
	require.NoError(t, json.Unmarshal([]byte(item.Data.(string)), &rec))
	require.Equal(t, 200, app.postForm("/save", url.Values{"fname": {"copy"}, "data": {rec.Data}}).Code)

	// load
	w = app.postForm("/usersheet", url.Values{"pagename": {"copy"}})
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), "रमेश कुमार")

	// export both ways
	require.Equal(t, 200, exportFile(t, app, "copy", "xlsx").Code)
	csvResp := exportFile(t, app, "copy", "csv")
	require.Equal(t, 200, csvResp.Code)
	require.Contains(t, csvResp.Body.String(), "Widget B,3,899.99,2699.97,18,Large")
	require.Contains(t, csvResp.Body.String(), "Notebook (हिंदी)")

	// log out: the cookie is cleared
	w = app.postJSON("/logout", "{}")
	require.Equal(t, 200, w.Code)
	cleared := false
	for _, c := range w.Result().Cookies() {
		if c.Name == "user" && (c.MaxAge < 0 || c.Value == "") {
			cleared = true
		}
	}
	require.True(t, cleared, "logout clears the login cookie")
}
