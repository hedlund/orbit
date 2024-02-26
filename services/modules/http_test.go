package modules

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hedlund/orbit/pkg/expect"
	"github.com/hedlund/orbit/pkg/router"
)

func TestListVersions(t *testing.T) {
	tests := []struct {
		name      string
		repo      *mockRepository
		req       *http.Request
		expStatus int
		expBody   string
	}{
		{
			name: "success",
			req:  mockRequest(t, "/v1/modules/repo/module/owner/versions"),
			repo: &mockRepository{
				versions: []string{"666", "1349"},
				owner:    expect.Value("owner"),
				repo:     expect.Value("repo"),
				module:   expect.Value("module"),
			},
			expStatus: http.StatusOK,
			expBody:   `{"modules":[{"versions":[{"version":"666"},{"version":"1349"}]}]}`,
		},
		{
			name: "empty_response",
			req:  mockRequest(t, "/v1/modules/foo/bar/baz/versions"),
			repo: &mockRepository{
				owner:  expect.Value("baz"),
				repo:   expect.Value("foo"),
				module: expect.Value("bar"),
			},
			expStatus: http.StatusOK,
			expBody:   `{"modules":[{"versions":[]}]}`,
		},
		{
			name: "repo_error",
			req:  mockRequest(t, "/v1/modules/foo/bar/baz/versions"),
			repo: &mockRepository{
				err:    fmt.Errorf("oopsie"),
				owner:  expect.Value("baz"),
				repo:   expect.Value("foo"),
				module: expect.Value("bar"),
			},
			expStatus: http.StatusInternalServerError,
			expBody:   `Internal Server Error`,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			handler := &Handler{
				log:  slog.Default(),
				repo: tt.repo,
			}

			rr := httptest.NewRecorder()
			h := route("/v1/modules/:namespace/:name/:system/versions", handler.ListVersions)
			h.ServeHTTP(rr, tt.req)

			res := rr.Result()
			defer res.Body.Close()

			b, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("reading response body: %s", err)
			}
			body := strings.TrimSpace(string(b))

			if res.StatusCode != tt.expStatus {
				t.Errorf("unexpected status code, exp: %d, got: %d", tt.expStatus, res.StatusCode)
			}
			if body != tt.expBody {
				t.Errorf("unexpected response body, exp: %s, got: %s", tt.expBody, body)
			}

			tt.repo.validate(t)
		})
	}
}

// route is a helper function to wrap the router config of the handler func we
// are testing. That we have to do this in the first place, and copy the routes
// from main.go, is an indication that there's a poor abstraction in place that
// we'll need to fix.
// TODO: Remove the need for this function
func route(path string, f http.HandlerFunc) http.Handler {
	r := router.New()
	r.Get(path, f)
	return r
}

// mockRequest simply creates an HTTP request suitable for testing the modules
// handler.
func mockRequest(t *testing.T, url string) *http.Request {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("creating request: %s", err)
	}
	return req
}

type mockRepository struct {
	versions []string
	err      error

	owner   expect.V[string]
	repo    expect.V[string]
	module  expect.V[string]
	version expect.V[string]
}

func (m *mockRepository) ListVersions(ctx context.Context, owner, repo, module string) ([]string, error) {
	m.owner.Got(owner)
	m.repo.Got(repo)
	m.module.Got(module)
	return m.versions, m.err
}

func (m *mockRepository) ProxyDownload(ctx context.Context, owner, repo, module, version string, w io.Writer) error {
	m.owner.Got(owner)
	m.repo.Got(repo)
	m.module.Got(module)
	m.version.Got(version)
	return m.err
}

func (m *mockRepository) validate(t *testing.T) {
	t.Helper()

	m.owner.Validate(t, "owner")
	m.repo.Validate(t, "repo")
	m.module.Validate(t, "module")
	m.version.Validate(t, "version")
}
