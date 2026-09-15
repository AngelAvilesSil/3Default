package auth

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestLoginAttemptLimiter(
	t *testing.T,
	capacity int,
	refillInterval time.Duration,
	maxEntries int,
	now *time.Time,
) *InMemoryLoginAttemptLimiter {
	t.Helper()

	limiter, err := NewInMemoryLoginAttemptLimiter(
		LoginAttemptLimiterConfig{
			Capacity:       capacity,
			RefillInterval: refillInterval,
			MaxEntries:     maxEntries,
		},
	)
	if err != nil {
		t.Fatalf(
			"NewInMemoryLoginAttemptLimiter() error = %v",
			err,
		)
	}

	limiter.now = func() time.Time {
		return *now
	}

	return limiter
}

func TestLoginAttemptLimiterConsumesCapacity(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.September,
		14,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	limiter := newTestLoginAttemptLimiter(
		t,
		3,
		time.Minute,
		100,
		&now,
	)

	for attempt := 1; attempt <= 3; attempt++ {
		if !limiter.Allow("person@example.com") {
			t.Fatalf(
				"Allow() attempt %d = false, want true",
				attempt,
			)
		}
	}

	if limiter.Allow("person@example.com") {
		t.Fatal(
			"Allow() after capacity exhausted = true, want false",
		)
	}
}

func TestLoginAttemptLimiterRefillsOverTime(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.September,
		14,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	limiter := newTestLoginAttemptLimiter(
		t,
		2,
		time.Minute,
		100,
		&now,
	)

	if !limiter.Allow("person@example.com") {
		t.Fatal("first Allow() = false, want true")
	}

	if !limiter.Allow("person@example.com") {
		t.Fatal("second Allow() = false, want true")
	}

	if limiter.Allow("person@example.com") {
		t.Fatal("third Allow() = true, want false")
	}

	now = now.Add(59 * time.Second)

	if limiter.Allow("person@example.com") {
		t.Fatal(
			"Allow() before refill interval = true, want false",
		)
	}

	now = now.Add(time.Second)

	if !limiter.Allow("person@example.com") {
		t.Fatal(
			"Allow() after refill interval = false, want true",
		)
	}

	if limiter.Allow("person@example.com") {
		t.Fatal(
			"second Allow() after one refill = true, want false",
		)
	}
}

func TestLoginAttemptLimiterPreservesPartialRefillTime(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.September,
		14,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	limiter := newTestLoginAttemptLimiter(
		t,
		2,
		time.Minute,
		100,
		&now,
	)

	if !limiter.Allow("person@example.com") {
		t.Fatal("first Allow() = false, want true")
	}

	if !limiter.Allow("person@example.com") {
		t.Fatal("second Allow() = false, want true")
	}

	now = now.Add(90 * time.Second)

	if !limiter.Allow("person@example.com") {
		t.Fatal(
			"Allow() after 90 seconds = false, want true",
		)
	}

	now = now.Add(30 * time.Second)

	if !limiter.Allow("person@example.com") {
		t.Fatal(
			"Allow() after remaining 30 seconds = false, want true",
		)
	}

	if limiter.Allow("person@example.com") {
		t.Fatal(
			"Allow() without another refill interval = true, want false",
		)
	}
}

func TestLoginAttemptLimiterRefillDoesNotExceedCapacity(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.September,
		14,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	limiter := newTestLoginAttemptLimiter(
		t,
		2,
		time.Minute,
		100,
		&now,
	)

	if !limiter.Allow("person@example.com") {
		t.Fatal("Allow() = false, want true")
	}

	now = now.Add(10 * time.Minute)

	if !limiter.Allow("person@example.com") {
		t.Fatal(
			"first Allow() after long refill = false, want true",
		)
	}

	if !limiter.Allow("person@example.com") {
		t.Fatal(
			"second Allow() after long refill = false, want true",
		)
	}

	if limiter.Allow("person@example.com") {
		t.Fatal(
			"third Allow() after long refill = true, want false",
		)
	}
}

func TestLoginAttemptLimiterSuccessClearsFailures(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.September,
		14,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	limiter := newTestLoginAttemptLimiter(
		t,
		2,
		time.Hour,
		100,
		&now,
	)

	if !limiter.Allow("person@example.com") {
		t.Fatal("first Allow() = false, want true")
	}

	if !limiter.Allow("person@example.com") {
		t.Fatal("second Allow() = false, want true")
	}

	if limiter.Allow("person@example.com") {
		t.Fatal(
			"Allow() before success reset = true, want false",
		)
	}

	limiter.Succeeded("person@example.com")

	if !limiter.Allow("person@example.com") {
		t.Fatal(
			"Allow() after success reset = false, want true",
		)
	}
}

func TestLoginAttemptLimiterSeparatesIdentifiers(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.September,
		14,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	limiter := newTestLoginAttemptLimiter(
		t,
		1,
		time.Hour,
		100,
		&now,
	)

	if !limiter.Allow("first@example.com") {
		t.Fatal(
			"Allow() for first identifier = false, want true",
		)
	}

	if limiter.Allow("first@example.com") {
		t.Fatal(
			"second Allow() for first identifier = true, want false",
		)
	}

	if !limiter.Allow("second@example.com") {
		t.Fatal(
			"Allow() for second identifier = false, want true",
		)
	}
}

func TestLoginAttemptLimiterEvictsLeastRecentlyUsedEntry(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.September,
		14,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	limiter := newTestLoginAttemptLimiter(
		t,
		1,
		time.Hour,
		2,
		&now,
	)

	if !limiter.Allow("first@example.com") {
		t.Fatal("first identifier Allow() = false, want true")
	}

	if !limiter.Allow("second@example.com") {
		t.Fatal("second identifier Allow() = false, want true")
	}

	if limiter.Allow("first@example.com") {
		t.Fatal(
			"second first-identifier Allow() = true, want false",
		)
	}

	if !limiter.Allow("third@example.com") {
		t.Fatal("third identifier Allow() = false, want true")
	}

	if !limiter.Allow("second@example.com") {
		t.Fatal(
			"evicted identifier Allow() = false, want true",
		)
	}
}

func TestLoginAttemptLimiterIsSafeForConcurrentAttempts(
	t *testing.T,
) {
	now := time.Date(
		2026,
		time.September,
		14,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	limiter := newTestLoginAttemptLimiter(
		t,
		5,
		time.Hour,
		100,
		&now,
	)

	var allowed atomic.Int32

	var group sync.WaitGroup

	for range 100 {
		group.Add(1)

		go func() {
			defer group.Done()

			if limiter.Allow("person@example.com") {
				allowed.Add(1)
			}
		}()
	}

	group.Wait()

	if allowed.Load() != 5 {
		t.Fatalf(
			"allowed attempts = %d, want 5",
			allowed.Load(),
		)
	}
}

func TestNewLoginAttemptLimiterRejectsInvalidConfig(
	t *testing.T,
) {
	tests := []struct {
		name   string
		config LoginAttemptLimiterConfig
		want   error
	}{
		{
			name: "capacity",
			config: LoginAttemptLimiterConfig{
				Capacity:       0,
				RefillInterval: time.Minute,
				MaxEntries:     100,
			},
			want: ErrInvalidLoginAttemptCapacity,
		},
		{
			name: "refill interval",
			config: LoginAttemptLimiterConfig{
				Capacity:       5,
				RefillInterval: 0,
				MaxEntries:     100,
			},
			want: ErrInvalidLoginAttemptRefillInterval,
		},
		{
			name: "max entries",
			config: LoginAttemptLimiterConfig{
				Capacity:       5,
				RefillInterval: time.Minute,
				MaxEntries:     0,
			},
			want: ErrInvalidLoginAttemptMaxEntries,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewInMemoryLoginAttemptLimiter(
				test.config,
			)

			if !errors.Is(err, test.want) {
				t.Fatalf(
					"error = %v, want %v",
					err,
					test.want,
				)
			}
		})
	}
}
