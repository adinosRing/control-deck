package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

const (
	address       = "127.0.0.1:8080"
	healthMarker  = "personal-dashboard-v1\n"
	sessionHeader = "X-Dashboard-Session"
)

var assets = map[string]string{
	"/dashboard.css":         "text/css; charset=utf-8",
	"/dashboard.js":          "text/javascript; charset=utf-8",
	"/assets/assistant.webp": "image/webp",
	"/assets/pixelify.ttf":   "font/ttf",
	"/assets/plex-mono.ttf":  "font/ttf",
}

func main() {
	dir := flag.String("dir", "web", "directory containing the dashboard")
	kindle := flag.Bool("kindle", false, "return to Kindle Home on dashboard exit")
	check := flag.Bool("check", false, "check if this dashboard is running")
	stop := flag.Bool("stop", false, "stop this dashboard")
	flag.Parse()

	var err error
	switch {
	case *check:
		err = checkServer()
	case *stop:
		err = stopServer()
	default:
		err = serve(*dir, *kindle)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func checkServer() error {
	resp, err := localRequest("GET", "/_health", "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 128))
	if err != nil || resp.StatusCode != http.StatusOK || string(body) != healthMarker {
		return errors.New("the dashboard is not listening on " + address)
	}
	return nil
}

func stopServer() error {
	if err := checkServer(); err != nil {
		return err
	}
	index, err := localRequest("GET", "/", "")
	if err != nil {
		return err
	}
	defer index.Body.Close()
	body, err := io.ReadAll(io.LimitReader(index.Body, 65536))
	match := regexp.MustCompile(`name="dashboard-session" content="([0-9a-f]{48})"`).FindSubmatch(body)
	if err != nil || index.StatusCode != http.StatusOK || len(match) != 2 {
		return errors.New("could not read the dashboard session")
	}

	resp, err := localRequest("POST", "/_exit", string(match[1]))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("dashboard exit returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func localRequest(method, path, session string) (*http.Response, error) {
	request, err := http.NewRequest(method, "http://"+address+path, nil)
	if err != nil {
		return nil, err
	}
	if session != "" {
		request.Header.Set(sessionHeader, session)
	}
	client := http.Client{
		Timeout:       800 * time.Millisecond,
		Transport:     &http.Transport{DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return client.Do(request)
}

func serve(dir string, kindle bool) error {
	index, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		return err
	}
	const placeholder = `content="DASHBOARD_SESSION"`
	if !bytes.Contains(index, []byte(placeholder)) {
		return errors.New("index.html is missing its dashboard session placeholder")
	}
	key := make([]byte, 24)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	session := hex.EncodeToString(key)
	index = bytes.Replace(index, []byte(placeholder), []byte(`content="`+session+`"`), 1)

	listener, err := net.Listen("tcp4", address)
	if err != nil {
		return err
	}
	defer listener.Close()

	exit := make(chan struct{})
	var requestExit sync.Once
	server := &http.Server{
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       6 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       10 * time.Second,
		MaxHeaderBytes:    8192,
	}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != address && r.Host != "localhost:8080" {
			http.Error(w, "This dashboard runs on localhost.", http.StatusForbidden)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; script-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")

		if r.URL.Path == "/_exit" {
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "POST")
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			origin := r.Header.Get("Origin")
			if (origin != "" && origin != "http://"+r.Host) ||
				subtle.ConstantTimeCompare([]byte(r.Header.Get(sessionHeader)), []byte(session)) != 1 {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			requestExit.Do(func() { close(exit) })
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/_health":
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(w, healthMarker)
		case "/", "/index.html":
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
		case "/favicon.ico":
			w.WriteHeader(http.StatusNoContent)
		default:
			contentType, ok := assets[r.URL.Path]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, filepath.Join(dir, filepath.FromSlash(r.URL.Path[1:])))
		}
	})

	stopped := make(chan struct{})
	go func() {
		<-exit
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Print(err)
		}
		close(stopped)
	}()

	log.Printf("Personal Dashboard listening on http://%s/", address)
	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-stopped

	if kindle {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "lipc-set-prop", "com.lab126.appmgrd", "start", "app://com.lab126.booklet.home")
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Could not open Kindle Home: %v %s", err, output)
		}
	}
	return nil
}
