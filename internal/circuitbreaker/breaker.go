package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrOpen = errors.New("circuit breaker is open")
	ErrBusy = errors.New("circuit breaker half-open probe already in progress")
)

type State int

const (
	Closed State = iota
	Open
	HalfOpen
)

type Breaker struct {
	mu sync.Mutex

	state State

	failures int

	failureThreshold int

	resetTimeout time.Duration

	openedAt time.Time

	probeInProgress bool
}

func New(
	failureThreshold int,
	resetTimeout time.Duration,
) *Breaker {

	if failureThreshold < 1 {
		failureThreshold = 1
	}

	if resetTimeout <= 0 {
		resetTimeout = time.Second
	}

	return &Breaker{
		state:            Closed,
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
	}
}

func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.transitionIfReady()

	return b.state
}

func (b *Breaker) Allow() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.transitionIfReady()

	switch b.state {

	case Closed:
		return nil

	case Open:
		return ErrOpen

	case HalfOpen:

		if b.probeInProgress {
			return ErrBusy
		}

		b.probeInProgress = true

		return nil

	default:
		return ErrOpen
	}
}

func (b *Breaker) Success() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures = 0
	b.state = Closed
	b.probeInProgress = false
}

func (b *Breaker) Failure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == HalfOpen {
		b.state = Open
		b.openedAt = time.Now()
		b.probeInProgress = false
		return
	}

	if b.state != Closed {
		return
	}

	b.failures++

	if b.failures >= b.failureThreshold {
		b.state = Open
		b.openedAt = time.Now()
	}
}

func (b *Breaker) transitionIfReady() {

	if b.state != Open {
		return
	}

	if time.Since(b.openedAt) >= b.resetTimeout {
		b.state = HalfOpen
		b.probeInProgress = false
	}
}
