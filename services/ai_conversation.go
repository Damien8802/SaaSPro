package services

import (
    "sync"
    "time"
)

type ConversationState struct {
    Step        string            // greeting, ask_service, ask_design, ask_deadline, ask_contact, done
    UserName    string
    UserContact string
    Service     string
    Design      string
    Deadline    string
    Price       float64
    Services    []string
    LastUpdate  time.Time
}

type ConversationManager struct {
    conversations map[string]*ConversationState
    mu            sync.RWMutex
}

var convManager *ConversationManager
var once sync.Once

func GetConversationManager() *ConversationManager {
    once.Do(func() {
        convManager = &ConversationManager{
            conversations: make(map[string]*ConversationState),
        }
    })
    return convManager
}

func (m *ConversationManager) GetState(sessionID string) *ConversationState {
    m.mu.RLock()
    defer m.mu.RUnlock()
    if state, exists := m.conversations[sessionID]; exists {
        return state
    }
    return nil
}

func (m *ConversationManager) SetState(sessionID string, state *ConversationState) {
    m.mu.Lock()
    defer m.mu.Unlock()
    state.LastUpdate = time.Now()
    m.conversations[sessionID] = state
}

func (m *ConversationManager) ClearState(sessionID string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    delete(m.conversations, sessionID)
}

func (m *ConversationManager) CleanupOldSessions(maxAge time.Duration) {
    m.mu.Lock()
    defer m.mu.Unlock()
    now := time.Now()
    for id, state := range m.conversations {
        if now.Sub(state.LastUpdate) > maxAge {
            delete(m.conversations, id)
        }
    }
}
