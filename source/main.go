package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const addonVersion = "0.3.9"

type Preferences struct {
	Mode      string `json:"mode"`
	Port      int    `json:"port"`
	Highlight bool   `json:"highlight"`
}
type App struct {
	mu                              sync.Mutex
	tel                             Telemetry
	prefs                           Preferences
	data, assets                    string
	launcherData                    string
	displayCache                    string
	enabled, gameKnown, gameRunning bool
	listenErr                       string
	conn                            *net.UDPConn
	writer                          sync.Mutex
	out                             io.Writer
	expanded                        int
	expandedSet                     bool
	expandedUID                     uint64
	profileName                     string
	savedPort                       int
	rec                             *Recorder
	recordingErr                    string
	analysis                        AnalysisSelection
	done                            chan struct{}
	teamWake                        chan struct{}
	teamAttempt                     time.Time
	teamAttemptID                   string
}

func (a *App) send(v any) {
	a.writer.Lock()
	defer a.writer.Unlock()
	_ = json.NewEncoder(a.out).Encode(v)
}
func (a *App) save() error {
	b, err := json.MarshalIndent(a.prefs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.data, "settings.json"), b, 0644)
}
func (a *App) listen() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn != nil {
		_ = a.conn.Close()
		a.conn = nil
	}
	a.listenErr = ""
	if !a.enabled {
		a.tel.stop()
		if a.rec != nil {
			a.rec.finish("interrupted")
		}
		return
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: a.prefs.Port})
	if err != nil {
		a.listenErr = fmt.Sprintf("Cannot listen on UDP port %d: %v. Close another receiver using this port, or choose another port in the addon's launcher settings and in the game.", a.prefs.Port, err)
		return
	}
	a.conn = conn
	_ = conn.SetReadBuffer(4 * 1024 * 1024)
	go func() {
		b := make([]byte, 65535)
		for {
			n, _, err := conn.ReadFromUDP(b)
			if err != nil {
				return
			}
			a.mu.Lock()
			if a.conn == conn && a.enabled {
				a.tel.ingest(b[:n])
				if a.rec != nil {
					a.rec.ingest(b[:n], a.profileName)
					if n > 6 && b[6] == 4 && a.teamWake != nil {
						select {
						case a.teamWake <- struct{}{}:
						default:
						}
					}
				}
			}
			a.mu.Unlock()
		}
	}()
}
func (a *App) close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.done != nil {
		close(a.done)
		a.done = nil
	}
	if a.conn != nil {
		_ = a.conn.Close()
		a.conn = nil
	}
	if a.rec != nil {
		a.rec.finish("interrupted")
	}
}

type Message struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func (a *App) gameStatus(scan *bufio.Scanner, hostID *int) {
	*hostID++
	id := *hostID
	a.send(M{"id": id, "method": "game_status", "params": nil})
	// The launcher serialises requests, so only this host-call reply can arrive
	// until the current view/tick request has been answered.
	for scan.Scan() {
		var msg Message
		if json.Unmarshal(scan.Bytes(), &msg) != nil {
			continue
		}
		if msg.Method != "" { // profile notifications may arrive before a reply
			if msg.Method == "profile" {
				a.profile(msg.Params)
			}
			continue
		}
		var replyID int
		if json.Unmarshal(msg.ID, &replyID) != nil || replyID != id {
			continue
		}
		var status struct {
			Running *bool `json:"running"`
		}
		if json.Unmarshal(msg.Result, &status) == nil && status.Running != nil {
			a.mu.Lock()
			wasRunning := a.gameKnown && a.gameRunning
			a.gameKnown, a.gameRunning = true, *status.Running
			if !a.gameRunning && wasRunning {
				a.tel.stop()
				if a.rec != nil {
					a.rec.finish("interrupted")
				}
			} else if a.gameRunning && !wasRunning && a.tel.Ended {
				a.tel.reset(0, 0)
			}
			a.mu.Unlock()
		}
		return
	}
}
func (a *App) profile(raw json.RawMessage) {
	var p struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
		Vanilla bool   `json:"vanilla"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return
	}
	a.mu.Lock()
	if a.profileName != p.Name && a.rec != nil {
		a.rec.finish("interrupted")
	}
	a.profileName = p.Name
	enabled := p.Enabled && !p.Vanilla
	changed := a.enabled != enabled
	a.enabled = enabled
	if changed {
		a.tel.reset(0, 0)
	}
	a.mu.Unlock()
	if changed {
		a.listen()
	}
}

// Launcher 1.9.42 exposes native option controls on the Addons panel, but its
// helper protocol only supplies options during builds. Read the active profile's
// persisted options for live display changes; never write the profile ourselves.
func (a *App) syncOptions() {
	a.mu.Lock()
	name, data := a.profileName, a.launcherData
	a.mu.Unlock()
	if name == "" || len(name) > 40 || data == "" {
		return
	}
	for _, ch := range name {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
			return
		}
	}
	path := filepath.Join(filepath.Dir(filepath.Dir(data)), "profiles", name+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var profile struct {
		Options map[string]map[string]json.RawMessage `json:"options"`
	}
	if json.Unmarshal(b, &profile) != nil {
		return
	}
	options := profile.Options["unorthodoxstonendmodding.udp-telemetry"]
	a.mu.Lock()
	mode, highlight, port := "nearest", false, a.savedPort
	if port < 1024 || port > 65535 {
		port = 20777
	}
	if raw, ok := options["drivers_shown"]; ok {
		var v string
		if json.Unmarshal(raw, &v) == nil && (v == "nearest" || v == "full" || v == "none") {
			mode = v
		}
	}
	if raw, ok := options["highlight_player"]; ok {
		_ = json.Unmarshal(raw, &highlight)
	}
	if raw, ok := options["udp_port"]; ok {
		var v string
		if json.Unmarshal(raw, &v) == nil && v != "saved" {
			if n, e := strconv.Atoi(v); e == nil && n >= 1024 && n <= 65535 {
				port = n
			}
		}
	}
	changedPort := a.prefs.Port != port
	a.prefs.Mode, a.prefs.Highlight, a.prefs.Port = mode, highlight, port
	if changedPort {
		a.tel.reset(0, 0)
	}
	a.mu.Unlock()
	if changedPort {
		a.listen()
	}
}
func (a *App) event(raw json.RawMessage) (any, error) {
	if handled, err := a.analysisEvent(raw); handled || err != nil {
		return "reload", err
	}
	var p struct {
		Event struct {
			Click struct {
				ID string `json:"id"`
			} `json:"click"`
			Submit struct {
				Form   string                     `json:"form"`
				Row    string                     `json:"row"`
				Values map[string]json.RawMessage `json:"values"`
			} `json:"submit"`
		} `json:"event"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	if p.Event.Click.ID == "retry" {
		a.listen()
		return "reload", nil
	}
	if p.Event.Submit.Form == "drivers" || strings.HasPrefix(p.Event.Submit.Form, "drivers-after-") {
		var slot int
		if _, err := fmt.Sscanf(p.Event.Submit.Row, "car-%d", &slot); err != nil || slot < 0 || slot >= 24 {
			return nil, fmt.Errorf("unknown driver")
		}
		var open bool
		if err := json.Unmarshal(p.Event.Submit.Values["setup"], &open); err != nil {
			return nil, err
		}
		a.mu.Lock()
		a.expanded, a.expandedSet, a.expandedUID = slot, open, a.tel.UID
		a.mu.Unlock()
		return "reload", nil
	}
	if p.Event.Submit.Form == "display" {
		a.mu.Lock()
		old := a.prefs
		next := old
		if raw, ok := p.Event.Submit.Values["highlight"]; ok {
			if json.Unmarshal(raw, &next.Highlight) != nil {
				a.mu.Unlock()
				return nil, fmt.Errorf("highlight must be true or false")
			}
		}
		if raw, ok := p.Event.Submit.Values["mode"]; ok {
			if json.Unmarshal(raw, &next.Mode) != nil || (next.Mode != "nearest" && next.Mode != "full" && next.Mode != "none") {
				a.mu.Unlock()
				return nil, fmt.Errorf("unknown display mode")
			}
		}
		if raw, ok := p.Event.Submit.Values["port"]; ok {
			if json.Unmarshal(raw, &next.Port) != nil || next.Port < 1024 || next.Port > 65535 {
				a.mu.Unlock()
				return nil, fmt.Errorf("UDP port must be between 1024 and 65535")
			}
		}
		a.prefs = next
		err := a.save()
		if err != nil {
			a.prefs = old
			a.mu.Unlock()
			return nil, err
		}
		if next.Port != old.Port {
			a.tel.reset(0, 0)
		}
		a.mu.Unlock()
		if next.Port != old.Port {
			a.listen()
		}
		return "reload", nil
	}
	return "none", nil
}

func run(in io.Reader, out io.Writer) error {
	a := &App{out: out, prefs: Preferences{Mode: "nearest", Port: 20777}}
	defer a.close()
	scan := bufio.NewScanner(in)
	scan.Buffer(make([]byte, 4096), 4*1024*1024)
	hostID := 100000
	ready := false
	for scan.Scan() {
		var msg Message
		if err := json.Unmarshal(scan.Bytes(), &msg); err != nil {
			fmt.Fprintln(os.Stderr, "Invalid launcher JSON:", err)
			continue
		}
		var result any
		var err error
		switch msg.Method {
		case "hello":
			var p struct {
				API     int             `json:"api"`
				Data    string          `json:"data_dir"`
				Cache   string          `json:"cache_dir"`
				Profile json.RawMessage `json:"profile"`
			}
			if err = json.Unmarshal(msg.Params, &p); err == nil {
				if p.API != 1 || p.Data == "" {
					err = fmt.Errorf("launcher API 1 and data_dir are required")
					break
				}
				a.launcherData = p.Data
				cache := p.Cache
				if cache == "" {
					cache = p.Data
				}
				a.displayCache = filepath.Join(cache, "telemetry-display-v1")
				a.data, err = prepareStorage(p.Data)
				if err != nil {
					break
				}
				if b, e := os.ReadFile(filepath.Join(a.data, "settings.json")); e == nil {
					var prefs Preferences
					if json.Unmarshal(b, &prefs) == nil && prefs.Port >= 1024 && prefs.Port <= 65535 && (prefs.Mode == "nearest" || prefs.Mode == "full" || prefs.Mode == "none") {
						a.prefs = prefs
					}
				}
				a.assets = filepath.Join(a.data, "pills-v0.1")
				a.rec, err = openRecorder(a.data)
				if err != nil {
					a.recordingErr = err.Error()
					err = nil
				}
				if a.rec != nil {
					a.done = make(chan struct{})
					a.teamWake = make(chan struct{}, 1)
					done := a.done
					wake := a.teamWake
					go func() {
						ticker := time.NewTicker(time.Second)
						defer ticker.Stop()
						for {
							select {
							case <-done:
								return
							case <-ticker.C:
							case <-wake:
							}
							a.mu.Lock()
							if a.rec.current != nil && a.rec.current.Packets != a.rec.flushedPackets {
								if e := a.rec.flush(); e != nil {
									a.rec.fail(e)
								}
							}
							a.mu.Unlock()
							a.captureTeamNames()
						}
					}()
				}
				a.savedPort = a.prefs.Port
				if err = makePills(a.assets); err != nil {
					break
				}
				a.profile(p.Profile)
				a.syncOptions()
				ready = true
				result = M{"api": 1}
			}
		case "profile":
			a.profile(msg.Params)
			a.syncOptions()
		case "view", "tick":
			if !ready {
				err = fmt.Errorf("hello is required first")
				break
			}
			var p struct {
				Section string `json:"section"`
			}
			err = json.Unmarshal(msg.Params, &p)
			if err != nil {
				break
			}
			if p.Section != "telemetry" && p.Section != "analysis" {
				err = fmt.Errorf("unknown section %q", p.Section)
				break
			}
			a.syncOptions()
			a.gameStatus(scan, &hostID)
			if msg.Method == "view" {
				if p.Section == "analysis" {
					result = a.analysisView()
				} else {
					result = a.telemetryView()
				}
			} else {
				a.send(M{"method": "invalidate", "params": M{"section": p.Section}})
			}
		case "event":
			result, err = a.event(msg.Params)
		case "summary":
			result = M{"label": "UDP TELEMETRY", "items": []string{"v" + addonVersion + " · 2025 / 2026"}}
		case "build":
			result = M{"ops": []any{}, "lines": []string{"UDP Telemetry v0.2 preview: launcher pages and local UDP recordings; enable UDP in the game."}, "warnings": []any{}}
		default:
			err = fmt.Errorf("unsupported launcher method %q", msg.Method)
		}
		if len(msg.ID) != 0 {
			if err != nil {
				a.send(M{"id": msg.ID, "error": err.Error()})
			} else {
				a.send(M{"id": msg.ID, "result": result})
			}
		}
	}
	return scan.Err()
}
func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
