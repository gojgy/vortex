package main

import (
	"net/http"
	"runtime/debug"

	"github.com/rs/zerolog"
)

func panicRecoveryMiddleware(next http.Handler, logger zerolog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error().
					Interface("panic", err).
					Bytes("stack", debug.Stack()).
					Msg("recovered from panic")

				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
