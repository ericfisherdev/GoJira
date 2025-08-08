package queue

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type RateLimiter struct {
	rate       int           // requests per second
	burst      int           // burst capacity
	tokens     chan struct{}
	stopCh     chan struct{}
	mu         sync.RWMutex
	stats      *RateLimiterStats
}

type RateLimiterStats struct {
	TotalRequests   int64
	AllowedRequests int64
	DeniedRequests  int64
	CurrentTokens   int
	mu              sync.RWMutex
}

func NewRateLimiter(rate, burst int) *RateLimiter {
	if rate <= 0 {
		rate = 10
	}
	if burst <= 0 {
		burst = rate * 2
	}

	rl := &RateLimiter{
		rate:   rate,
		burst:  burst,
		tokens: make(chan struct{}, burst),
		stopCh: make(chan struct{}),
		stats:  &RateLimiterStats{},
	}

	// Fill initial tokens
	for i := 0; i < burst; i++ {
		rl.tokens <- struct{}{}
	}
	rl.updateStats(func(s *RateLimiterStats) {
		s.CurrentTokens = burst
	})

	// Start token refill goroutine
	go rl.refillTokens()

	log.Info().
		Int("rate", rate).
		Int("burst", burst).
		Msg("Rate limiter initialized")

	return rl
}

func (rl *RateLimiter) refillTokens() {
	ticker := time.NewTicker(time.Second / time.Duration(rl.rate))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			select {
			case rl.tokens <- struct{}{}:
				// Token added
				rl.updateStats(func(s *RateLimiterStats) {
					s.CurrentTokens = len(rl.tokens)
				})
			default:
				// Bucket full
			}
		case <-rl.stopCh:
			log.Info().Msg("Rate limiter stopping")
			return
		}
	}
}

func (rl *RateLimiter) Wait() {
	rl.updateStats(func(s *RateLimiterStats) {
		s.TotalRequests++
	})

	<-rl.tokens

	rl.updateStats(func(s *RateLimiterStats) {
		s.AllowedRequests++
		s.CurrentTokens = len(rl.tokens)
	})
}

func (rl *RateLimiter) WaitContext(ctx context.Context) error {
	rl.updateStats(func(s *RateLimiterStats) {
		s.TotalRequests++
	})

	select {
	case <-rl.tokens:
		rl.updateStats(func(s *RateLimiterStats) {
			s.AllowedRequests++
			s.CurrentTokens = len(rl.tokens)
		})
		return nil
	case <-ctx.Done():
		rl.updateStats(func(s *RateLimiterStats) {
			s.DeniedRequests++
		})
		return ctx.Err()
	}
}

func (rl *RateLimiter) TryWait(timeout time.Duration) bool {
	rl.updateStats(func(s *RateLimiterStats) {
		s.TotalRequests++
	})

	select {
	case <-rl.tokens:
		rl.updateStats(func(s *RateLimiterStats) {
			s.AllowedRequests++
			s.CurrentTokens = len(rl.tokens)
		})
		return true
	case <-time.After(timeout):
		rl.updateStats(func(s *RateLimiterStats) {
			s.DeniedRequests++
		})
		return false
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.updateStats(func(s *RateLimiterStats) {
		s.TotalRequests++
	})

	select {
	case <-rl.tokens:
		rl.updateStats(func(s *RateLimiterStats) {
			s.AllowedRequests++
			s.CurrentTokens = len(rl.tokens)
		})
		return true
	default:
		rl.updateStats(func(s *RateLimiterStats) {
			s.DeniedRequests++
		})
		return false
	}
}

func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

func (rl *RateLimiter) GetStats() RateLimiterStats {
	rl.stats.mu.RLock()
	defer rl.stats.mu.RUnlock()
	return *rl.stats
}

func (rl *RateLimiter) updateStats(fn func(*RateLimiterStats)) {
	rl.stats.mu.Lock()
	defer rl.stats.mu.Unlock()
	fn(rl.stats)
}

func (rl *RateLimiter) AvailableTokens() int {
	return len(rl.tokens)
}

func (rl *RateLimiter) ResetStats() {
	rl.stats.mu.Lock()
	defer rl.stats.mu.Unlock()
	rl.stats.TotalRequests = 0
	rl.stats.AllowedRequests = 0
	rl.stats.DeniedRequests = 0
}

type TokenBucket struct {
	capacity     int
	tokens       float64
	rate         float64 // tokens per second
	lastRefill   time.Time
	mu           sync.Mutex
}

func NewTokenBucket(capacity int, rate float64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     float64(capacity),
		rate:       rate,
		lastRefill: time.Now(),
	}
}

func (tb *TokenBucket) Allow(tokens int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= float64(tokens) {
		tb.tokens -= float64(tokens)
		return true
	}

	return false
}

func (tb *TokenBucket) AllowN(tokens int) bool {
	return tb.Allow(tokens)
}

func (tb *TokenBucket) Wait(tokens int) {
	for !tb.Allow(tokens) {
		time.Sleep(100 * time.Millisecond)
	}
}

func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.rate

	if tb.tokens > float64(tb.capacity) {
		tb.tokens = float64(tb.capacity)
	}

	tb.lastRefill = now
}

func (tb *TokenBucket) AvailableTokens() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	return int(tb.tokens)
}

type LeakyBucket struct {
	capacity int
	used     int
	rate     int // leak rate per second
	mu       sync.Mutex
	stopCh   chan struct{}
}

func NewLeakyBucket(capacity, rate int) *LeakyBucket {
	lb := &LeakyBucket{
		capacity: capacity,
		rate:     rate,
		stopCh:   make(chan struct{}),
	}

	go lb.leak()
	return lb
}

func (lb *LeakyBucket) Allow(amount int) bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if lb.used+amount <= lb.capacity {
		lb.used += amount
		return true
	}

	return false
}

func (lb *LeakyBucket) leak() {
	ticker := time.NewTicker(time.Second / time.Duration(lb.rate))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lb.mu.Lock()
			if lb.used > 0 {
				lb.used--
			}
			lb.mu.Unlock()
		case <-lb.stopCh:
			return
		}
	}
}

func (lb *LeakyBucket) Stop() {
	close(lb.stopCh)
}

func (lb *LeakyBucket) Available() int {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.capacity - lb.used
}