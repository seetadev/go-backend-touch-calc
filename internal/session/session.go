package session

import (
    "encoding/json"
    "sync"
    "time"
)

type Session struct {
    ID       string                 `json:"id"`
    Data     map[string]interface{} `json:"data"`
    LastUsed time.Time             `json:"last_used"`
}

type Manager struct {
    sessions map[string]*Session
    mutex    sync.RWMutex
}

func NewManager() *Manager {
    manager := &Manager{
        sessions: make(map[string]*Session),
    }
    
    // Start cleanup goroutine
    go manager.cleanup()
    
    return manager
}

func NewSession(id string) *Session {
    return &Session{
        ID:       id,
        Data:     make(map[string]interface{}),
        LastUsed: time.Now(),
    }
}

func (m *Manager) Get(sessionID string) (*Session, bool) {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    
    session, exists := m.sessions[sessionID]
    if exists {
        session.LastUsed = time.Now()
    }
    return session, exists
}

func (m *Manager) Set(sessionID string, session *Session) {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    session.LastUsed = time.Now()
    m.sessions[sessionID] = session
}

func (m *Manager) Delete(sessionID string) {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    delete(m.sessions, sessionID)
}

func (m *Manager) GetOrCreate(sessionID string) *Session {
    session, found := m.Get(sessionID)
    if !found {
        session = NewSession(sessionID)
        m.Set(sessionID, session)
    }
    return session
}

func (m *Manager) cleanup() {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            m.mutex.Lock()
            now := time.Now()
            for id, session := range m.sessions {
                if now.Sub(session.LastUsed) > 24*time.Hour {
                    delete(m.sessions, id)
                }
            }
            m.mutex.Unlock()
        }
    }
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
