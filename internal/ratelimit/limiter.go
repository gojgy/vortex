package ratelimit

import (
	"net/http"
	"strings"
	"time"
	"vortex/internal/utils"

	lru "github.com/hashicorp/golang-lru/v2"
)

func NewLRUFixedWindowLimiter(size int, limit int64, window time.Duration) (*LRUFixedWindowLimiter, error) {
	cache, err := lru.New[string, *windowState](size)
	if err != nil {
		return nil, err
	}

	return &LRUFixedWindowLimiter{
		limit:   limit,
		window:  window,
		clients: cache,
	}, nil
}

func (l *LRUFixedWindowLimiter) Allow(key string) bool {
	now := time.Now()

	state, ok := l.clients.Get(key)

	if !ok {
		newState := &windowState{
			counter: 1,
			start:   now,
		}
		l.clients.Add(key, newState)
		return true
	}
	state.mu.Lock()
	defer state.mu.Unlock()

	if now.After(state.start.Add(l.window)) {
		state.start = now
		state.counter = 1
		return true
	}
	state.counter++

	return state.counter <= l.limit
}

func BuildKey(template string, r *http.Request) string {
	key := strings.NewReplacer(
		"${remote_addr}", utils.GetClientIP(r),
		"${host}", r.Host,
		"${uri}", r.URL.Path,
	).Replace(template)

	return key
}

// TODO: нет четкой связи в создании ключа и констант, в одном месте парсинг в другом сбор + дублирование. стоит придумать хитрее
