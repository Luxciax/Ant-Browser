package secretstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ant-chrome/backend/internal/fsutil"
	"github.com/google/uuid"
)

const (
	documentVersion = 1
	refPrefix = "secret://"
)

var ErrUnavailable = errors.New("secret protection provider is unavailable")

type Entry struct {
	Label     string `json:"label"`
	Provider  string `json:"provider"`
	Payload   string `json:"payload"`
	UpdatedAt string `json:"updatedAt"`
}

type document struct {
	Version int              `json:"version"`
	Entries map[string]Entry `json:"entries"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func New(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("secret store path is empty")
	}
	return &Store{path: filepath.Clean(path)}, nil
}

func IsRef(value string) bool {
	_, ok := refID(value)
	return ok
}

func refID(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, refPrefix) {
		return "", false
	}
	id := strings.TrimSpace(strings.TrimPrefix(value, refPrefix))
	if id == "" || strings.ContainsAny(id, "/\\") {
		return "", false
	}
	return id, true
}

func refForID(id string) string { return refPrefix + id }

func (s *Store) Get(ref string) (string, error) {
	if s == nil {
		return "", ErrUnavailable
	}
	id, ok := refID(ref)
	if !ok {
		return "", fmt.Errorf("invalid secret ref")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadLocked()
	if err != nil {
		return "", err
	}
	entry, ok := doc.Entries[id]
	if !ok {
		return "", fmt.Errorf("secret ref not found")
	}
	plaintext, err := unprotectValue(entry.Provider, entry.Payload)
	if err != nil {
		return "", err
	}
	defer zeroBytes(plaintext)
	return string(plaintext), nil
}

func (s *Store) Set(ref, label, value string) (string, error) {
	if s == nil {
		return "", ErrUnavailable
	}
	if value == "" {
		return "", fmt.Errorf("secret value is empty")
	}
	id, ok := refID(ref)
	if !ok {
		id = uuid.NewString()
	}
	provider, payload, err := protectValue([]byte(value))
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadLocked()
	if err != nil {
		return "", err
	}
	doc.Entries[id] = Entry{
		Label: strings.TrimSpace(label),
		Provider: provider,
		Payload: payload,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := s.saveLocked(doc); err != nil {
		return "", err
	}
	return refForID(id), nil
}

func (s *Store) Delete(ref string) error {
	if s == nil {
		return ErrUnavailable
	}
	id, ok := refID(ref)
	if !ok {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.loadLocked()
	if err != nil {
		return err
	}
	if _, exists := doc.Entries[id]; !exists {
		return nil
	}
	delete(doc.Entries, id)
	return s.saveLocked(doc)
}

func (s *Store) loadLocked() (document, error) {
	doc := document{Version: documentVersion, Entries: map[string]Entry{}}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return doc, nil
		}
		return document{}, fmt.Errorf("read secret store: %w", err)
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return document{}, fmt.Errorf("decode secret store: %w", err)
	}
	if doc.Version != documentVersion {
		return document{}, fmt.Errorf("unsupported secret store version")
	}
	if doc.Entries == nil {
		doc.Entries = map[string]Entry{}
	}
	return doc, nil
}

func (s *Store) saveLocked(doc document) error {
	doc.Version = documentVersion
	if doc.Entries == nil {
		doc.Entries = map[string]Entry{}
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode secret store: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create secret store directory: %w", err)
	}
	if err := fsutil.AtomicWriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write secret store: %w", err)
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		return fmt.Errorf("protect secret store permissions: %w", err)
	}
	return nil
}

func zeroBytes(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
