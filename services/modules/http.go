// Copyright 2023 Henrik Hedlund. All rights reserved.
// Use of this source code is governed by the GNU Affero
// GPL license that can be found in the LICENSE file.

package modules

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hedlund/orbit/pkg/auth"
	"github.com/hedlund/orbit/pkg/router"
)

var (
	errInvalidToken = errors.New("invalid token")
	errTokenExpired = errors.New("token expired")
)

type Config struct {
	ProxySecret     []byte        `envconfig:"PROXY_SECRET" required:"true"`
	TokenExpiration time.Duration `envconfig:"TOKEN_EXPIRATION" default:"60s"`
}

type Cipher interface {
	Seal(dst, nonce, plaintext, additionalData []byte) []byte
	Open(dst, nonce, ciphertext, additionalData []byte) ([]byte, error)
	NonceSize() int
}

type Logger interface {
	Error(msg string, args ...any)
	Info(msg string, args ...any)
}

type Repository interface {
	ListVersions(ctx context.Context, owner, repo, module string) ([]string, error)
	ProxyDownload(ctx context.Context, owner, repo, module, version string, w io.Writer) error
}

func NewHTTP(cfg Config, log Logger, r Repository) (*Handler, error) {
	c, err := aes.NewCipher(cfg.ProxySecret)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	return &Handler{
		cfg:    cfg,
		cipher: gcm,
		log:    log,
		now:    time.Now,
		repo:   r,
	}, nil
}

type Handler struct {
	cfg    Config
	cipher Cipher
	log    Logger
	now    func() time.Time
	repo   Repository
}

func (h *Handler) ListVersions(w http.ResponseWriter, r *http.Request) {
	var (
		ctx       = r.Context()
		namespace = router.GetParameter(ctx, "namespace")
		name      = router.GetParameter(ctx, "name")
		system    = router.GetParameter(ctx, "system")
	)

	versions, err := h.repo.ListVersions(ctx, system, namespace, name)
	if err != nil {
		h.log.Error("list versions", "err", err)
		respErr(w, err)
		return
	}

	res := newListVersionsResponse(versions)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&res); err != nil {
		h.log.Error("encode response", "err", err)
		return
	}
}

func (h *Handler) DownloadURL(w http.ResponseWriter, r *http.Request) {
	downloadURL := "./proxy?archive=tar.gz"
	if token := auth.GetToken(r.Context(), ""); token != "" {
		encoded, err := h.encodeToken(token)
		if err != nil {
			h.log.Error("encoding token", "err", err)
			respErr(w, err)
			return
		}
		downloadURL += "&token=" + encoded
	}
	w.Header().Add("X-Terraform-Get", downloadURL)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ProxyDownload(w http.ResponseWriter, r *http.Request) {
	var (
		ctx       = r.Context()
		namespace = router.GetParameter(ctx, "namespace")
		name      = router.GetParameter(ctx, "name")
		system    = router.GetParameter(ctx, "system")
		version   = router.GetParameter(ctx, "version")
		token     = r.URL.Query().Get("token")
	)

	if token != "" {
		var err error
		token, err = h.decodeToken(token)
		if err != nil {
			h.log.Error("decoding token", "err", err)
			respErr(w, err)
			return
		}
		ctx = auth.WithToken(ctx, token)
	}

	// w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s-%s-%s.tar.gz", owner, repo, module, version))
	if err := h.repo.ProxyDownload(ctx, system, namespace, name, version, w); err != nil {
		h.log.Error("proxy download", "err", err)
		respErr(w, err)
		return
	}
}

func (h *Handler) encodeToken(token string) (string, error) {
	b, err := json.Marshal(&encodedToken{
		Token:     token,
		EncodedAt: h.now().Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("marshalling token into JSON: %w", err)
	}

	nonce := make([]byte, h.cipher.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generating token nonce: %w", err)
	}

	encoded := h.cipher.Seal(nonce, nonce, b, nil)
	return hex.EncodeToString(encoded), nil
}

func (h *Handler) decodeToken(encoded string) (string, error) {
	ciphertext, err := hex.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding token string: %w", err)
	}

	nonceSize := h.cipher.NonceSize()
	if len(encoded) < nonceSize {
		return "", fmt.Errorf("token is too short: %w", errInvalidToken)
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	b, err := h.cipher.Open(nil, nonce, []byte(ciphertext), nil)
	if err != nil {
		return "", fmt.Errorf("decoding token: %w", err)
	}

	var token encodedToken
	if err := json.Unmarshal(b, &token); err != nil {
		return "", fmt.Errorf("unmarshal token: %w", err)
	}

	now := h.now().UTC()
	validUntil := time.Unix(token.EncodedAt, 0).Add(h.cfg.TokenExpiration).UTC()
	if !now.Before(validUntil) {
		return "", fmt.Errorf("%w: valid until %s", errTokenExpired, validUntil)
	}

	return token.Token, nil
}

func respErr(w http.ResponseWriter, err error) {
	var code int
	switch x := err.(type) {
	case interface{ StatusCode() int }:
		code = x.StatusCode()
	default:
		code = http.StatusInternalServerError
	}
	http.Error(w, http.StatusText(code), code)
}

func newListVersionsResponse(versions []string) *listVersionsResponse {
	m := module{
		Versions: make([]version, len(versions)),
	}
	for n, v := range versions {
		m.Versions[n] = version{v}
	}
	return &listVersionsResponse{
		Modules: []module{m},
	}
}

type listVersionsResponse struct {
	Modules []module `json:"modules"`
}

type module struct {
	Versions []version `json:"versions"`
}

type version struct {
	Version string `json:"version"`
}

type encodedToken struct {
	Token     string `json:"token"`
	EncodedAt int64  `json:"encoded_at"`
}
