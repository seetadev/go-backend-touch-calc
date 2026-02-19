package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/c4gt/tornado-nginx-go-backend/internal/config"
	"github.com/c4gt/tornado-nginx-go-backend/internal/handlers"
	"github.com/c4gt/tornado-nginx-go-backend/internal/session"
	"github.com/c4gt/tornado-nginx-go-backend/tests/testutils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupSpreadsheetTest creates a test server with templates loaded
func setupSpreadsheetTest(t *testing.T) (*gin.Engine, *handlers.Handler) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Environment:    "test",
		Port:           "8080",
		CookieSecret:   "testsecret",
		StorageBackend: "mock",
	}

	router := gin.Default()

	// Load templates from the project root
	// In tests, we need to find the template directory relative to this test file
	templateDir := "../web/templates/*"
	if _, err := os.Stat("../web/templates"); os.IsNotExist(err) {
		// Try from project root
		templateDir = "web/templates/*"
	}
	router.LoadHTMLGlob(templateDir)

	// Serve static files
	router.Static("/static", "../web/static")

	mockStorage := testutils.NewMockStorage()
	sessionMgr := session.NewManager()

	h := &handlers.Handler{
		Config:  cfg,
		Storage: mockStorage,
		Session: sessionMgr,
	}

	h.Auth = handlers.NewAuthHandler(h, nil)
	h.WebApp = handlers.NewWebAppHandler(h)
	h.App = handlers.NewAppHandler(h)

	// Setup routes matching the main.go pattern
	router.GET("/save", h.WebApp.HandleSave)
	router.POST("/save", h.WebApp.HandleSave)
	router.POST("/usersheet", h.WebApp.HandleUserSheet)

	return router, h
}

// addUserCookie adds a user cookie to a request
func addUserCookie(req *http.Request, username string) {
	req.AddCookie(&http.Cookie{
		Name:  "user",
		Value: username,
	})
}

// TestImportcollabloadTemplateScriptOrder verifies the importcollabload.html
// template includes SocialCalc scripts in the correct order and uses
// the proper WorkBook initialization pattern.
func TestImportcollabloadTemplateScriptOrder(t *testing.T) {
	router, h := setupSpreadsheetTest(t)

	// Create a test file in storage
	user := "testuser"
	path := []string{"home", user, "testfile"}
	fileData := map[string]interface{}{
		"user":  user,
		"fname": "testfile",
		"data":  "\n",
	}
	dataJSON, _ := json.Marshal(fileData)
	err := h.Storage.CreateFile(path, string(dataJSON))
	require.NoError(t, err)

	// Request the usersheet page (which renders importcollabload.html)
	form := url.Values{}
	form.Set("pagename", "testfile")
	form.Set("edit", "yes")

	req, _ := http.NewRequest("POST", "/usersheet", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addUserCookie(req, user)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())

	body := w.Body.String()

	// 1. Verify critical scripts are present
	requiredScripts := []string{
		"socialcalcconstants.js",
		"socialcalc-3.js",
		"socialcalctouch.js",
		"socialcalctableeditor.js",
		"formatnumber2.js",
		"formula1.js",
		"socialcalcpopup.js",
		"socialcalcspreadsheetcontrol.js",
		"socialcalcworkbook.js",
		"socialcalcworkbookcontrol.js",
		"socialcalcimages.js",
	}
	for _, script := range requiredScripts {
		assert.Contains(t, body, script, "Missing required script: %s", script)
	}

	// 2. Verify script load order: socialcalcconstants.js must come BEFORE socialcalc-3.js
	constIdx := strings.Index(body, "socialcalcconstants.js")
	scIdx := strings.Index(body, "socialcalc-3.js")
	assert.Greater(t, scIdx, constIdx,
		"socialcalcconstants.js must load BEFORE socialcalc-3.js")

	// 3. Verify formatnumber2.js loads BEFORE socialcalcspreadsheetcontrol.js
	fnIdx := strings.Index(body, "formatnumber2.js")
	sscIdx := strings.Index(body, "socialcalcspreadsheetcontrol.js")
	assert.Greater(t, sscIdx, fnIdx,
		"formatnumber2.js must load BEFORE socialcalcspreadsheetcontrol.js")

	// 4. Verify formula1.js loads BEFORE socialcalcspreadsheetcontrol.js
	f1Idx := strings.Index(body, "formula1.js")
	assert.Greater(t, sscIdx, f1Idx,
		"formula1.js must load BEFORE socialcalcspreadsheetcontrol.js")

	// 5. Verify WorkBook initialization pattern is used (not bare SpreadsheetControl)
	assert.Contains(t, body, "new SocialCalc.WorkBook(spreadsheet)",
		"Must use WorkBook initialization")
	assert.Contains(t, body, "workbook.InitializeWorkBook",
		"Must call InitializeWorkBook")
	assert.Contains(t, body, "new SocialCalc.WorkBookControl(",
		"Must create WorkBookControl")
	assert.Contains(t, body, "workbookcontrol.InitializeWorkBookControl()",
		"Must call InitializeWorkBookControl")

	// 6. Verify data loading uses SocialCalc.WorkBookControlLoad
	assert.Contains(t, body, "SocialCalc.WorkBookControlLoad(",
		"Must use WorkBookControlLoad to load data")

	// 7. Verify data is in a textarea (not a JS template literal)
	assert.Contains(t, body, `<textarea name="savestr" id="sheetdata"`,
		"Data should be in a hidden textarea")

	// 8. Verify spreadsheet.InitializeSpreadsheetControl takes a string ID
	assert.Contains(t, body, `spreadsheet.InitializeSpreadsheetControl("tableeditor")`,
		"InitializeSpreadsheetControl should take string ID 'tableeditor'")

	// 9. Verify body has onresize handler
	assert.Contains(t, body, `onresize="spreadsheet.DoOnResize();"`,
		"Body must have onresize handler for spreadsheet resize")

	// 10. Verify the workbookControl div exists
	assert.Contains(t, body, `id="workbookControl"`,
		"Must have workbookControl div for tab UI")

	// 11. Verify the filename is rendered
	assert.Contains(t, body, "testfile",
		"Filename should appear in the rendered template")
}

// TestSaveAndLoadSpreadsheetData tests the save/load roundtrip for spreadsheet data
func TestSaveAndLoadSpreadsheetData(t *testing.T) {
	router, h := setupSpreadsheetTest(t)

	user := "testuser"

	// Create the user directory structure
	h.Storage.CreateDir([]string{"home"})
	h.Storage.CreateDir([]string{"home", user})

	// Simulate saving SocialCalc data (the format WorkBookControlSaveSheet produces)
	socialCalcData := `socialcalc:version:1.0
sheet:c:5:r:20:tvf:1
cell:A1:t:Hello World:f:1
cell:B1:t:Test Data:f:1
cell:A2:v:42:f:1
sheet:c:5:r:20:tvf:1`

	// POST save request
	form := url.Values{}
	form.Set("fname", "mysheet")
	form.Set("data", socialCalcData)

	req, _ := http.NewRequest("POST", "/save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addUserCookie(req, user)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Save should succeed")

	var saveResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &saveResp)
	require.NoError(t, err)
	assert.Equal(t, "ok", saveResp["result"], "Save result should be 'ok'")

	// Now load the file via usersheet
	form = url.Values{}
	form.Set("pagename", "mysheet")
	form.Set("edit", "yes")

	req, _ = http.NewRequest("POST", "/usersheet", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addUserCookie(req, user)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "UserSheet should load successfully")

	body := w.Body.String()

	// Verify the saved SocialCalc data appears in the textarea
	assert.Contains(t, body, "Hello World",
		"Saved spreadsheet data should be present in the loaded page")
	assert.Contains(t, body, "Test Data",
		"Saved spreadsheet data should be present in the loaded page")
}

// TestSaveGetFileList tests the /save GET endpoint returns the file list page
func TestSaveGetFileList(t *testing.T) {
	router, h := setupSpreadsheetTest(t)

	user := "testuser"

	// Setup: create user directory with some files
	h.Storage.CreateDir([]string{"home", user})

	// Request the file list
	req, _ := http.NewRequest("GET", "/save", nil)
	addUserCookie(req, user)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should render the allusersheets.html template
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "testuser", "Should show the username")
}

// TestSaveGetRedirectsUnauthenticated tests that /save redirects when not logged in
func TestSaveGetRedirectsUnauthenticated(t *testing.T) {
	router, _ := setupSpreadsheetTest(t)

	req, _ := http.NewRequest("GET", "/save", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code, "Should redirect unauthenticated users")
	assert.Equal(t, "/login", w.Header().Get("Location"))
}

// TestUserSheetRedirectsUnauthenticated tests that /usersheet redirects when not logged in
func TestUserSheetRedirectsUnauthenticated(t *testing.T) {
	router, _ := setupSpreadsheetTest(t)

	form := url.Values{}
	form.Set("pagename", "testfile")

	req, _ := http.NewRequest("POST", "/usersheet", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code, "Should redirect unauthenticated users")
}

// TestUserSheetDeleteFile tests the delete operation through /usersheet
func TestUserSheetDeleteFile(t *testing.T) {
	router, h := setupSpreadsheetTest(t)

	user := "testuser"

	// Create a file first
	path := []string{"home", user, "deleteme"}
	fileData := map[string]interface{}{
		"user":  user,
		"fname": "deleteme",
		"data":  "\n",
	}
	dataJSON, _ := json.Marshal(fileData)
	h.Storage.CreateFile(path, string(dataJSON))

	// Delete the file
	form := url.Values{}
	form.Set("pagename", "deleteme")
	form.Set("delete", "yes")

	req, _ := http.NewRequest("POST", "/usersheet", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addUserCookie(req, user)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code, "Should redirect after delete")
	assert.Equal(t, "/save", w.Header().Get("Location"))

	// Verify the file is gone
	_, err := h.Storage.GetFile(path)
	assert.Error(t, err, "File should be deleted from storage")
}

// TestStaticJSFilesExist verifies that all required JS files exist in the static directory
func TestStaticJSFilesExist(t *testing.T) {
	requiredFiles := []string{
		"../web/static/js/socialcalcconstants.js",
		"../web/static/js/socialcalc-3.js",
		"../web/static/js/socialcalctouch.js",
		"../web/static/js/socialcalctableeditor.js",
		"../web/static/js/formatnumber2.js",
		"../web/static/js/formula1.js",
		"../web/static/js/socialcalcpopup.js",
		"../web/static/js/socialcalcspreadsheetcontrol.js",
		"../web/static/js/socialcalcworkbook.js",
		"../web/static/js/socialcalcworkbookcontrol.js",
		"../web/static/js/socialcalcimages.js",
		"../web/static/js/json2.js",
		"../web/static/js/jquery.min.js",
	}

	for _, file := range requiredFiles {
		_, err := os.Stat(file)
		assert.NoError(t, err, "Required JS file missing: %s", file)
	}
}

// TestAmazonWebAppTemplateInitialization verifies the amazonwebapp.html
// template uses the correct SocialCalc initialization pattern (WorkBook + WorkBookControl)
func TestAmazonWebAppTemplateInitialization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		Environment:    "test",
		Port:           "8080",
		CookieSecret:   "testsecret",
		StorageBackend: "mock",
	}

	router := gin.Default()

	templateDir := "../web/templates/*"
	if _, err := os.Stat("../web/templates"); os.IsNotExist(err) {
		templateDir = "web/templates/*"
	}
	router.LoadHTMLGlob(templateDir)
	router.Static("/static", "../web/static")

	mockStorage := testutils.NewMockStorage()
	sessionMgr := session.NewManager()

	h := &handlers.Handler{
		Config:  cfg,
		Storage: mockStorage,
		Session: sessionMgr,
	}
	h.Auth = handlers.NewAuthHandler(h, nil)
	h.WebApp = handlers.NewWebAppHandler(h)
	h.App = handlers.NewAppHandler(h)

	router.GET("/browser/:param1/:paramCode/:param2", h.App.HandleAmazonWebApp)

	// Request the touchcalc spreadsheet page
	req, _ := http.NewRequest("GET", "/browser/touchcalc/testcode/index.html", nil)
	addUserCookie(req, "testuser")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Expected 200 OK, got %d", w.Code)

	body := w.Body.String()

	// 1. Verify critical scripts are present in correct order
	requiredScripts := []string{
		"socialcalcconstants.js",
		"socialcalc-3.js",
		"socialcalctouch.js",
		"socialcalctableeditor.js",
		"formatnumber2.js",
		"formula1.js",
		"socialcalcpopup.js",
		"socialcalcspreadsheetcontrol.js",
		"socialcalcworkbook.js",
		"socialcalcworkbookcontrol.js",
		"socialcalcimages.js",
	}
	for _, script := range requiredScripts {
		assert.Contains(t, body, script, "Missing required script: %s", script)
	}

	// 2. Verify script order: socialcalcconstants before socialcalc-3
	constIdx := strings.Index(body, "socialcalcconstants.js")
	scIdx := strings.Index(body, "socialcalc-3.js")
	assert.Greater(t, scIdx, constIdx,
		"socialcalcconstants.js must load BEFORE socialcalc-3.js")

	// 3. Verify WorkBook initialization pattern
	assert.Contains(t, body, "new SocialCalc.WorkBook(spreadsheet)",
		"Must use WorkBook initialization")
	assert.Contains(t, body, "workbook.InitializeWorkBook",
		"Must call InitializeWorkBook")
	assert.Contains(t, body, "new SocialCalc.WorkBookControl(",
		"Must create WorkBookControl")
	assert.Contains(t, body, "workbookcontrol.InitializeWorkBookControl()",
		"Must call InitializeWorkBookControl")

	// 4. Verify data loading uses WorkBookControlLoad
	assert.Contains(t, body, "SocialCalc.WorkBookControlLoad(",
		"Must use WorkBookControlLoad to load data")

	// 5. Verify data is in a textarea (not a JS template literal)
	assert.Contains(t, body, `<textarea name="savestr" id="sheetdata"`,
		"Data should be in a hidden textarea")

	// 6. Verify tableeditor div used for the spreadsheet grid
	assert.Contains(t, body, `id="tableeditor"`,
		"Must have tableeditor div")

	// 7. Verify workbookControl div for tab bar
	assert.Contains(t, body, `id="workbookControl"`,
		"Must have workbookControl div for tab UI")

	// 8. Verify body has onresize handler
	assert.Contains(t, body, `onresize="spreadsheet.DoOnResize();"`,
		"Body must have onresize handler")

	// 9. Verify NO custom CSS that breaks SocialCalc layout
	assert.NotContains(t, body, "overflow-x: auto",
		"Should not have overflow-x:auto which breaks SocialCalc mouse coordinates")
	assert.NotContains(t, body, "padding: 20px",
		"Body should not have padding that breaks SocialCalc layout calculations")

	// 10. Verify the spreadsheet.InitializeSpreadsheetControl takes string ID
	assert.Contains(t, body, `spreadsheet.InitializeSpreadsheetControl("tableeditor")`,
		"InitializeSpreadsheetControl should use string ID 'tableeditor'")
}
