package view

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const wantCSP = "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'"

// served is a running Serve: its URL, port, and how to stop it.
type served struct {
	url  string
	port int
	stop context.CancelFunc
	done chan error
}

// startServe runs Serve on dir and port in the background and waits for ready.
// It is stopped and waited for at the end of the test.
func startServe(t *testing.T, dir string, port int) *served {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	urls := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, dir, port, func(u string) { urls <- u }) }()
	s := &served{stop: cancel, done: done}
	select {
	case s.url = <-urls:
	case err := <-done:
		cancel()
		t.Fatalf("Serve returned before ready: %v", err)
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("Serve never became ready")
	}
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("Serve did not stop")
		}
	})
	_, p, err := net.SplitHostPort(strings.TrimSuffix(strings.TrimPrefix(s.url, "http://"), "/"))
	if err != nil {
		t.Fatalf("ready URL %q: %v", s.url, err)
	}
	s.port, _ = strconv.Atoi(p)
	return s
}

// serveSite writes the sample site under a temp dir, next to a secret file
// that must never be served, and starts a server on it.
func serveSite(t *testing.T) (*served, string) {
	t.Helper()
	parent := t.TempDir()
	dir := filepath.Join(parent, "view")
	if err := Write(dir, sampleSite()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "secret.txt"), []byte("TOPSECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	return startServe(t, dir, 0), dir
}

var client = &http.Client{
	Transport:     &http.Transport{DisableKeepAlives: true},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	Timeout:       5 * time.Second,
}

// do sends method with the raw request target (sent as is, not cleaned) and Host.
func do(t *testing.T, s *served, method, target, host string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, s.url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.URL.Opaque = target
	req.Host = host
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, string(b)
}

func get(t *testing.T, s *served, target string) (*http.Response, string) {
	t.Helper()
	return do(t, s, http.MethodGet, target, "127.0.0.1:"+strconv.Itoa(s.port))
}

func checkSecurityHeaders(t *testing.T, target string, resp *http.Response) {
	t.Helper()
	h := resp.Header
	if h.Get("Content-Security-Policy") != wantCSP {
		t.Errorf("%s: CSP %q", target, h.Get("Content-Security-Policy"))
	}
	if h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("%s: X-Content-Type-Options %q", target, h.Get("X-Content-Type-Options"))
	}
	if h.Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("%s: Referrer-Policy %q", target, h.Get("Referrer-Policy"))
	}
}

func TestServeServesIndexAndAssets(t *testing.T) {
	s, dir := serveSite(t)
	cases := []struct{ target, file, ctype string }{
		{"/", "index.html", "text/html; charset=utf-8"},
		{"/index.html", "index.html", "text/html; charset=utf-8"},
		{"/decisions.html", "decisions.html", "text/html; charset=utf-8"},
		{"/assets/app.css", "assets/app.css", "text/css; charset=utf-8"},
		{"/assets/app.js", "assets/app.js", "text/javascript; charset=utf-8"},
		{"/data.js", "data.js", "text/javascript; charset=utf-8"},
		{"/notes/n-aaaaaa.html", "notes/n-aaaaaa.html", "text/html; charset=utf-8"},
		{"/notes/n-aaaaaa.md", "notes/n-aaaaaa.md", "text/markdown; charset=utf-8"},
	}
	for _, c := range cases {
		resp, body := get(t, s, c.target)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", c.target, resp.StatusCode)
		}
		if body != readFile(t, dir, c.file) {
			t.Errorf("%s: body differs from %s", c.target, c.file)
		}
		if got := resp.Header.Get("Content-Type"); got != c.ctype {
			t.Errorf("%s: Content-Type %q, want %q", c.target, got, c.ctype)
		}
		checkSecurityHeaders(t, c.target, resp)
	}
	// localhost with the right port is accepted too, and HEAD works.
	resp, body := do(t, s, http.MethodHead, "/", "localhost:"+strconv.Itoa(s.port))
	if resp.StatusCode != http.StatusOK || body != "" {
		t.Fatalf("HEAD via localhost: status %d body %q", resp.StatusCode, body)
	}
}

func TestServeRejectsForeignHost(t *testing.T) {
	s, _ := serveSite(t)
	port := strconv.Itoa(s.port)
	other := strconv.Itoa(s.port + 1)
	for _, host := range []string{
		"evil.example", "evil.example:" + port, "127.0.0.1", "localhost",
		"127.0.0.1:" + other, "localhost:" + other, "localhost.:" + port, "LOCALHOST:" + port,
		"[::1]:" + port, "127.0.0.2:" + port, "0.0.0.0:" + port, "127.1:" + port,
		"localhost.evil.example:" + port, "127.0.0.1:" + port + ".evil.example",
	} {
		resp, body := do(t, s, http.MethodGet, "/", host)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Host %q: status %d, want 403", host, resp.StatusCode)
		}
		if strings.Contains(body, "tidepool") {
			t.Errorf("Host %q: page content served", host)
		}
		checkSecurityHeaders(t, host, resp)
	}
}

func TestServeHidesDotfiles(t *testing.T) {
	s, dir := serveSite(t)
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("DOTSECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes", ".hidden.md"), []byte("DOTSECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{
		"/.gitignore", "/%2egitignore", "/%2Egitignore", "/.git/config", "/.git/", "/notes/.hidden.md",
		"/notes/%2ehidden.md", "/./index.html", "/notes/./n-aaaaaa.html",
	} {
		resp, body := get(t, s, target)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", target, resp.StatusCode)
		}
		if strings.Contains(body, "DOTSECRET") || body == "*\n" {
			t.Errorf("%s: dotfile content served", target)
		}
		checkSecurityHeaders(t, target, resp)
	}
}

func TestServeBlocksTraversal(t *testing.T) {
	s, dir := serveSite(t)
	// A symlink inside the folder pointing outside it must not be followed out.
	if err := os.Symlink(filepath.Join(filepath.Dir(dir), "secret.txt"), filepath.Join(dir, "leak.html")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../secret.txt", filepath.Join(dir, "notes", "leak.md")); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{
		"/../../etc/passwd", "/../secret.txt", "/%2e%2e/", "/%2e%2e/secret.txt", "/%2E%2E/secret.txt",
		"/notes/../../secret.txt", "/notes/%2e%2e/%2e%2e/secret.txt", "/notes/..%2f..%2fsecret.txt",
		"/..%2fsecret.txt", "/..%5csecret.txt", "/..\\secret.txt", "/notes\\..\\..\\secret.txt",
		"/assets//app.css", "/notes//n-aaaaaa.html", "/leak.html", "/notes/leak.md", "/index.html%00",
		"/" + strings.TrimPrefix(filepath.Join(filepath.Dir(dir), "secret.txt"), "/"),
	} {
		resp, body := get(t, s, target)
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 404 or 400", target, resp.StatusCode)
		}
		if strings.Contains(body, "TOPSECRET") || strings.Contains(body, "root:") {
			t.Errorf("%s: file outside the folder served", target)
		}
	}
	// net/http sends an opaque target starting with "//" in absolute form, so
	// these go over a raw connection.
	for _, target := range []string{"//secret.txt", "//etc/passwd", "//index.html"} {
		status, body := rawGet(t, s, target)
		if !strings.HasPrefix(status, "HTTP/1.1 404") && !strings.HasPrefix(status, "HTTP/1.1 400") {
			t.Errorf("%s: status %q, want 404 or 400", target, status)
		}
		if strings.Contains(body, "TOPSECRET") || strings.Contains(body, "root:") || strings.Contains(body, "tidepool") {
			t.Errorf("%s: content served", target)
		}
	}
}

// rawGet sends a GET for target exactly as given over a plain TCP connection
// and returns the status line and the rest of the response.
func rawGet(t *testing.T, s *served, target string) (string, string) {
	t.Helper()
	addr := "127.0.0.1:" + strconv.Itoa(s.port)
	c, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(c, "GET "+target+" HTTP/1.1\r\nHost: "+addr+"\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(c)
	if err != nil {
		t.Fatal(err)
	}
	status, rest, _ := strings.Cut(string(b), "\r\n")
	return status, rest
}

func TestServeNoDirectoryListing(t *testing.T) {
	s, _ := serveSite(t)
	for _, target := range []string{"/notes/", "/notes", "/assets/", "/assets"} {
		resp, body := get(t, s, target)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", target, resp.StatusCode)
		}
		if strings.Contains(body, "n-aaaaaa") || strings.Contains(body, "app.css") {
			t.Errorf("%s: directory listed", target)
		}
	}
}

func TestServeRejectsPost(t *testing.T) {
	s, _ := serveSite(t)
	host := "127.0.0.1:" + strconv.Itoa(s.port)
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions, "TRACE"} {
		resp, body := do(t, s, m, "/", host)
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s: status %d, want 405", m, resp.StatusCode)
		}
		if strings.Contains(body, "tidepool") {
			t.Errorf("%s: page content served", m)
		}
		checkSecurityHeaders(t, m, resp)
	}
	resp, _ := do(t, s, http.MethodOptions, "*", host)
	if resp.StatusCode == http.StatusOK {
		t.Errorf("OPTIONS *: status 200")
	}
}

func TestServeBindsLoopbackOnly(t *testing.T) {
	s, _ := serveSite(t)
	if !strings.HasPrefix(s.url, "http://127.0.0.1:") || !strings.HasSuffix(s.url, "/") || s.port == 0 {
		t.Fatalf("ready URL %q", s.url)
	}
	// Not reachable on the IPv6 loopback.
	if c, err := net.DialTimeout("tcp", net.JoinHostPort("::1", strconv.Itoa(s.port)), time.Second); err == nil {
		c.Close()
		t.Error("server reachable on [::1]")
	}
}

func TestServeStopsOnContextCancel(t *testing.T) {
	dir := writeSite(t, sampleSite())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	urls := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, dir, 0, func(u string) { urls <- u }) }()
	var u string
	select {
	case u = <-urls:
	case <-time.After(5 * time.Second):
		t.Fatal("never ready")
	}
	resp, err := client.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve returned %v, want nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}
	if c, err := net.DialTimeout("tcp", strings.TrimSuffix(strings.TrimPrefix(u, "http://"), "/"), time.Second); err == nil {
		c.Close()
		t.Fatal("still listening after cancel")
	}
}

func TestServePortInUse(t *testing.T) {
	s, dir := serveSite(t)
	err := Serve(context.Background(), dir, s.port, func(string) { t.Error("ready called on a port in use") })
	if err == nil {
		t.Fatal("second Serve on the same port returned nil")
	}
}

func TestServeMissingDir(t *testing.T) {
	err := Serve(context.Background(), filepath.Join(t.TempDir(), "absent"), 0, func(string) { t.Error("ready called") })
	if err == nil {
		t.Fatal("Serve on a missing folder returned nil")
	}
}
