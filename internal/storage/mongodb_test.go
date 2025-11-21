package storage

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
	"github.com/stretchr/testify/require"
)

// setupMongoTest initializes a MongoStorage instance pointing at a test
// database. It requires a running MongoDB instance, either local or via
// docker-compose. If MongoDB is not reachable, the tests are skipped.
func setupMongoTest(t *testing.T) *MongoStorage {
	t.Helper()

	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	dbName := fmt.Sprintf("touchcalc_test_%d", time.Now().UnixNano())

	storage, err := NewMongoStorage(uri, dbName)
	if err != nil {
		t.Skipf("Skipping MongoDB tests; cannot connect to %s: %v", uri, err)
	}

	// Teardown: drop the test database after the test finishes.
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = storage.database.Drop(ctx) // best-effort cleanup
	})

	return storage
}

// TestMongoConnection verifies that NewMongoStorage can connect and ping
// a running MongoDB instance.
func TestMongoConnection(t *testing.T) {
	storage := setupMongoTest(t)
	require.NotNil(t, storage)
	require.NotNil(t, storage.client)
	require.NotNil(t, storage.database)
}

// TestMongoSaveAndLoadSpreadsheet verifies saving and loading a single
// spreadsheet document via the MongoStorage spreadsheet APIs.
func TestMongoSaveAndLoadSpreadsheet(t *testing.T) {
	storage := setupMongoTest(t)

	ctx := context.Background()

	userID := "user1@example.com"
	appName := "touchcalc"
	fileName := "test1.json"
	data := `{"A1":"Hello"}`

	doc := &models.SpreadsheetDocument{
		User:     userID,
		AppName:  appName,
		FileName: fileName,
		Data:     data,
	}

	// Save spreadsheet
	err := storage.SaveSpreadsheet(ctx, doc)
	require.NoError(t, err, "SaveSpreadsheet should not error")

	// Load spreadsheet
	loaded, err := storage.LoadSpreadsheet(ctx, appName, fileName, userID)
	require.NoError(t, err, "LoadSpreadsheet should not error")
	require.NotNil(t, loaded)
	require.Equal(t, userID, loaded.User)
	require.Equal(t, appName, loaded.AppName)
	require.Equal(t, fileName, loaded.FileName)
	require.Equal(t, data, loaded.Data)
	require.False(t, loaded.CreatedAt.IsZero())
	require.False(t, loaded.UpdatedAt.IsZero())
}

// TestMongoListSpreadsheets verifies listing all spreadsheets for a user
// and application.
func TestMongoListSpreadsheets(t *testing.T) {
	storage := setupMongoTest(t)

	ctx := context.Background()

	userID := "user2@example.com"
	appName := "touchcalc"

	// Seed with two spreadsheets
	files := []string{"sheet1.json", "sheet2.json"}
	for _, fname := range files {
		doc := &models.SpreadsheetDocument{
			User:     userID,
			AppName:  appName,
			FileName: fname,
			Data:     fmt.Sprintf(`{"name":"%s"}`, fname),
		}
		err := storage.SaveSpreadsheet(ctx, doc)
		require.NoError(t, err, "SaveSpreadsheet should not error for %s", fname)
	}

	// List spreadsheets
	list, err := storage.ListSpreadsheets(ctx, appName, userID)
	require.NoError(t, err, "ListSpreadsheets should not error")
	require.Len(t, list, 2)

	names := make(map[string]bool)
	for _, doc := range list {
		names[doc.FileName] = true
	}
	require.True(t, names["sheet1.json"])
	require.True(t, names["sheet2.json"])
}

