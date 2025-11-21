package storage_test

import (
	"testing"

	"github.com/c4gt/tornado-nginx-go-backend/tests/testutils"
	"github.com/stretchr/testify/assert"
)

func TestCreateGetUpdateDeleteFile(t *testing.T) {
	store := testutils.NewMockStorage()

	path := []string{"home", "user1", "securestore", "app", "file1.txt"}

	err := store.CreateDir([]string{"home", "user1", "securestore", "app"})
	assert.NoError(t, err)

	// Create file with plain content; the storage implementation is
	// responsible for wrapping it into a StorageItem.
	const initialContent = "test content"
	err = store.CreateFile(path, initialContent)
	assert.NoError(t, err)

	item, err := store.GetFile(path)
	assert.NoError(t, err)
	assert.Equal(t, initialContent, item.Data)

	err = store.UpdateFile(path, `{"data":"updated"}`)
	assert.NoError(t, err)

	err = store.DeleteFile(path)
	assert.NoError(t, err)
}
