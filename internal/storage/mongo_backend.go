package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
)

// NOTE: This adapter allows MongoStorage (which uses a richer
// SpreadsheetDocument API with context) to satisfy the simpler
// StorageBackend interface without removing or changing any existing code.

// mongoSpreadsheetBackend wraps a MongoStorage to implement StorageBackend.
type mongoSpreadsheetBackend struct {
	store   *MongoStorage
	appName string
}

// NewMongoSpreadsheetBackend creates a StorageBackend implementation backed
// by MongoStorage. The appName is used for namespacing spreadsheets; if
// empty, "touchcalc" is used by default.
func NewMongoSpreadsheetBackend(store *MongoStorage, appName string) StorageBackend {
	if appName == "" {
		appName = "touchcalc"
	}
	return &mongoSpreadsheetBackend{
		store:   store,
		appName: appName,
	}
}

func (m *mongoSpreadsheetBackend) SaveSpreadsheet(userID, fileName string, data []byte) error {
	if m.store == nil {
		return fmt.Errorf("mongodb storage is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	doc := &models.SpreadsheetDocument{
		User:     userID,
		AppName:  m.appName,
		FileName: fileName,
		Data:     string(data),
	}

	return m.store.SaveSpreadsheet(ctx, doc)
}

func (m *mongoSpreadsheetBackend) LoadSpreadsheet(userID, fileName string) ([]byte, error) {
	if m.store == nil {
		return nil, fmt.Errorf("mongodb storage is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	doc, err := m.store.LoadSpreadsheet(ctx, m.appName, fileName, userID)
	if err != nil {
		return nil, err
	}

	return []byte(doc.Data), nil
}

func (m *mongoSpreadsheetBackend) ListSpreadsheets(userID string) ([]string, error) {
	if m.store == nil {
		return nil, fmt.Errorf("mongodb storage is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	list, err := m.store.ListSpreadsheets(ctx, m.appName, userID)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(list))
	for _, doc := range list {
		if doc != nil {
			names = append(names, doc.FileName)
		}
	}

	return names, nil
}

func (m *mongoSpreadsheetBackend) DeleteSpreadsheet(userID, fileName string) error {
	if m.store == nil {
		return fmt.Errorf("mongodb storage is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return m.store.DeleteSpreadsheet(ctx, m.appName, fileName, userID)
}



