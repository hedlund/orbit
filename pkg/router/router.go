// Copyright 2023 Henrik Hedlund. All rights reserved.
// Use of this source code is governed by the GNU Affero
// GPL license that can be found in the LICENSE file.

package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

func New() *Router {
	return &Router{
		root: &node{},
	}
}

type Router struct {
	root       *node
	handler    http.Handler
	middleware []func(http.Handler) http.Handler
}

func (r *Router) Use(mw ...func(http.Handler) http.Handler) {
	r.middleware = append(r.middleware, mw...)
}

func (r *Router) Get(path string, h http.HandlerFunc) {
	path = strings.Trim(strings.ToLower(path), "/")
	if err := r.root.add(strings.Split(path, "/"), h); err != nil {
		panic(fmt.Sprintf("%s in path %s", err, path))
	}
}

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	rt.Handler().ServeHTTP(w, r)
}

func (rt *Router) Handler() http.Handler {
	if rt.handler == nil {
		rt.handler = http.HandlerFunc(rt.traverse)
		for n := len(rt.middleware) - 1; n >= 0; n-- {
			rt.handler = rt.middleware[n](rt.handler)
		}
	}
	return rt.handler
}

func (rt *Router) traverse(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	ctx, h := rt.root.find(r.Context(), strings.Split(path, "/"))
	if h == nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	h.ServeHTTP(w, r.WithContext(ctx))
}

type node struct {
	next    map[string]*node
	param   *parameter
	handler http.Handler
}

func (n *node) find(ctx context.Context, path []string) (context.Context, http.Handler) {
	if len(path) == 0 {
		return ctx, n.handler
	}

	segment := strings.ToLower(path[0])
	if next, ok := n.next[segment]; ok {
		return next.find(ctx, path[1:])
	}
	if n.param != nil {
		return n.param.traverse(ctx, path)
	}
	return nil, nil
}

func (n *node) add(path []string, h http.Handler) error {
	if len(path) == 0 {
		return n.addHandler(h)
	}

	segment := path[0]
	if len(segment) == 0 {
		return fmt.Errorf("empty segment")
	}
	if segment[0] == ':' {
		return n.addParameter(segment[1:], path[1:], h)
	}

	if n.next == nil {
		n.next = make(map[string]*node)
	}
	next, ok := n.next[segment]
	if !ok {
		next = &node{}
		n.next[segment] = next
	}
	return next.add(path[1:], h)
}

func (n *node) addHandler(h http.Handler) error {
	if n.handler != nil {
		return fmt.Errorf("duplicate handler")
	}
	n.handler = h
	return nil
}

func (n *node) addParameter(name string, path []string, h http.Handler) error {
	if n.param != nil {
		if n.param.name != name {
			return fmt.Errorf("duplicate parameter")
		}
	} else {
		n.param = &parameter{
			name: name,
		}
	}
	return n.param.add(path, h)
}

type parameter struct {
	node
	name string
}

func (p *parameter) traverse(ctx context.Context, path []string) (context.Context, http.Handler) {
	ctx = withParameter(ctx, p.name, path[0])
	return p.find(ctx, path[1:])
}

func GetParameter(ctx context.Context, name string) string {
	key := parameterContextKey(strings.ToLower(name))
	value, _ := ctx.Value(key).(string)
	return value
}

func withParameter(ctx context.Context, name, value string) context.Context {
	return context.WithValue(ctx, parameterContextKey(name), value)
}

type parameterContextKey string

func (c parameterContextKey) String() string {
	return "parameter context key " + string(c)
}
