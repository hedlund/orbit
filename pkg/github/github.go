// Copyright 2023 Henrik Hedlund. All rights reserved.
// Use of this source code is governed by the GNU Affero
// GPL license that can be found in the LICENSE file.

package github

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const (
	apiVersion  = "2022-11-28"
	contentType = "application/vnd.github+json"
	tagsPerPage = 100
)

type Config struct {
	Repositories map[string][]string `envconfig:"REPOSITORIES"`
	Token        string              `envconfig:"TOKEN"`
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func New(cfg Config, c HTTPClient) *Service {
	return &Service{
		cfg:    cfg,
		client: c,
	}
}

type Service struct {
	cfg    Config
	client HTTPClient
}

// https://docs.github.com/en/rest/repos/repos?apiVersion=2022-11-28#list-repository-tags
func (s *Service) ListVersions(ctx context.Context, owner, repo, module string) ([]string, error) {
	if err := s.validRepo(owner, repo); err != nil {
		return nil, err
	}

	var (
		page     = 1
		prefix   = module + "/"
		versions = []string{}
	)
	for {
		uri := fmt.Sprintf("repos/%s/%s/tags?per_page=%d&page=%d", owner, repo, tagsPerPage, page)
		res, err := s.makeRequest(ctx, uri)
		if err != nil {
			return nil, err
		}

		var tags []struct {
			Name string `json:"name"`
		}
		err = json.NewDecoder(res).Decode(&tags)
		res.Close()
		if err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}

		for _, tag := range tags {
			if strings.HasPrefix(tag.Name, prefix) {
				versions = append(versions, strings.TrimPrefix(tag.Name, prefix))
			}
		}

		if len(tags) < tagsPerPage {
			break
		}
		page++
	}
	return versions, nil
}

func (s *Service) ProxyDownload(ctx context.Context, owner, repo, module, version string, w io.Writer) error {
	if err := s.validRepo(owner, repo); err != nil {
		return err
	}

	uri := fmt.Sprintf("repos/%s/%s/tarball/refs/tags/%s/%s", owner, repo, module, version)
	body, err := s.makeRequest(ctx, uri)
	if err != nil {
		return err
	}
	defer body.Close()

	zr, err := gzip.NewReader(body)
	if err != nil {
		return fmt.Errorf("read gzip: %w", err)
	}
	defer zr.Close()

	zw := gzip.NewWriter(w)
	defer zw.Close()

	tr := tar.NewReader(zr)
	tw := tar.NewWriter(zw)
	defer tw.Close()

	prefix := fmt.Sprintf("^%s-%s-[^/]+/%s/(.+)", owner, repo, module)
	if err := copy(prefix, tw, tr); err != nil {
		return err
	}
	return nil
}

func (s *Service) makeRequest(ctx context.Context, uri string) (io.ReadCloser, error) {
	url := fmt.Sprintf("https://api.github.com/%s", uri)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	req.Header.Add("Accept", contentType)
	req.Header.Add("X-GitHub-Api-Version", apiVersion)

	if token := GetToken(ctx, s.cfg.Token); token != "" {
		req.Header.Add("Authorization", "Bearer "+token)
	}

	res, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, &httpErr{
			code: res.StatusCode,
			msg:  slurp(res.Body),
		}
	}

	return res.Body, nil
}

func (s *Service) validRepo(owner, repo string) error {
	if len(s.cfg.Repositories) == 0 {
		return nil
	}

	if repos, ok := s.cfg.Repositories[owner]; ok {
		for _, r := range repos {
			if r == repo {
				return nil
			}
		}
	}

	return &httpErr{
		code: http.StatusForbidden,
		msg:  "not a valid repository",
	}
}

func copy(prefix string, w *tar.Writer, r *tar.Reader) error {
	re, err := regexp.Compile(prefix)
	if err != nil {
		return fmt.Errorf("compile prefix regexp: %w", err)
	}

	for {
		hdr, err := r.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		match := re.FindStringSubmatch(hdr.Name)
		if len(match) == 2 {
			hdr.Name = match[1]
			if err := w.WriteHeader(hdr); err != nil {
				return fmt.Errorf("writing header: %w", err)
			}
			io.Copy(w, r)
		}
	}
}

func slurp(r io.ReadCloser) string {
	defer r.Close()

	b, err := io.ReadAll(r)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

type httpErr struct {
	code int
	msg  string
}

func (e *httpErr) Error() string {
	return e.msg
}

func (e *httpErr) StatusCode() int {
	return e.code
}
