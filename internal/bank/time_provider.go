package bank

import "time"

type TimeProvider interface {
	NowUTC() time.Time
}

type timeProvider struct{}

func NewTimeProvider() TimeProvider {
	return &timeProvider{}
}

func (t timeProvider) NowUTC() time.Time {
	return time.Now().UTC()
}
