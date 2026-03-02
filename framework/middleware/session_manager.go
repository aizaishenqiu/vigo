package middleware

import (
	"sync"
	"time"
)

// SessionInfo 会话信息
type SessionInfo struct {
	UserID     uint
	Username   string
	Role       string
	LastAccess time.Time
	CreatedAt  time.Time
	ExpireTime time.Duration
	mu         sync.RWMutex
}

// IsActive 检查会话是否仍然有效
func (s *SessionInfo) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// 检查是否超过滑动过期时间
	return time.Since(s.LastAccess) < s.ExpireTime
}

// Touch 更新最后访问时间（滑动过期）
func (s *SessionInfo) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastAccess = time.Now()
}

// GetUserInfo 获取用户信息
func (s *SessionInfo) GetUserInfo() (uint, string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.UserID, s.Username, s.Role
}

// SessionManager 会话管理器
type SessionManager struct {
	sessions map[string]*SessionInfo
	mu       sync.RWMutex
	cleanupInterval time.Duration
}

// NewSessionManager 创建新的会话管理器
func NewSessionManager(cleanupInterval time.Duration) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*SessionInfo),
		cleanupInterval: cleanupInterval,
	}
	
	// 启动定期清理过期会话的协程
	go sm.startCleanup()
	
	return sm
}

// CreateSession 创建新会话
func (sm *SessionManager) CreateSession(sessionID string, userID uint, username, role string, expireTime time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.sessions[sessionID] = &SessionInfo{
		UserID:     userID,
		Username:   username,
		Role:       role,
		LastAccess: time.Now(),
		CreatedAt:  time.Now(),
		ExpireTime: expireTime,
	}
}

// GetSession 获取会话
func (sm *SessionManager) GetSession(sessionID string) (*SessionInfo, bool) {
	sm.mu.RLock()
	session, exists := sm.sessions[sessionID]
	sm.mu.RUnlock()
	
	if exists && session.IsActive() {
		// 更新最后访问时间（滑动过期）
		session.Touch()
		return session, true
	}
	
	// 如果会话不存在或已过期，删除它
	if exists {
		sm.DeleteSession(sessionID)
	}
	
	return nil, false
}

// DeleteSession 删除会话
func (sm *SessionManager) DeleteSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	delete(sm.sessions, sessionID)
}

// ClearExpiredSessions 清理所有过期会话
func (sm *SessionManager) ClearExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	for sessionID, session := range sm.sessions {
		if !session.IsActive() {
			delete(sm.sessions, sessionID)
		}
	}
}

// startCleanup 启动清理协程
func (sm *SessionManager) startCleanup() {
	ticker := time.NewTicker(sm.cleanupInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		sm.ClearExpiredSessions()
	}
}

// 全局会话管理器
var globalSessionManager *SessionManager

// InitSessionManager 初始化全局会话管理器
func InitSessionManager() {
	globalSessionManager = NewSessionManager(5 * time.Minute) // 每5分钟清理一次
}

// GetSessionManager 获取全局会话管理器
func GetSessionManager() *SessionManager {
	if globalSessionManager == nil {
		InitSessionManager()
	}
	return globalSessionManager
}