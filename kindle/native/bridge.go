package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type youtubeVideo struct {
	Title, Artist                    string
	Art                              string // ID of the cover the PC has ready, or ""
	Source                           string // "music" for YouTube Music
	Playing, Captions, Repeat, Muted bool
	Duration, Position               int
	Volume                           *int
}

type bridgeReply struct {
	YouTube      *youtubeVideo
	Stats        *pcStats
	Cover        *coverArt `json:"-"`
	Weather      *weatherInfo
	BrowserReady bool `json:"browser_ready"`
	OK           bool
	Error        string
}

type bridgeAction struct {
	Name   string `json:"name,omitempty"`
	Action string `json:"action,omitempty"`
	Value  int    `json:"value"`
	Text   string `json:"text,omitempty"`
}

type bridgeConfig struct {
	Addresses []string
	Port      int
	Token     string
}

type pcBridge struct {
	config bridgeConfig
	client *http.Client
	active string
	cover  *coverArt
}

func readBridge(folder string) (*pcBridge, error) {
	raw, err := os.ReadFile(filepath.Join(folder, "pc-bridge.json"))
	if err != nil {
		return nil, err
	}
	var config bridgeConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	if len(config.Addresses) == 0 || config.Port < 1 || config.Port > 65535 || len(config.Token) < 32 {
		return nil, errors.New("incomplete PC pairing")
	}
	for _, candidate := range config.Addresses {
		ip := net.ParseIP(candidate)
		if ip == nil || (!ip.IsPrivate() && !ip.IsLoopback()) {
			return nil, errors.New("the PC address must be local")
		}
	}
	cert, err := os.ReadFile(filepath.Join(folder, "pc-bridge.pem"))
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(cert) {
		return nil, errors.New("invalid PC certificate")
	}
	transport := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 850 * time.Millisecond}).DialContext,
		TLSHandshakeTimeout: 1500 * time.Millisecond,
		TLSClientConfig:     &tls.Config{RootCAs: roots, ServerName: "kindle-dashboard.local", MinVersion: tls.VersionTLS12},
		MaxIdleConns:        4,
		IdleConnTimeout:     20 * time.Second,
	}
	client := &http.Client{
		Transport:     transport,
		Timeout:       6 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return &pcBridge{config: config, client: client}, nil
}

func (b *pcBridge) request(ctx context.Context, host, path string, action *bridgeAction) (bridgeReply, error) {
	method := http.MethodGet
	var body io.Reader
	if action != nil {
		method = http.MethodPost
		encoded, err := json.Marshal(action)
		if err != nil {
			return bridgeReply{}, err
		}
		body = bytes.NewReader(encoded)
	}
	url := "https://" + net.JoinHostPort(host, strconv.Itoa(b.config.Port)) + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return bridgeReply{}, err
	}
	req.Header.Set("Authorization", "Bearer "+b.config.Token)
	if action != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	reply, err := b.client.Do(req)
	if err != nil {
		return bridgeReply{}, err
	}
	defer reply.Body.Close()
	var decoded bridgeReply
	if err := json.NewDecoder(io.LimitReader(reply.Body, 12000)).Decode(&decoded); err != nil {
		return bridgeReply{}, err
	}
	if reply.StatusCode != http.StatusOK && decoded.Error == "" {
		return bridgeReply{}, fmt.Errorf("PC companion returned HTTP %d", reply.StatusCode)
	}
	return decoded, nil
}

func (b *pcBridge) poll(ctx context.Context) (bridgeReply, error) {
	addresses := b.config.Addresses
	if b.active != "" {
		addresses = append([]string{b.active}, addresses...)
	}
	visited := make(map[string]bool)
	for _, address := range addresses {
		if visited[address] {
			continue
		}
		visited[address] = true
		result, err := b.request(ctx, address, "/status", nil)
		if err == nil && result.Error == "" {
			b.active = address
			b.attachCover(ctx, &result)
			return result, nil
		}
	}
	b.active = ""
	return bridgeReply{}, errors.New("PC companion unavailable")
}

func (b *pcBridge) send(ctx context.Context, action bridgeAction) (bridgeReply, error) {
	if b.active == "" {
		if _, err := b.poll(ctx); err != nil {
			return bridgeReply{}, err
		}
	}
	path := "/youtube"
	if action.Name != "" {
		path = "/launch"
	}
	// A timed-out action may already have run. Do not retry it.
	reply, err := b.request(ctx, b.active, path, &action)
	if err != nil {
		b.active = ""
		return reply, err
	}
	b.attachCover(ctx, &reply)
	return reply, nil
}

// attachCover adds the album cover to a reply, downloading it from the PC when the song changes.
func (b *pcBridge) attachCover(ctx context.Context, reply *bridgeReply) {
	video := reply.YouTube
	if video == nil || video.Art == "" {
		return
	}
	if b.cover == nil || b.cover.ID != video.Art {
		art, err := b.fetchCover(ctx, video.Art)
		if err != nil {
			return // try again on the next poll
		}
		b.cover = art
	}
	reply.Cover = b.cover
}

func (b *pcBridge) fetchCover(ctx context.Context, id string) (*coverArt, error) {
	url := "https://" + net.JoinHostPort(b.active, strconv.Itoa(b.config.Port)) + "/art?id=" + id
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+b.config.Token)
	reply, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer reply.Body.Close()
	if reply.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PC companion returned HTTP %d for the cover", reply.StatusCode)
	}
	source, _, err := image.Decode(io.LimitReader(reply.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	return newCoverArt(id, source), nil
}

type bridgeUpdate struct {
	action *bridgeAction
	reply  bridgeReply
	err    error
}

func (b *pcBridge) run(ctx context.Context, commands <-chan bridgeAction, updates chan<- bridgeUpdate) {
	// Poll often enough for the PC performance monitor to feel live.
	timer := time.NewTicker(5 * time.Second)
	defer timer.Stop()
	deliver := func(update bridgeUpdate) {
		select {
		case updates <- update:
		case <-ctx.Done():
		}
	}
	reply, err := b.poll(ctx)
	deliver(bridgeUpdate{reply: reply, err: err})
	for {
		select {
		case <-ctx.Done():
			return
		case command := <-commands:
			reply, err := b.send(ctx, command)
			deliver(bridgeUpdate{action: &command, reply: reply, err: err})
		case <-timer.C:
			reply, err := b.poll(ctx)
			deliver(bridgeUpdate{reply: reply, err: err})
		}
	}
}
