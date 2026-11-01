package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Where releases are published; tests point these at a local server.
var (
	updateBase   = "https://github.com/R0b-1ns/cairn/releases"
	updateClient = &http.Client{Timeout: 2 * time.Minute}
	updateExe    = os.Executable
)

const (
	maxBinarySize = 200 << 20
	maxSumsSize   = 1 << 20
)

var releaseTag = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$`)

func init() {
	register(Command{Name: "update", Usage: "[flags]", Summary: "replace this binary with the latest release, after checking its SHA-256", Define: defineUpdate})
}

func defineUpdate(fs *flag.FlagSet) Handler {
	var check bool
	var version string
	fs.BoolVar(&check, "check", false, "only report whether a newer release exists; install nothing")
	fs.StringVar(&version, "version", "", "install this release tag, for example v1.0.1, instead of the latest")
	return func(c *Ctx, args []string) int {
		if len(args) != 0 {
			return fail(c, ExitUsage, "update takes no arguments")
		}
		if Version == "dev" {
			return fail(c, ExitRuntime, "update: this is a development build; reinstall with scripts/install.sh or 'go install'")
		}
		tag := version
		if tag != "" && !releaseTag.MatchString(tag) {
			return fail(c, ExitUsage, "--version must look like v1.2.3")
		}
		if tag == "" {
			var err error
			if tag, err = latestTag(); err != nil {
				return fail(c, ExitRuntime, "update: %v", err)
			}
		}
		tag = "v" + strings.TrimPrefix(tag, "v")
		current := strings.TrimPrefix(Version, "v")
		if strings.TrimPrefix(tag, "v") == current {
			fmt.Fprintf(c.Stdout, "cairn %s is up to date\n", current)
			return ExitOK
		}
		if check {
			fmt.Fprintf(c.Stdout, "cairn %s is available (installed: %s); run 'cairn update'\n", strings.TrimPrefix(tag, "v"), current)
			return ExitOK
		}
		if err := installRelease(tag); err != nil {
			return fail(c, ExitRuntime, "update: %v", err)
		}
		fmt.Fprintf(c.Stdout, "updated cairn %s -> %s\n", current, strings.TrimPrefix(tag, "v"))
		return ExitOK
	}
}

// latestTag reads the tag the "latest release" URL redirects to.
func latestTag() (string, error) {
	client := *updateClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Get(updateBase + "/latest")
	if err != nil {
		return "", fmt.Errorf("cannot reach the release page: %w", err)
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	if resp.StatusCode < 300 || resp.StatusCode > 399 || loc == "" {
		return "", fmt.Errorf("cannot find the latest release (HTTP %d)", resp.StatusCode)
	}
	tag := path.Base(strings.TrimRight(loc, "/"))
	if !releaseTag.MatchString(tag) {
		return "", fmt.Errorf("unexpected release tag %q", tag)
	}
	return tag, nil
}

func installRelease(tag string) error {
	if (runtime.GOOS != "darwin" && runtime.GOOS != "linux") || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		return fmt.Errorf("no release for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	asset := "cairn-" + runtime.GOOS + "-" + runtime.GOARCH
	base := updateBase + "/download/" + tag

	exe, err := updateExe()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	// The temporary file sits next to the binary so the final rename is atomic.
	tmp, err := os.CreateTemp(filepath.Dir(exe), ".cairn-update-*")
	if err != nil {
		return fmt.Errorf("cannot write next to %s (%v); rerun with sudo or reinstall with scripts/install.sh", exe, err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	sums, err := fetch(base+"/SHA256SUMS", maxSumsSize, nil)
	if err != nil {
		return err
	}
	var want string
	for _, line := range strings.Split(string(sums), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[1] == asset {
			want = f[0]
		}
	}
	if want == "" {
		return fmt.Errorf("%s is not listed in SHA256SUMS of %s", asset, tag)
	}
	h := sha256.New()
	if _, err := fetch(base+"/"+asset, maxBinarySize, io.MultiWriter(tmp, h)); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("checksum mismatch for %s (expected %s, got %s); nothing was changed", asset, want, got)
	}
	if err := tmp.Chmod(0o755); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), exe)
}

// fetch downloads url. With a sink it streams the body there and returns nil;
// without one it returns the body.
func fetch(url string, limit int64, sink io.Writer) ([]byte, error) {
	resp, err := updateClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("cannot download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot download %s (HTTP %d)", url, resp.StatusCode)
	}
	body := io.LimitReader(resp.Body, limit+1)
	if sink == nil {
		b, err := io.ReadAll(body)
		if err == nil && int64(len(b)) > limit {
			err = errors.New("too large")
		}
		if err != nil {
			return nil, fmt.Errorf("cannot download %s: %w", url, err)
		}
		return b, nil
	}
	n, err := io.Copy(sink, body)
	if err == nil && n > limit {
		err = errors.New("too large")
	}
	if err != nil {
		return nil, fmt.Errorf("cannot download %s: %w", url, err)
	}
	return nil, nil
}
