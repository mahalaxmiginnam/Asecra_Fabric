package idempotency

import (
	"sync"
	"time"
)

type Status string

const (
	StatusInFlight Status = "in-flight"
	StatusComplete Status = "complete"
)

type Response struct {
	StatusCode int
	Header     map[string][]string
	Body       []byte
}

type Entry struct {
	Status    Status
	Response  *Response
	CreatedAt time.Time
}

type Store struct {
	mu sync.Mutex

	entries map[string]*Entry

	ttl time.Duration
}

func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}

	return &Store{
		entries: make(map[string]*Entry),
		ttl:     ttl,
	}
}

func (s *Store) Get(
	key string,
) (*Entry, bool) {

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.entries[key]

	if !exists {
		return nil, false
	}

	if time.Since(entry.CreatedAt) >= s.ttl {
		delete(s.entries, key)
		return nil, false
	}

	return cloneEntry(entry), true
}

func (s *Store) Create(
	key string,
) bool {

	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, exists := s.entries[key]; exists {

		if time.Since(entry.CreatedAt) < s.ttl {
			return false
		}

		delete(s.entries, key)
	}

	s.entries[key] = &Entry{
		Status:    StatusInFlight,
		CreatedAt: time.Now(),
	}

	return true
}

func (s *Store) Complete(
	key string,
	response Response,
) {

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.entries[key]

	if !exists {
		return
	}

	entry.Status = StatusComplete
	entry.Response = &Response{
		StatusCode: response.StatusCode,
		Header:     cloneHeader(response.Header),
		Body:       append([]byte(nil), response.Body...),
	}
}

func cloneEntry(
	entry *Entry,
) *Entry {

	result := &Entry{
		Status:    entry.Status,
		CreatedAt: entry.CreatedAt,
	}

	if entry.Response != nil {
		result.Response = &Response{
			StatusCode: entry.Response.StatusCode,
			Header:     cloneHeader(entry.Response.Header),
			Body: append(
				[]byte(nil),
				entry.Response.Body...,
			),
		}
	}

	return result
}

func cloneHeader(
	header map[string][]string,
) map[string][]string {

	if header == nil {
		return nil
	}

	result := make(map[string][]string, len(header))

	for key, values := range header {
		result[key] = append(
			[]string(nil),
			values...,
		)
	}

	return result
}
