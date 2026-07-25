// Package overlay serves the Svelte frontends: the in-app web overlay and the
// admin dashboard. In production they are embedded in the binary; in dev mode
// requests are proxied to the Vite dev server so hot reload keeps working.
package overlay

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"time"
)

// zeroTime suppresses Last-Modified/If-Modified-Since handling: embedded files
// have no meaningful modification time, and their names are content-hashed.
var zeroTime time.Time

// distFS holds the built Svelte assets. The `all:` prefix keeps files that
// begin with "_" or "." — Vite emits some of those.
//
//go:embed all:dist
var distFS embed.FS

// Entry points produced by the Vite build.
const (
	overlayEntry   = "overlay.html"
	dashboardEntry = "dashboard.html"
)

// DefaultViteURL is where `npm run dev` serves the frontend.
const DefaultViteURL = "http://localhost:5173"

// Assets returns the embedded build output rooted at dist/.
func Assets() (fs.FS, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, fmt.Errorf("open embedded assets: %w", err)
	}
	return sub, nil
}

// Built reports whether a real frontend build is embedded. A binary compiled
// before `make web` only carries the placeholder, and callers may want to say
// so rather than serve a stub.
func Built() bool {
	assets, err := Assets()
	if err != nil {
		return false
	}
	_, err = fs.Stat(assets, overlayEntry)
	return err == nil
}

// OverlayHandler serves the in-app overlay. Unknown paths fall back to the
// overlay entry point so client-side routing works.
func OverlayHandler() (http.Handler, error) {
	assets, err := Assets()
	if err != nil {
		return nil, err
	}
	return spaHandler(assets, overlayEntry, ""), nil
}

// DashboardHandler serves the admin SPA mounted at prefix (e.g.
// "/internal/admin").
func DashboardHandler(prefix string) (http.Handler, error) {
	assets, err := Assets()
	if err != nil {
		return nil, err
	}
	return spaHandler(assets, dashboardEntry, prefix), nil
}

// spaHandler serves static files out of assets, falling back to entry for any
// path that does not resolve to a file. prefix, when set, is stripped from the
// request path first.
func spaHandler(assets fs.FS, entry, prefix string) http.Handler {
	files := http.FileServer(http.FS(assets))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if prefix != "" {
			upath = strings.TrimPrefix(upath, strings.Trim(prefix, "/"))
			upath = strings.TrimPrefix(upath, "/")
		}

		if upath == "" || !exists(assets, upath) {
			serveFile(w, r, assets, entry)
			return
		}

		// Rewrite so the file server resolves the path we just normalised
		// rather than the original, prefixed one.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/" + upath
		files.ServeHTTP(w, r2)
	})
}

func exists(assets fs.FS, name string) bool {
	info, err := fs.Stat(assets, name)
	return err == nil && !info.IsDir()
}

func serveFile(w http.ResponseWriter, r *http.Request, assets fs.FS, name string) {
	data, err := fs.ReadFile(assets, name)
	if err != nil {
		http.Error(w, "frontend not built", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// The entry HTML must not be cached: it names the hashed asset bundles.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, name, zeroTime, strings.NewReader(string(data)))
}

// DevProxy forwards everything to a running Vite dev server. stripPrefix is
// removed from the path first, so the dashboard mounted under /internal/admin
// still hits Vite's root.
func DevProxy(target, stripPrefix string) (http.Handler, error) {
	if target == "" {
		target = DefaultViteURL
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("parse vite url %q: %w", target, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(parsed)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if stripPrefix != "" {
			trimmed := strings.TrimPrefix(r.URL.Path, strings.TrimSuffix(stripPrefix, "/"))
			if trimmed == "" {
				trimmed = "/"
			}
			r = r.Clone(r.Context())
			r.URL.Path = trimmed
		}
		proxy.ServeHTTP(w, r)
	}), nil
}
