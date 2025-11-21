package tests

import (
	"path/filepath"
	"testing"

	"github.com/c4gt/tornado-nginx-go-backend/internal/storage"
)

// TestSQLiteStorage_BasicCRUD is a lightweight smoke test to ensure the
// SQLiteStorage implementation can create, read, update, and delete files
// using the shared Storage interface semantics.
func TestSQLiteStorage_BasicCRUD(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dsn := "file:" + filepath.Join(tmpDir, "touchcalc.db") + "?_pragma=foreign_keys(1)"

	s, err := storage.NewSQLiteStorage(dsn)
	if err != nil {
		t.Fatalf("failed to create SQLite storage: %v", err)
	}

	// Create directory and file
	path := []string{"home", "users", "test@example.com", "testfile"}
	if err := s.CreateDir(path[:len(path)-1]); err != nil {
		t.Fatalf("CreateDir failed: %v", err)
	}

	const initialData = "hello world"
	if err := s.CreateFile(path, initialData); err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	item, err := s.GetFile(path)
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}
	if item == nil || item.Data != initialData {
		t.Fatalf("GetFile returned unexpected data: %+v", item)
	}

	// Update file
	const updatedData = "updated data"
	if err := s.UpdateFile(path, updatedData); err != nil {
		t.Fatalf("UpdateFile failed: %v", err)
	}

	item, err = s.GetFile(path)
	if err != nil {
		t.Fatalf("GetFile after update failed: %v", err)
	}
	if item.Data != updatedData {
		t.Fatalf("expected updated data %q, got %q", updatedData, item.Data)
	}

	// Delete file
	if err := s.DeleteFile(path); err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	if _, err := s.GetFile(path); err == nil {
		t.Fatalf("expected error after DeleteFile, got nil")
	}
}


