// Copyright 2023 Henrik Hedlund. All rights reserved.
// Use of this source code is governed by the GNU Affero
// GPL license that can be found in the LICENSE file.

package auth

import (
	"context"
	"net/http"
	"strings"
)

func TokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if len(header) >= 7 && strings.ToLower(header[0:7]) == "bearer " {
			token := strings.TrimSpace(header[7:])
			if token != "" {
				ctx := WithToken(r.Context(), token)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenContextKey, token)
}

func GetToken(ctx context.Context, s string) string {
	if token, _ := ctx.Value(tokenContextKey).(string); token != "" {
		return token
	}
	return s
}

type contextKey struct{}

var tokenContextKey contextKey
