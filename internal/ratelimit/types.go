package ratelimit

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type Limiter interface {
	Allow(key string) bool
}

type windowState struct {
	mu      sync.Mutex
	counter int64
	start   time.Time
}

type LRUFixedWindowLimiter struct {
	limit   int64
	window  time.Duration
	clients *lru.Cache[string, *windowState]
}
