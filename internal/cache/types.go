package cache

import (
	"net/http"
)

type CachedResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}
