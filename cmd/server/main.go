// Copyright 2023 Henrik Hedlund. All rights reserved.
// Use of this source code is governed by the GNU Affero
// GPL license that can be found in the LICENSE file.

package main

import (
	"net/http"
	"os"
	"time"

	"golang.org/x/exp/slog"

	"github.com/hedlund/orbit/pkg/envconfig"
	"github.com/hedlund/orbit/pkg/github"
	"github.com/hedlund/orbit/pkg/mcache"
	"github.com/hedlund/orbit/pkg/router"
	"github.com/hedlund/orbit/pkg/server"
	"github.com/hedlund/orbit/services/modules"
)

type config struct {
	Cache struct {
		Enabled    bool          `envconfig:"ENABLED"`
		Path       string        `envconfig:"PATH" default:"/tmp"`
		Expiration time.Duration `envconfig:"EXPIRATION" default:"10s"`
	} `envconfig:"CACHE_"`
	Github github.Config `envconfig:"GITHUB_"`
	Server server.Config
}

func main() {
	var cfg config
	envconfig.MustProcess(&cfg)

	log := slog.New(slog.NewTextHandler(os.Stdout))

	var repo modules.Repository
	repo = github.New(&http.Client{
		Timeout: 5 * time.Second,
	})

	if cfg.Cache.Enabled {
		log.Info("enabling cache", "path", cfg.Cache.Path, "expiration", cfg.Cache.Expiration)
		repo = modules.NewCache(
			repo,
			mcache.New[string, []string](cfg.Cache.Expiration),
			modules.StoreInPath(cfg.Cache.Path),
			log,
		)
	}

	h := modules.NewHTTP(log, repo)

	r := router.New()
	r.Use(github.AddTokenMiddleware)

	r.Get("/v1/modules/:namespace/:name/:system/versions", h.ListVersions)
	r.Get("/v1/modules/:namespace/:name/:system/:version/download", h.DownloadURL)
	r.Get("/v1/modules/:namespace/:name/:system/:version/proxy", h.ProxyDownload)
	r.Get("/.well-known/terraform.json", discovery)

	if err := server.Start(cfg.Server, log, r); err != nil {
		panic(err)
	}
}

func discovery(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.Write([]byte(`{"modules.v1":"/v1/modules"}`))
}
