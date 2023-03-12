// Copyright 2023 Henrik Hedlund. All rights reserved.
// Use of this source code is governed by the GNU Affero
// GPL license that can be found in the LICENSE file.

package modules

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/hedlund/orbit/pkg/router"
)

type Logger interface {
	Error(msg string, args ...any)
	Info(msg string, args ...any)
}

type Repository interface {
	ListVersions(ctx context.Context, owner, repo, module string) ([]string, error)
	ProxyDownload(ctx context.Context, owner, repo, module, version string, w io.Writer) error
}

func NewHTTP(log Logger, m Repository) *Handler {
	return &Handler{log, m}
}

type Handler struct {
	log  Logger
	repo Repository
}

func (h Handler) ListVersions(w http.ResponseWriter, r *http.Request) {
	var (
		ctx       = r.Context()
		namespace = router.GetParameter(ctx, "namespace")
		name      = router.GetParameter(ctx, "name")
		system    = router.GetParameter(ctx, "system")
	)

	versions, err := h.repo.ListVersions(ctx, namespace, name, system)
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

func (h Handler) DownloadURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("X-Terraform-Get", "./proxy?archive=tar.gz")
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) ProxyDownload(w http.ResponseWriter, r *http.Request) {
	var (
		ctx       = r.Context()
		namespace = router.GetParameter(ctx, "namespace")
		name      = router.GetParameter(ctx, "name")
		system    = router.GetParameter(ctx, "system")
		version   = router.GetParameter(ctx, "version")
	)

	//w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s-%s-%s.tar.gz", owner, repo, module, version))
	if err := h.repo.ProxyDownload(ctx, namespace, name, system, version, w); err != nil {
		h.log.Error("proxy download", "err", err)
		respErr(w, err)
		return
	}
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
	var m = module{
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
