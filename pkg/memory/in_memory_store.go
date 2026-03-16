package memory

import (
	"context"
	"sync"

	"github.com/luanlima/gaal-lib/pkg/types"
)

const (
	metadataUserIDKey             = "user_id"
	metadataUserIDAlias           = "userID"
	metadataConversationIDKey     = "conversation_id"
	metadataConversationIDAlias   = "conversationID"
	inMemoryDefaultUserScopeKey   = "_"
	inMemoryDefaultConversationID = "_"
)

// InMemoryStore is the reference in-process Store implementation.
//
// It is deterministic within a single process, preserves isolation by
// SessionID, and is not durable across process restarts. When metadata contains
// user and conversation identifiers, the store keeps its internal state grouped
// by that namespace while preserving SessionID as the public lookup key.
type InMemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]inMemorySessionScope
	data     map[string]map[string]map[string]Snapshot
}

// Load returns the persisted conversational snapshot for sessionID.
func (s *InMemoryStore) Load(ctx context.Context, sessionID string) (Snapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	scope, ok := s.sessions[sessionID]
	if !ok {
		return Snapshot{}, nil
	}

	userBucket := s.data[scope.userID]
	if userBucket == nil {
		return Snapshot{}, nil
	}

	conversationBucket := userBucket[scope.conversationID]
	if conversationBucket == nil {
		return Snapshot{}, nil
	}

	snapshot, ok := conversationBucket[sessionID]
	if !ok {
		return Snapshot{}, nil
	}
	return cloneSnapshot(snapshot), nil
}

// Save materializes delta as the latest conversational state for sessionID.
func (s *InMemoryStore) Save(ctx context.Context, sessionID string, delta Delta) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureMaps()

	scope := inMemoryScopeForSession(sessionID, delta.Metadata)
	previousScope, hadPrevious := s.sessions[sessionID]
	if hadPrevious && previousScope != scope {
		s.deleteSessionLocked(previousScope, sessionID)
	}

	userBucket, ok := s.data[scope.userID]
	if !ok {
		userBucket = make(map[string]map[string]Snapshot)
		s.data[scope.userID] = userBucket
	}

	conversationBucket, ok := userBucket[scope.conversationID]
	if !ok {
		conversationBucket = make(map[string]Snapshot)
		userBucket[scope.conversationID] = conversationBucket
	}

	conversationBucket[sessionID] = Snapshot{
		Messages: types.CloneMessages(delta.Messages),
		Records:  cloneRecords(delta.Records),
		Metadata: types.CloneMetadata(delta.Metadata),
	}
	s.sessions[sessionID] = scope

	return nil
}

func (s *InMemoryStore) ensureMaps() {
	if s.sessions == nil {
		s.sessions = make(map[string]inMemorySessionScope)
	}
	if s.data == nil {
		s.data = make(map[string]map[string]map[string]Snapshot)
	}
}

func (s *InMemoryStore) deleteSessionLocked(scope inMemorySessionScope, sessionID string) {
	userBucket := s.data[scope.userID]
	if userBucket == nil {
		return
	}

	conversationBucket := userBucket[scope.conversationID]
	if conversationBucket == nil {
		return
	}

	delete(conversationBucket, sessionID)
	if len(conversationBucket) == 0 {
		delete(userBucket, scope.conversationID)
	}
	if len(userBucket) == 0 {
		delete(s.data, scope.userID)
	}
}

type inMemorySessionScope struct {
	userID         string
	conversationID string
}

func inMemoryScopeForSession(sessionID string, metadata types.Metadata) inMemorySessionScope {
	userID := firstNonEmpty(metadata[metadataUserIDKey], metadata[metadataUserIDAlias], inMemoryDefaultUserScopeKey)
	conversationID := firstNonEmpty(metadata[metadataConversationIDKey], metadata[metadataConversationIDAlias], inMemoryDefaultConversationID)
	return inMemorySessionScope{
		userID:         userID,
		conversationID: conversationID,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
