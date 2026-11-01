package view

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// csp allows only same-origin scripts and styles: the pages carry no inline
// script or style, and downloads are navigations, not fetches. No other page
// may frame the site.
const csp = "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// contentTypes lists the only kinds of file the site contains; anything else is 404.
var contentTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".js":   "text/javascript; charset=utf-8",
	".md":   "text/markdown; charset=utf-8",
}

// shutdownGrace is how long in-flight requests get once ctx is done.
const shutdownGrace = 5 * time.Second

// Serve serves the site in dir on 127.0.0.1:port (0 picks a free port). It
// calls ready with the site URL once listening, blocks until ctx is done, then
// shuts down gracefully and returns nil. It returns an error if dir cannot be
// opened or the port cannot be bound.
func Serve(ctx context.Context, dir string, port int, ready func(url string)) error {
	if port < 0 || port > 65535 {
		return fmt.Errorf("invalid port %d", port)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()

	ln, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return err
	}
	actual := ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{
		Handler:                      siteHandler(root.FS(), actual),
		ReadHeaderTimeout:            10 * time.Second,
		IdleTimeout:                  60 * time.Second,
		DisableGeneralOptionsHandler: true, // "OPTIONS *" goes through the handler's checks
	}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
	if ready != nil {
		ready("http://127.0.0.1:" + strconv.Itoa(actual) + "/")
	}

	select {
	case <-ctx.Done():
	case err := <-served:
		return err // Serve never returns nil; this is a listener failure
	}
	sctx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		srv.Close()
	}
	<-served // http.ErrServerClosed
	return nil
}

// siteHandler serves the files of fsys, which must be rooted at the site folder,
// to requests addressed to this server on port.
func siteHandler(fsys fs.FS, port int) http.Handler {
	hosts := map[string]bool{
		"127.0.0.1:" + strconv.Itoa(port): true,
		"localhost:" + strconv.Itoa(port): true,
	}
	files := http.FS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		// A page of another origin may not embed any file, e.g. data.js as a script.
		h.Set("Cross-Origin-Resource-Policy", "same-origin")

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			h.Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// An exact match defeats DNS rebinding: a page from another origin
		// reaching this port sends its own host name.
		if !hosts[r.Host] || !sameOriginFetch(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		name, ok := siteName(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		fi, err := fs.Stat(fsys, name)
		if err == nil && fi.IsDir() && (name == "." || strings.HasSuffix(r.URL.Path, "/")) {
			name = path.Join(name, "index.html")
			fi, err = fs.Stat(fsys, name)
		}
		ctype := contentTypes[path.Ext(name)]
		// Regular files only: never a directory, and never a FIFO or device,
		// whose open could block or read something that is not the site.
		if err != nil || !fi.Mode().IsRegular() || ctype == "" {
			http.NotFound(w, r)
			return
		}
		f, err := files.Open("/" + name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		h.Set("Content-Type", ctype)
		h.Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, name, fi.ModTime(), f)
	})
}

// sameOriginFetch reports whether the request comes from the site itself or from
// the user (address bar, bookmark). A page of another origin that includes
// <script src="http://127.0.0.1:PORT/data.js"> sends this server's own Host, so
// the Host check alone cannot stop it; the browser's Sec-Fetch-Site header can.
// A request without the header (an older browser, curl) is allowed.
func sameOriginFetch(r *http.Request) bool {
	v, ok := r.Header["Sec-Fetch-Site"]
	if !ok {
		return true
	}
	return len(v) == 1 && (v[0] == "same-origin" || v[0] == "none")
}

// siteName maps a decoded URL path to a name inside the site folder ("." for
// the root). It refuses empty, "." and ".." segments, any segment starting
// with "." (dotfiles at any depth), backslashes and NUL.
func siteName(p string) (string, bool) {
	if !strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\x00") {
		return "", false
	}
	rest := strings.TrimSuffix(p[1:], "/")
	if rest == "" {
		if p != "/" {
			return "", false // "//"
		}
		return ".", true
	}
	for _, seg := range strings.Split(rest, "/") {
		if seg == "" || strings.HasPrefix(seg, ".") {
			return "", false
		}
	}
	if !fs.ValidPath(rest) {
		return "", false
	}
	return rest, true
}
