package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
)

// Required: if true, any failure to this client causes LINE to receive 5xx (retry).
type Client struct {
	WebhookURL string `json:"webhook_url"`
	Required   bool   `json:"required"`
}

type clientStore struct {
	mu       sync.RWMutex
	clients  []Client
	filePath string
	logger   *slog.Logger
}

func newClientStore(filePath string, logger *slog.Logger) (*clientStore, error) {
	s := &clientStore{filePath: filePath, logger: logger, clients: []Client{}}
	if err := s.load(); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		logger.Info("clients file not found; starting with empty list", "path", filePath)
	}
	return s, nil
}

func (s *clientStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var clients []Client
	if err := json.Unmarshal(data, &clients); err != nil {
		return err
	}
	s.clients = clients
	s.logger.Info("clients loaded", "path", s.filePath, "count", len(clients))
	return nil
}

// snapshotAndUnlock: caller must hold s.mu.Lock().
func (s *clientStore) snapshotAndUnlock() []Client {
	snapshot := make([]Client, len(s.clients))
	copy(snapshot, s.clients)
	s.mu.Unlock()
	return snapshot
}

// persistWith: caller must not hold s.mu.
func (s *clientStore) persistWith(clients []Client) error {
	data, err := json.MarshalIndent(clients, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

func (s *clientStore) persist() error {
	s.mu.RLock()
	clients := make([]Client, len(s.clients))
	copy(clients, s.clients)
	s.mu.RUnlock()
	return s.persistWith(clients)
}

func (s *clientStore) List() []Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Client, len(s.clients))
	copy(out, s.clients)
	return out
}

func (s *clientStore) Add(url string, required bool) (updated bool, err error) {
	s.mu.Lock()
	for i := range s.clients {
		if s.clients[i].WebhookURL == url {
			s.clients[i].Required = required
			return true, s.persistWith(s.snapshotAndUnlock())
		}
	}
	s.clients = append(s.clients, Client{WebhookURL: url, Required: required})
	return false, s.persistWith(s.snapshotAndUnlock())
}

func (s *clientStore) Remove(url string) (removed bool, err error) {
	s.mu.Lock()
	for i, c := range s.clients {
		if c.WebhookURL == url {
			s.clients = append(s.clients[:i], s.clients[i+1:]...)
			return true, s.persistWith(s.snapshotAndUnlock())
		}
	}
	s.mu.Unlock()
	return false, nil
}
