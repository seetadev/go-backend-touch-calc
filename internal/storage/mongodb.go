package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/c4gt/tornado-nginx-go-backend/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoStorage struct {
	client   *mongo.Client
	database *mongo.Database
}

type MongoItem struct {
	ID   string      `bson:"_id"`
	Path string      `bson:"path"`
	Type string      `bson:"type"`
	Data interface{} `bson:"data"`
}

func NewMongoStorage(uri, dbName string) (*MongoStorage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Test connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	database := client.Database(dbName)

	return &MongoStorage{
		client:   client,
		database: database,
	}, nil
}

func (m *MongoStorage) pathToString(path []string) string {
	return strings.Join(path, "/")
}

func (m *MongoStorage) getCollection() *mongo.Collection {
	return m.database.Collection("storage_items")
}

// getSpreadsheetCollection returns the MongoDB collection used to store
// spreadsheet documents.
func (m *MongoStorage) getSpreadsheetCollection() *mongo.Collection {
	return m.database.Collection("spreadsheets")
}

func (m *MongoStorage) PutItem(path string, data string, bucket ...string) error {
	collection := m.getCollection()
	ctx := context.Background()

	item := MongoItem{
		ID:   path,
		Path: path,
		Data: data,
	}

	opts := options.Replace().SetUpsert(true)
	_, err := collection.ReplaceOne(ctx, bson.M{"_id": path}, item, opts)
	return err
}

func (m *MongoStorage) GetItem(path string, bucket ...string) (string, error) {
	collection := m.getCollection()
	ctx := context.Background()

	var item MongoItem
	err := collection.FindOne(ctx, bson.M{"_id": path}).Decode(&item)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", ErrNotFound
		}
		return "", err
	}

	if dataStr, ok := item.Data.(string); ok {
		return dataStr, nil
	}

	dataBytes, err := json.Marshal(item.Data)
	if err != nil {
		return "", err
	}
	return string(dataBytes), nil
}

func (m *MongoStorage) ExistsItem(path string, bucket ...string) (bool, error) {
	collection := m.getCollection()
	ctx := context.Background()

	count, err := collection.CountDocuments(ctx, bson.M{"_id": path})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (m *MongoStorage) DeleteItem(path string, bucket ...string) error {
	collection := m.getCollection()
	ctx := context.Background()

	_, err := collection.DeleteOne(ctx, bson.M{"_id": path})
	return err
}

func (m *MongoStorage) ensureParentDirectories(path []string) error {
	if len(path) == 0 {
		return nil
	}

	// Check if directory exists
	spath := m.pathToString(path)
	exists, err := m.ExistsItem(spath)
	if err != nil {
		return err
	}
	if exists {
		return nil // Directory already exists
	}

	// Create parent directories recursively
	if len(path) > 1 {
		err = m.ensureParentDirectories(path[:len(path)-1])
		if err != nil {
			return err
		}
	}

	// Create this directory
	dirData := models.NewStorageItem(path, "dir", []string{})
	dataJSON, err := dirData.ToJSON()
	if err != nil {
		return err
	}

	return m.PutItem(spath, dataJSON)
}

func (m *MongoStorage) CreateDir(path []string) error {
	if len(path) == 0 {
		return fmt.Errorf("invalid path: cannot be empty")
	}

	spath := m.pathToString(path)
	exists, err := m.ExistsItem(spath)
	if err != nil {
		return err
	}
	if exists {
		return nil // Don't error if directory already exists
	}

	// Create parent directories recursively
	if len(path) > 1 {
		err = m.ensureParentDirectories(path[:len(path)-1])
		if err != nil {
			return err
		}
	}

	dirData := models.NewStorageItem(path, "dir", []string{})
	dataJSON, err := dirData.ToJSON()
	if err != nil {
		return err
	}

	return m.PutItem(spath, dataJSON)
}

func (m *MongoStorage) DeleteDir(path []string) error {
	collection := m.getCollection()
	ctx := context.Background()

	spath := m.pathToString(path)
	_, err := collection.DeleteMany(ctx, bson.M{
		"path": bson.M{"$regex": "^" + spath},
	})
	return err
}

func (m *MongoStorage) GetFile(path []string) (*models.StorageItem, error) {
	spath := m.pathToString(path)
	data, err := m.GetItem(spath)
	if err != nil {
		return nil, err
	}

	return models.StorageItemFromJSON(data)
}

func (m *MongoStorage) CreateFile(path []string, data string) error {
	if len(path) == 0 {
		return fmt.Errorf("invalid path: cannot be empty")
	}

	spath := m.pathToString(path)
	exists, err := m.ExistsItem(spath)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("file already exists")
	}

	// Create parent directories recursively if they don't exist
	if len(path) > 1 {
		err = m.ensureParentDirectories(path[:len(path)-1])
		if err != nil {
			return fmt.Errorf("failed to create parent directories: %w", err)
		}
	}

	fileData := models.NewStorageItem(path, "file", data)
	dataJSON, err := fileData.ToJSON()
	if err != nil {
		return err
	}

	err = m.PutItem(spath, dataJSON)
	if err != nil {
		return err
	}

	// Update parent directory if it exists
	if len(path) > 1 {
		parentPath := path[:len(path)-1]
		fileName := path[len(path)-1]

		parentItem, err := m.GetFile(parentPath)
		if err == nil { // Parent exists
			var filesList []string
			if parentData, ok := parentItem.Data.([]interface{}); ok {
				for _, item := range parentData {
					if str, ok := item.(string); ok {
						filesList = append(filesList, str)
					}
				}
			}

			// Add filename if not already present
			found := false
			for _, existing := range filesList {
				if existing == fileName {
					found = true
					break
				}
			}
			if !found {
				filesList = append(filesList, fileName)
			}

			parentItem.Data = filesList
			parentJSON, err := parentItem.ToJSON()
			if err != nil {
				return err
			}

			parentSPath := m.pathToString(parentPath)
			err = m.PutItem(parentSPath, parentJSON)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *MongoStorage) UpdateFile(path []string, data string) error {
	fileItem, err := m.GetFile(path)
	if err != nil {
		return err
	}
	if fileItem.Type != "file" {
		return fmt.Errorf("path is not a file")
	}

	fileItem.Data = data
	dataJSON, err := fileItem.ToJSON()
	if err != nil {
		return err
	}

	spath := m.pathToString(path)
	return m.PutItem(spath, dataJSON)
}

func (m *MongoStorage) DeleteFile(path []string) error {
	fileItem, err := m.GetFile(path)
	if err != nil {
		return err
	}
	if fileItem.Type != "file" {
		return fmt.Errorf("path is not a file")
	}

	if len(path) > 1 {
		parentPath := path[:len(path)-1]
		parentItem, err := m.GetFile(parentPath)
		if err != nil {
			return err
		}

		fileName := path[len(path)-1]

		var filesList []string
		if parentData, ok := parentItem.Data.([]interface{}); ok {
			for _, item := range parentData {
				if str, ok := item.(string); ok && str != fileName {
					filesList = append(filesList, str)
				}
			}
		}

		parentItem.Data = filesList

		parentJSON, err := parentItem.ToJSON()
		if err != nil {
			return err
		}

		parentSPath := m.pathToString(parentPath)
		err = m.PutItem(parentSPath, parentJSON)
		if err != nil {
			return err
		}
	}

	spath := m.pathToString(path)
	return m.DeleteItem(spath)
}

// SaveSpreadsheet inserts or updates a spreadsheet document for the given
// user/app/file combination. A per-operation timeout is applied to avoid
// hanging connections.
func (m *MongoStorage) SaveSpreadsheet(ctx context.Context, doc *models.SpreadsheetDocument) error {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	coll := m.getSpreadsheetCollection()

	now := time.Now().UTC()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now

	filter := bson.M{
		"user":    doc.User,
		"appname": doc.AppName,
		"fname":   doc.FileName,
	}

	update := bson.M{
		"$set": doc,
	}

	opts := options.Update().SetUpsert(true)
	if _, err := coll.UpdateOne(cctx, filter, update, opts); err != nil {
		return fmt.Errorf("failed to save spreadsheet %s/%s for user %s: %w",
			doc.AppName, doc.FileName, doc.User, err)
	}

	return nil
}

// LoadSpreadsheet loads a single spreadsheet document by user, app name
// and file name. Returns ErrNotFound if no document exists.
func (m *MongoStorage) LoadSpreadsheet(ctx context.Context, appName, fileName, user string) (*models.SpreadsheetDocument, error) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	coll := m.getSpreadsheetCollection()

	filter := bson.M{
		"user":    user,
		"appname": appName,
		"fname":   fileName,
	}

	var doc models.SpreadsheetDocument
	if err := coll.FindOne(cctx, filter).Decode(&doc); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to load spreadsheet %s/%s for user %s: %w",
			appName, fileName, user, err)
	}

	return &doc, nil
}

// ListSpreadsheets returns all spreadsheet documents for a given user
// and application. It applies a per-operation timeout.
func (m *MongoStorage) ListSpreadsheets(ctx context.Context, appName, user string) ([]*models.SpreadsheetDocument, error) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	coll := m.getSpreadsheetCollection()

	filter := bson.M{
		"user":    user,
		"appname": appName,
	}

	cursor, err := coll.Find(cctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list spreadsheets for user %s app %s: %w",
			user, appName, err)
	}
	defer cursor.Close(cctx)

	var results []*models.SpreadsheetDocument
	for cursor.Next(cctx) {
		var doc models.SpreadsheetDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("failed to decode spreadsheet document: %w", err)
		}
		results = append(results, &doc)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("error iterating spreadsheets cursor: %w", err)
	}

	return results, nil
}

// DeleteSpreadsheet deletes a spreadsheet document for the given user,
// app name and file name. If the document does not exist, it returns
// ErrNotFound.
func (m *MongoStorage) DeleteSpreadsheet(ctx context.Context, appName, fileName, user string) error {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	coll := m.getSpreadsheetCollection()

	filter := bson.M{
		"user":    user,
		"appname": appName,
		"fname":   fileName,
	}

	res, err := coll.DeleteOne(cctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete spreadsheet %s/%s for user %s: %w",
			appName, fileName, user, err)
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}

	return nil
}
