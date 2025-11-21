package models

import "time"

// SpreadsheetDocument represents a spreadsheet stored in MongoDB.
// It is designed to work both with BSON (MongoDB) and JSON (API).
type SpreadsheetDocument struct {
	ID       string `bson:"_id,omitempty" json:"id"`
	User     string `bson:"user" json:"user"`
	AppName  string `bson:"appname" json:"appname"`
	FileName string `bson:"fname" json:"fname"`
	Data     string `bson:"data" json:"data"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

