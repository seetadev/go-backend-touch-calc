package models

import (
	"encoding/json"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Email       string    `json:"email"`
	PWHash      string    `json:"pwhash"`
	Confirmed   bool      `json:"confirmed"`
	LastLogin   time.Time `json:"lastlogin"`
	CreatedOn   time.Time `json:"createdon"`
	Dongle      string    `json:"dongle"`
}

func NewUser(email, password string) (*User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return &User{
		Email:     email,
		PWHash:    string(hashedPassword),
		Confirmed: true,
		CreatedOn: time.Now(),
		LastLogin: time.Time{},
		Dongle:    "",
	}, nil
}

func (u *User) Authenticate(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PWHash), []byte(password))
	return err == nil
}

func (u *User) SetPassword(newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PWHash = string(hashedPassword)
	return nil
}

func (u *User) ToJSON() (string, error) {
	data, err := json.Marshal(u)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func UserFromJSON(data string) (*User, error) {
	var user User
	err := json.Unmarshal([]byte(data), &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (u *User) SetConfirmed() {
	u.Confirmed = true
}

func (u *User) GetConfirmed() bool {
	return u.Confirmed
}

func (u *User) SetDongle(dongle string) {
	u.Dongle = dongle
}

func (u *User) GetDongle() string {
	return u.Dongle
}