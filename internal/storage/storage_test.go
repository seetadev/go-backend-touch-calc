package storage_test

import (
	"testing"

	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
	"github.com/c4gt/tornado-nginx-go-backend/tests/testutils"
	"github.com/stretchr/testify/assert"
)

func TestCreateGetUpdateDeleteFile(t *testing.T) {
	store := testutils.NewMockStorage()

	path := []string{"home", "user1", "securestore", "app", "file1.txt"}

	err := store.CreateDir([]string{"home", "user1", "securestore", "app"})
	assert.NoError(t, err)

	fileData := models.NewStorageItem(path, "file", "test content")
	dataJSON, _ := fileData.ToJSON()
	err = store.CreateFile(path, dataJSON)
	assert.NoError(t, err)

	item, err := store.GetFile(path)
	assert.NoError(t, err)
	assert.Equal(t, "test content", item.Data)

	err = store.UpdateFile(path, `{"data":"updated"}`)
	assert.NoError(t, err)

	err = store.DeleteFile(path)
	assert.NoError(t, err)
}
