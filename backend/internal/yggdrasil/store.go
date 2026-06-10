package yggdrasil

import (
	"sync"
	"time"
)

const (
	sessionTTL = 15 * time.Minute
	joinTTL    = 60 * time.Second
)

// Session — выданная лаунчеру игровая сессия (Minecraft accessToken).
//
// Verified выставляется только после античит-handshake (confirm от агентов). join
// пускает лишь Verified-сессии — это рычаг принуждения: без агентов сервер не пустит
// игрока. Nonce связывает сессию с launch-token, выданным на handshake/init.
type Session struct {
	AccessToken string
	ClientToken string
	UUID        string
	Name        string
	Nonce       string
	Verified    bool
	expiresAt   time.Time
}

// JoinRecord — факт того, что клиент вызвал /join с валидным accessToken.
// На него опирается hasJoined: нет записи — игрок не из лаунчера, доступ закрыт.
type JoinRecord struct {
	UUID      string
	Name      string
	IP        string
	expiresAt time.Time
}

// Store держит активные сессии и join-записи в памяти с TTL.
type Store struct {
	mu       sync.Mutex
	sessions map[string]Session    // accessToken -> session
	joins    map[string]JoinRecord // serverId -> join
	nonces   map[string]string     // nonce -> accessToken (для confirm от агентов)
}

func NewStore() *Store {
	s := &Store{
		sessions: make(map[string]Session),
		joins:    make(map[string]JoinRecord),
		nonces:   make(map[string]string),
	}
	go s.collectGarbage()
	return s
}

func (s *Store) PutSession(sess Session) {
	sess.expiresAt = time.Now().Add(sessionTTL)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.AccessToken] = sess
	if sess.Nonce != "" {
		s.nonces[sess.Nonce] = sess.AccessToken
	}
}

// MarkVerifiedByNonce помечает сессию, связанную с nonce, как прошедшую античит.
// Nonce одноразовый: после успеха он удаляется (анти-replay confirm). Возвращает
// false, если nonce неизвестен/использован или сессия истекла.
func (s *Store) MarkVerifiedByNonce(nonce string) bool {
	if nonce == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	token, ok := s.nonces[nonce]
	if !ok {
		return false
	}
	delete(s.nonces, nonce)
	sess, ok := s.sessions[token]
	if !ok || time.Now().After(sess.expiresAt) {
		return false
	}
	sess.Verified = true
	s.sessions[token] = sess
	return true
}

func (s *Store) Session(accessToken string) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[accessToken]
	if !ok || time.Now().After(sess.expiresAt) {
		return Session{}, false
	}
	return sess, true
}

// ReplaceToken используется при /authserver/refresh: старый accessToken
// заменяется новым, сессия сохраняется.
func (s *Store) ReplaceToken(oldToken, newToken string) (Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[oldToken]
	if !ok || time.Now().After(sess.expiresAt) {
		return Session{}, false
	}
	delete(s.sessions, oldToken)
	sess.AccessToken = newToken
	sess.expiresAt = time.Now().Add(sessionTTL)
	s.sessions[newToken] = sess
	// Nonce-индекс должен указывать на новый токен (Verified сохраняется в sess).
	if sess.Nonce != "" {
		s.nonces[sess.Nonce] = newToken
	}
	return sess, true
}

// InvalidateByNonce гасит игровую сессию, связанную с nonce (kick от античита):
// предотвращает повторный вход с тем же токеном. Возвращает true, если сессия была.
func (s *Store) InvalidateByNonce(nonce string) bool {
	if nonce == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	token, ok := s.nonces[nonce]
	if !ok {
		return false
	}
	delete(s.nonces, nonce)
	if _, ok := s.sessions[token]; ok {
		delete(s.sessions, token)
		return true
	}
	return false
}

func (s *Store) Invalidate(accessToken string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[accessToken]; ok && sess.Nonce != "" {
		delete(s.nonces, sess.Nonce)
	}
	delete(s.sessions, accessToken)
}

// TouchSession продлевает срок жизни сессии (sliding TTL): пока игрок активно
// переподключается, токен жив; после выхода из игры лаунчер его гасит.
func (s *Store) TouchSession(accessToken string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[accessToken]; ok {
		sess.expiresAt = time.Now().Add(sessionTTL)
		s.sessions[accessToken] = sess
	}
}

func (s *Store) PutJoin(serverID string, record JoinRecord) {
	record.expiresAt = time.Now().Add(joinTTL)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.joins[serverID] = record
}

// ConsumeJoin возвращает join-запись и сразу удаляет её — один join проверяется
// ровно один раз (анти-replay для hasJoined).
func (s *Store) ConsumeJoin(serverID string) (JoinRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.joins[serverID]
	delete(s.joins, serverID)
	if !ok || time.Now().After(record.expiresAt) {
		return JoinRecord{}, false
	}
	return record, true
}

func (s *Store) collectGarbage() {
	for range time.Tick(time.Minute) {
		now := time.Now()
		s.mu.Lock()
		for token, sess := range s.sessions {
			if now.After(sess.expiresAt) {
				delete(s.sessions, token)
				if sess.Nonce != "" {
					delete(s.nonces, sess.Nonce)
				}
			}
		}
		for serverID, record := range s.joins {
			if now.After(record.expiresAt) {
				delete(s.joins, serverID)
			}
		}
		s.mu.Unlock()
	}
}
