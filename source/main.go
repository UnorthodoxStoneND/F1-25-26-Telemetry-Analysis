package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const addonVersion = "0.5.0"
const addonID = "unorthodoxstonendmodding.udp-telemetry"

type M = map[string]any
type Message struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}
type App struct {
	reader                          Reader
	frame                           Frame
	enabled                         bool
	profileName, data, mode, status string
	highlight                       bool
	expanded                        string
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
	if a.profileName != p.Name || a.enabled != (p.Enabled && !p.Vanilla) {
		a.reader.Close()
		a.frame = Frame{Player: -1}
		a.expanded = ""
	}
	a.profileName = p.Name
	a.enabled = p.Enabled && !p.Vanilla
}
func (a *App) syncOptions() {
	a.mode = "nearest"
	a.highlight = false
	name := a.profileName
	if name == "" || len(name) > 40 || a.data == "" {
		return
	}
	for _, ch := range name {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_') {
			return
		}
	}
	file, err := os.Open(filepath.Join(filepath.Dir(filepath.Dir(a.data)), "profiles", name+".json"))
	if err != nil {
		return
	}
	defer file.Close()
	var p struct {
		Options map[string]map[string]json.RawMessage `json:"options"`
	}
	if json.NewDecoder(io.LimitReader(file, 1024*1024)).Decode(&p) != nil {
		return
	}
	options := p.Options[addonID]
	var mode string
	if json.Unmarshal(options["drivers_shown"], &mode) == nil && mode == "full" {
		a.mode = mode
	}
	_ = json.Unmarshal(options["highlight_player"], &a.highlight)
}
func (a *App) refresh() {
	a.syncOptions()
	if !a.enabled {
		a.reader.Close()
		a.frame = Frame{Player: -1}
		a.status = "Add-on disabled"
		return
	}
	f, err := a.reader.Read()
	if err != nil {
		a.status = err.Error()
		// A torn copy is transient. Process/mapping loss clears the display immediately.
		if a.reader.address == 0 || !errors.Is(err, errUpdating) {
			a.frame = Frame{Player: -1}
			a.expanded = ""
		}
		return
	}
	if f.Session != a.frame.Session {
		a.expanded = ""
	}
	a.frame = f
	a.status = "Live"
	if f.State&64 != 0 {
		a.status = "No new game values · retained"
	}
	if f.State&active == 0 {
		a.status = "Waiting for a session"
		if f.State&32 != 0 {
			a.status = "Session ended"
		} else if f.Diagnostic != "" {
			a.status = "Live data unavailable"
		}
		a.expanded = ""
	} else if f.State&paused != 0 {
		a.status = "Paused · values retained"
	}
}
func (a *App) event(raw json.RawMessage) (string, error) {
	var p struct {
		Event struct {
			Submit struct {
				Form, Row string
				Values    map[string]json.RawMessage
			}
		}
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", err
	}
	s := p.Event.Submit
	if s.Form != "drivers" && !strings.HasPrefix(s.Form, "drivers-after-") {
		return "none", nil
	}
	var open bool
	if err := json.Unmarshal(s.Values["setup"], &open); err != nil {
		return "", err
	}
	for _, c := range a.frame.Cars {
		if rowID(a.frame, c) == s.Row {
			a.expanded = ""
			if open {
				a.expanded = s.Row
			}
			return "reload", nil
		}
	}
	return "reload", nil // Session or car changed while the event was in flight.
}
func run(in io.Reader, out io.Writer) error {
	a := App{mode: "nearest", frame: Frame{Player: -1}}
	defer a.reader.Close()
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	send := func(value any) { _ = json.NewEncoder(out).Encode(value) }
	ready := false
	for scanner.Scan() {
		var msg Message
		if json.Unmarshal(scanner.Bytes(), &msg) != nil {
			continue
		}
		var result any
		var err error
		switch msg.Method {
		case "hello":
			var p struct {
				API     int             `json:"api"`
				Data    string          `json:"data_dir"`
				Profile json.RawMessage `json:"profile"`
			}
			err = json.Unmarshal(msg.Params, &p)
			if err == nil {
				if p.API != 1 || p.Data == "" {
					err = fmt.Errorf("launcher API 1 and data_dir are required")
				} else {
					a.data = p.Data
					a.profile(p.Profile)
					ready = true
					result = M{"api": 1}
				}
			}
		case "profile":
			a.profile(msg.Params)
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
			if p.Section != "telemetry" {
				err = fmt.Errorf("unknown section")
				break
			}
			a.refresh()
			if msg.Method == "view" {
				result = a.view()
			} else {
				send(M{"method": "invalidate", "params": M{"section": "telemetry"}})
			}
		case "event":
			if !ready {
				err = fmt.Errorf("hello is required first")
			} else {
				result, err = a.event(msg.Params)
			}
		case "summary":
			result = M{"label": "LIVE TELEMETRY", "items": []string{"v" + addonVersion + " · live display only"}}
		case "build":
			result = M{"ops": []any{}, "lines": []string{"Live Telemetry v" + addonVersion + ": native game readers; no UDP or recording."}, "warnings": []any{}}
		default:
			err = fmt.Errorf("unsupported launcher method")
		}
		if len(msg.ID) > 0 {
			if err != nil {
				send(M{"id": msg.ID, "error": err.Error()})
			} else {
				send(M{"id": msg.ID, "result": result})
			}
		}
	}
	return scanner.Err()
}
func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
