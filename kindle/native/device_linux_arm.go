package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const evIOCGrab = 0x40044590

func readAxis(file *os.File, code uint16) (axisRange, bool) {
	var data [6]int32
	request := uintptr(0x80184540) + uintptr(code)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), request, uintptr(unsafe.Pointer(&data)))
	return axisRange{int(data[1]), int(data[2])}, errno == 0 && data[2] > data[1]
}

func openTouch() (*os.File, touchDecoder, error) {
	devices, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, touchDecoder{}, err
	}
	for _, path := range devices {
		nameFile := filepath.Join("/sys/class/input", filepath.Base(path), "device", "name")
		name, _ := os.ReadFile(nameFile)
		label := strings.ToLower(string(name))
		if path != "/dev/input/event2" && !strings.Contains(label, "touch") && !strings.Contains(label, "cyttsp") {
			continue
		}
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		x, hasX := readAxis(file, absMtX)
		y, hasY := readAxis(file, absMtY)
		multiTouch := hasX && hasY
		if !multiTouch {
			x, hasX = readAxis(file, absX)
			y, hasY = readAxis(file, absY)
		}
		if !hasX || !hasY {
			file.Close()
			continue
		}
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), evIOCGrab, 1)
		if errno != 0 {
			file.Close()
			return nil, touchDecoder{}, fmt.Errorf("cannot take control of touchscreen %s: %w", path, errno)
		}
		log.Printf("Touchscreen %s (%s), X=%v Y=%v MT=%v", path, strings.TrimSpace(string(name)), x, y, multiTouch)
		return file, touchDecoder{multiTouch: multiTouch, horizontal: x, vertical: y, primary: -1}, nil
	}
	return nil, touchDecoder{}, errors.New("Kindle touchscreen is unavailable")
}

type touchResult struct {
	point image.Point
	err   error
}

func readTouch(ctx context.Context, file *os.File, decoder touchDecoder, results chan<- touchResult) {
	for {
		// struct input_event uses two 32-bit time fields on the KT4 kernel.
		var raw [16]byte
		_, err := io.ReadFull(file, raw[:])
		result := touchResult{err: err}
		if err == nil {
			kind := binary.LittleEndian.Uint16(raw[8:10])
			code := binary.LittleEndian.Uint16(raw[10:12])
			value := int(int32(binary.LittleEndian.Uint32(raw[12:16])))
			point, ok := decoder.decode(kind, code, value)
			if !ok {
				continue
			}
			result.point = point
		}
		select {
		case results <- result:
		case <-ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

func fbinkPath() (string, error) {
	for _, path := range []string{"/mnt/us/libkh/bin/fbink", "/var/local/kmc/bin/fbink", "/usr/bin/fbink", "/mnt/us/koreader/fbink"} {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return path, nil
		}
	}
	return exec.LookPath("fbink")
}

func paint(ctx context.Context, fbink, path string, frame image.Image, flash bool) error {
	if err := writePNG(path, frame); err != nil {
		return err
	}
	args := []string{"-q", "-g", "file=" + path}
	if flash {
		args = append(args, "-f")
	}
	drawContext, stop := context.WithTimeout(ctx, 4*time.Second)
	defer stop()
	output, err := exec.CommandContext(drawContext, fbink, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("FBInk could not draw the dashboard: %w %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// screenViewport asks FBInk for the real screen size so the dashboard fills
// larger screens such as the Paperwhite's 1072 × 1448 panel.
func screenViewport(ctx context.Context, fbink string) viewport {
	probe, stop := context.WithTimeout(ctx, 4*time.Second)
	defer stop()
	output, err := exec.CommandContext(probe, fbink, "-e").CombinedOutput()
	if w, h, ok := parseFBInkState(string(output)); err == nil && ok {
		log.Printf("Screen %dx%d", w, h)
		return newViewport(w, h)
	}
	log.Printf("Could not read the screen size (%v); assuming 600x800", err)
	return newViewport(width, height)
}

func saveTheme(path, theme string) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".dashboard-theme-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.WriteString(theme + "\n")
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}

func exitOnSleep(ctx context.Context, stop context.CancelFunc) {
	command := exec.CommandContext(ctx, "lipc-wait-event", "com.lab126.powerd", "goingToScreenSaver")
	output, err := command.Output()
	if err == nil && strings.Contains(string(output), "goingToScreenSaver") {
		stop()
	}
}

func runDevice(art *artwork, model *dashboard, preference string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	fbink, err := fbinkPath()
	if err != nil {
		return fmt.Errorf("FBInk from the Kindle scriptlet installation is unavailable: %w", err)
	}
	working, err := os.MkdirTemp("/tmp", "personal-dashboard-")
	if err != nil {
		return err
	}
	defer os.Remove(working)
	framePath := filepath.Join(working, "screen.png")
	defer os.Remove(framePath)
	file, decoder, err := openTouch()
	if err != nil {
		return err
	}
	// Closing the evdev descriptor releases the touch grab even on errors.
	defer file.Close()
	go exitOnSleep(ctx, stop)
	results := make(chan touchResult, 1)
	go readTouch(ctx, file, decoder, results)

	model.connectedUI = true
	commands := make(chan bridgeAction, 1)
	updates := make(chan bridgeUpdate, 1)
	bridge, pairingErr := readBridge(filepath.Dir(preference))
	if pairingErr != nil {
		log.Printf("PC pairing unavailable: %v", pairingErr)
	} else {
		go bridge.run(ctx, commands, updates)
	}

	view := screenViewport(ctx, fbink)
	if err := paint(ctx, fbink, framePath, view.fit(art.render(model, time.Now()), model.theme.paper), true); err != nil {
		return err
	}
	// Repaint at the start of every minute so the time and date stay current.
	minute := time.NewTimer(time.Until(time.Now().Truncate(time.Minute).Add(time.Minute)))
	defer minute.Stop()
	for {
		flash := false
		select {
		case <-ctx.Done():
			return nil
		case <-minute.C:
			minute.Reset(time.Until(time.Now().Truncate(time.Minute).Add(time.Minute)))
		case update := <-updates:
			wasOnline, wasBrowserReady := model.pcOnline, model.browserReady
			model.pcOnline = update.err == nil
			if update.action == nil {
				model.browserReady = update.reply.BrowserReady
				if wasOnline == model.pcOnline && wasBrowserReady == model.browserReady && reflect.DeepEqual(model.youtube, update.reply.YouTube) && reflect.DeepEqual(model.stats, update.reply.Stats) && reflect.DeepEqual(model.todo, update.reply.Todo) {
					continue
				}
				model.youtube = update.reply.YouTube
				model.stats = update.reply.Stats
				model.todo = update.reply.Todo
			} else if update.err != nil {
				model.show("PC OFFLINE", "Connect the Kindle and PC to the same local network and start the PC companion. Then try again.")
			} else if update.reply.Error != "" {
				model.show("PC ACTION", update.reply.Error)
			} else if update.action.Action == "todo-toggle" {
				if update.reply.Todo != nil {
					model.todo = update.reply.Todo
				}
			} else if update.action.Name == "" {
				model.browserReady = update.reply.BrowserReady
				model.youtube = update.reply.YouTube
			}
		case tap := <-results:
			if tap.err != nil {
				return tap.err
			}
			var exit bool
			point := view.fromTouch(tap.point)
			log.Printf("Tap at %v (panel %v)", point, tap.point)
			exit, flash = model.tap(point, time.Now())
			if exit {
				return nil
			}
			if flash {
				if err := saveTheme(preference, model.theme.name); err != nil {
					log.Printf("Could not save theme: %v", err)
				}
			}
			if action := model.pending; action != nil {
				model.pending = nil
				if bridge == nil {
					model.show("PC PAIRING", "The pairing files are missing or unreadable. Reconnect to the PC to check the installation.")
				} else {
					select {
					case commands <- *action:
						continue
					default:
						model.show("PC BUSY", "Wait for the previous command to finish.")
					}
				}
			}
		}
		if err := paint(ctx, fbink, framePath, view.fit(art.render(model, time.Now()), model.theme.paper), flash); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
	}
}
