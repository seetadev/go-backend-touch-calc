package models

import (
	"encoding/json"
)

// File represents a file in the storage system
type File struct {
	FName string      `json:"fname"`
	Data  interface{} `json:"data"`
}

// Directory represents a directory in the storage system
type Directory struct {
	FName string  `json:"fname"`
	Files []*File `json:"files"`
}

// StorageItem represents a generic storage item (file or directory)
type StorageItem struct {
	Path []string    `json:"path"`
	Type string      `json:"type"` // "file" or "dir"
	Data interface{} `json:"data"`
}

func NewFile(name string, data interface{}) *File {
	return &File{
		FName: name,
		Data:  data,
	}
}

func NewDirectory(name string, fileList []string) *Directory {
	files := make([]*File, len(fileList))
	for i, fileName := range fileList {
		files[i] = NewFile(fileName, "")
	}
	
	return &Directory{
		FName: name,
		Files: files,
	}
}

func NewStorageItem(path []string, itemType string, data interface{}) *StorageItem {
	return &StorageItem{
		Path: path,
		Type: itemType,
		Data: data,
	}
}

func (si *StorageItem) ToJSON() (string, error) {
	data, err := json.Marshal(si)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func StorageItemFromJSON(data string) (*StorageItem, error) {
	var item StorageItem
	err := json.Unmarshal([]byte(data), &item)
	if err != nil {
		return nil, err
	}
	return &item, nil
}