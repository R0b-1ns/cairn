package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/R0b-1ns/cairn/internal/model"
	"github.com/R0b-1ns/cairn/internal/view"
)

func init() {
	register(Command{Name: "view", Usage: "[flags]", Summary: "write a read-only website of the notes, to browse and filter them", Define: defineView})
}

// openBrowser opens url in the default browser; tests replace it.
var openBrowser = defaultOpenBrowser

func defaultOpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reap the launcher without blocking the server
	return nil
}

// serveContext is the lifetime of `view --serve`: until Ctrl-C or SIGTERM. Tests replace it.
var serveContext = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

type viewResult struct {
	Dir   string `json:"dir"`
	Index string `json:"index"`
	URL   string `json:"url"`
}

func defineView(fs *flag.FlagSet) Handler {
	var f common
	f.projectFlag(fs)
	f.jsonFlag(fs)
	var serve, noOpen bool
	var port int
	fs.BoolVar(&serve, "serve", false, "also serve the site on http://127.0.0.1 until Ctrl-C")
	fs.IntVar(&port, "port", 0, "with --serve: the port to listen on (0 picks a free port)")
	fs.BoolVar(&noOpen, "no-open", false, "with --serve: do not open the browser")
	return func(c *Ctx, args []string) int {
		if len(args) != 0 {
			return fail(c, ExitUsage, "view takes no arguments")
		}
		set := map[string]bool{}
		fs.Visit(func(fl *flag.Flag) { set[fl.Name] = true })
		if !serve && (set["port"] || set["no-open"]) {
			return fail(c, ExitUsage, "--port and --no-open need --serve")
		}
		if port < 0 || port > 65535 {
			return fail(c, ExitUsage, "--port must be between 0 and 65535")
		}
		s, code := openProject(c, f.project)
		if s == nil {
			return code
		}
		open := true
		defer func() {
			if open {
				s.Close()
			}
		}()
		site, err := view.Collect(s.St, s.Proj, timestamp())
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		site.MarkdownHeader = renderMarkdownHeader(s.Proj)
		for i := range site.Notes {
			nv := &site.Notes[i]
			doc := exportDoc{Project: s.Proj, Notes: []model.Note{nv.Note}, Addenda: nv.Addenda}
			if nv.Markdown, err = renderMarkdownNotes(s, doc); err != nil {
				return fail(c, ExitRuntime, "%v", err)
			}
		}
		dir := filepath.Join(filepath.Dir(s.St.Path()), "view")
		if err := view.Write(dir, site); err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		res := viewResult{Dir: dir, Index: filepath.Join(dir, "index.html")}
		if !f.json {
			fmt.Fprintf(c.Stdout, "wrote %s\nopen %s\n", res.Dir, res.Index)
		}
		if !serve {
			if f.json {
				printJSON(c, res)
			}
			return ExitOK
		}
		// The database is not needed while serving the written files.
		s.Close()
		open = false
		ctx, cancel := serveContext()
		defer cancel()
		err = view.Serve(ctx, dir, port, func(url string) {
			if f.json {
				res.URL = url
				printJSON(c, res)
			} else {
				fmt.Fprintf(c.Stdout, "serving %s (press Ctrl-C to stop)\n", url)
			}
			if !noOpen {
				if err := openBrowser(url); err != nil {
					fmt.Fprintf(c.Stderr, "cairn: warning: could not open the browser: %v\n", err)
				}
			}
		})
		if err != nil {
			return fail(c, ExitRuntime, "%v", err)
		}
		return ExitOK
	}
}
