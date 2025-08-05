package session

import (
	"encoding/json"
	"time"

	"github.com/patrickmn/go-cache"
)

type Manager struct {
	cache *cache.Cache
}

type Session struct {
	ID   string                 `json:"id"`
	Data map[string]interface{} `json:"data"`
}

func NewManager() *Manager {
	// Cache with 30 minute default expiration and 10 minute cleanup interval
	c := cache.New(30*time.Minute, 10*time.Minute)
	
	return &Manager{
		cache: c,
	}
}

func NewSession(id string) *Session {
	return &Session{
		ID:   id,
		Data: make(map[string]interface{}),
	}
}

func (m *Manager) Get(sessionID string) (*Session, bool) {
	data, found := m.cache.Get(sessionID)
	if !found {
		return nil, false
	}

	session, ok := data.(*Session)
	if !ok {
		return nil, false
	}

	return session, true
}

func (m *Manager) Set(sessionID string, session *Session) {
	m.cache.Set(sessionID, session, cache.DefaultExpiration)
}

func (m *Manager) Delete(sessionID string) {
	m.cache.Delete(sessionID)
}

func (m *Manager) GetOrCreate(sessionID string) *Session {
	session, found := m.Get(sessionID)
	if !found {
		session = NewSession(sessionID)
		m.Set(sessionID, session)
	}
	return session
}

func (s *Session) SetValue(key string, value interface{}) {
	s.Data[key] = value
}

func (s *Session) GetValue(key string) (interface{}, bool) {
	value, exists := s.Data[key]
	return value, exists
}

func (s *Session) GetString(key string) (string, bool) {
	value, exists := s.GetValue(key)
	if !exists {
		return "", false
	}
	
	str, ok := value.(string)
	return str, ok
}

func (s *Session) GetInt(key string) (int, bool) {
	value, exists := s.GetValue(key)
	if !exists {
		return 0, false
	}
	
	// Handle both int and float64 (common from JSON unmarshaling)
	switch v := value.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func (s *Session) RemoveValue(key string) {
	delete(s.Data, key)
}

func (s *Session) ToJSON() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func SessionFromJSON(data string) (*Session, error) {
	var session Session
	err := json.Unmarshal([]byte(data), &session)
	if err != nil {
		return nil, err
	}
	
	// Initialize the Data map if it's nil
	if session.Data == nil {
		session.Data = make(map[string]interface{})
	}
	
	return &session, nil
}