package idempotency

import (
	"errors"
	"time"
)

var (
	ErrInFlight = errors.New("idempotent request already in progress")
)

type Decision int

const (
	Execute Decision = iota
	Replay
	Reject
)

type Controller struct {
	Store *Store
}

func NewController(store *Store) *Controller {
	return &Controller{
		Store: store,
	}
}

func (c *Controller) Begin(
	key string,
) (Decision, *Response, error) {

	if key == "" {
		return Execute, nil, nil
	}

	entry, exists := c.Store.Get(key)

	if exists {

		switch entry.Status {

		case StatusComplete:

			if entry.Response == nil {
				return Reject, nil, errors.New(
					"completed idempotency entry has no response",
				)
			}

			return Replay, entry.Response, nil

		case StatusInFlight:
			return Reject, nil, ErrInFlight
		}
	}

	if !c.Store.Create(key) {
		return Reject, nil, ErrInFlight
	}

	return Execute, nil, nil
}

func (c *Controller) Complete(
	key string,
	response Response,
) {
	if key == "" {
		return
	}

	c.Store.Complete(key, response)
}

func DefaultTTL() time.Duration {
	return 10 * time.Minute
}
