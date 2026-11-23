package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const addonVersion = "0.7.0-prototype.3"
const addonID = "unorthodoxstonendmodding.udp-telemetry"

type M = map[string]any
type Message struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}
type App struct {
	capture                           analysisCapture
	analysisPage                      analysisPage
	analysisEnabled, analysisViewSent bool
	desktopError                      string
	displayMode                       string
	reader                            Reader
	frame                             Frame
	enabled                           bool
	profileName, data, mode, status   string
	highlight                         bool
	disableSlides                     bool
	expanded                          string
	lastView                          viewState
	viewSent                          bool
	disabled                          [3]bool
	hidden                            [3]uint32
	widths                            [3]map[string]float32
	columnPath                        string
	columnsOpen                       bool
	editorLayout                      sessionLayout
	columnGeneration                  uint64
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
		a.capture.close("interrupted", "launcher profile changed", time.Now())
		a.capture.err = nil
		a.analysisPage = analysisPage{}
		a.desktopError = ""
		a.reader.Close()
		a.frame = Frame{Player: -1}
		a.expanded = ""
		a.columnsOpen = false
	}
	a.profileName = p.Name
	a.enabled = p.Enabled && !p.Vanilla
}
func (a *App) syncOptions() {
	wasRecordingEnabled := a.analysisEnabled
	a.analysisEnabled = false
	defer func() {
		if wasRecordingEnabled && !a.analysisEnabled {
			a.capture.close("interrupted", "Telemetry Analysis disabled", time.Now())
			a.capture.err = nil
		}
	}()
	a.displayMode = "launcher"
	a.mode = "nearest"
	a.highlight = false
	a.disableSlides = false
	a.disabled = [3]bool{}
	a.loadColumns()
	name := a.profileName
	if !validProfile(name) || a.data == "" {
		return
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
	_ = json.Unmarshal(options["enable_analysis"], &a.analysisEnabled)
	var display string
	if json.Unmarshal(options["display_mode"], &display) == nil && display == "window" {
		a.displayMode = display
	}
	var mode string
	if json.Unmarshal(options["drivers_shown"], &mode) == nil && mode == "full" {
		a.mode = mode
	}
	_ = json.Unmarshal(options["highlight_player"], &a.highlight)
	var sliding = true
	if raw := options["sliding_animation"]; string(raw) != "null" && json.Unmarshal(raw, &sliding) == nil {
		a.disableSlides = !sliding
	}
	for layout, key := range []string{"show_practice", "show_qualifying", "show_race"} {
		var on bool
		if raw := options[key]; string(raw) != "null" && json.Unmarshal(raw, &on) == nil {
			a.disabled[layout] = !on
		}
	}
	if a.disabled[layoutFor(a.frame)] {
		a.expanded = ""
		a.columnsOpen = false
	}
}
func (a *App) refresh() {
	a.syncOptions()
	a.refreshSynced()
}
func (a *App) refreshSynced() {
	if !a.enabled {
		a.reader.Close()
		a.frame = Frame{Player: -1}
		a.capture.close("interrupted", "add-on disabled", time.Now())
		a.status = "Add-on disabled"
		return
	}
	f, err := a.reader.Read()
	if err != nil {
		a.status = err.Error()
		// A torn copy is transient. Process/mapping loss clears the display immediately.
		if a.reader.address == 0 || !errors.Is(err, errUpdating) {
			a.capture.close("interrupted", "live game connection lost", time.Now())
			a.frame = Frame{Player: -1}
			a.expanded = ""
		}
		return
	}
	if f.Session != a.frame.Session || layoutFor(f) != layoutFor(a.frame) {
		a.expanded = ""
		a.columnsOpen = false
	}
	if a.analysisEnabled {
		a.capture.observe(a.profileName, f, time.Now())
	}
	a.frame = f
	a.status = "Live"
	if f.State&snapshotStale != 0 {
		a.status = "No new game values · retained"
	}
	if f.State&active == 0 {
		a.status = "Waiting for a session"
		if f.State&sessionEnded != 0 {
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
			Click  struct{ ID string }
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
	if handled, effect, err := a.columnEvent(p.Event.Click.ID, s.Form, s.Values); handled {
		return effect, err
	}
	if s.Form != "drivers" && !strings.HasPrefix(s.Form, "drivers-after-") {
		return "none", nil
	}
	if layoutFor(a.frame) == layoutRace || a.disabled[layoutFor(a.frame)] || !a.fieldVisible(layoutFor(a.frame), "setup") {
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
func run(in io.Reader, out io.Writer) error { return runWithDesktop(in, out, nil) }
func runWithDesktop(in io.Reader, out io.Writer, desktop *desktopBridge) error {
	a := App{mode: "nearest", frame: Frame{Player: -1}}
	defer a.reader.Close()
	defer func() { a.capture.close("interrupted", "launcher helper stopped", time.Now()) }()

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	send := func(value any) { _ = json.NewEncoder(out).Encode(value) }
	ready := false
	// The protocol, reader and UI preferences have one owner. The scanner and
	// desktop UI communicate with this loop, never touching App or Reader.
	incoming := make(chan []byte)
	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		defer close(incoming)
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			select {
			case incoming <- line:
			case <-stopped:
				return
			}
		}
	}()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	life := windowLifecycle{}
	var commands <-chan desktopAction
	if desktop != nil {
		commands = desktop.actions
	}
	updateDesktop := func() {
		if desktop != nil {
			desktop.publish(a.retroModel(life.observe(a.displayMode == "window" && a.enabled, a.frame, time.Now())))
		}
	}
	for {
		var line []byte
		select {
		case action := <-commands:
			if action.Kind == "dismiss" {
				life.dismiss()
			} else if a.displayMode == "window" {
				a.retroAction(action)
			}
			updateDesktop()
			continue
		case <-ticker.C:
			if ready {
				a.syncOptions()
				if a.analysisEnabled || (desktop != nil && a.displayMode == "window") {
					a.refreshSynced()
				}
				updateDesktop()
				if a.viewSent && a.needsView() {
					send(M{"method": "invalidate", "params": M{"section": "telemetry"}})
					a.lastView = a.viewState()
				}
				if a.analysisViewSent && a.analysisPage.state != a.analysisState() {
					a.analysisPage.state = a.analysisState()
					send(M{"method": "invalidate", "params": M{"section": "analysis"}})
				}
			}
			continue
		case next, ok := <-incoming:
			if !ok {
				return scanner.Err()
			}
			line = next
		}

		var msg Message
		if json.Unmarshal(line, &msg) != nil {
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
			if p.Section != "telemetry" && p.Section != "analysis" {
				err = fmt.Errorf("unknown section")
				break
			}
			if runtime.GOOS == "linux" && a.enabled {
				// Host calls are answered before the launcher sends another request.
				send(M{"id": 1000001, "method": "addon_game_dir", "params": nil})
				replyLine, ok := <-incoming
				if !ok {
					return scanner.Err()
				}
				var reply struct {
					ID     uint64
					Result *string
					Error  json.RawMessage
				}
				if json.Unmarshal(replyLine, &reply) != nil || reply.ID != 1000001 || len(reply.Error) > 0 && string(reply.Error) != "null" {
					err = fmt.Errorf("The launcher's add-on game directory is unavailable")
					break
				}
				dir := ""
				if reply.Result != nil {
					dir = *reply.Result
				}
				a.reader.setAddonDir(dir)
			}
			a.refresh()
			if p.Section == "analysis" {
				if msg.Method == "view" {
					result = a.analysisView()
					a.analysisViewSent = true
				} else if a.analysisPage.state != a.analysisState() {
					a.analysisPage.state = a.analysisState()
					send(M{"method": "invalidate", "params": M{"section": "analysis"}})
				}
				break
			}
			if msg.Method == "view" {
				result = a.view()
				a.lastView = a.viewState()
				a.viewSent = true
			} else if a.needsView() {
				send(M{"method": "invalidate", "params": M{"section": "telemetry"}})
			}
		case "event":
			if ready && runtime.GOOS == "linux" && a.enabled && strings.Contains(string(msg.Params), "startup") {
				send(M{"id": 1000001, "method": "addon_game_dir", "params": nil})
				replyLine, ok := <-incoming
				if !ok {
					return scanner.Err()
				}
				var reply struct {
					ID     uint64
					Result *string
					Error  json.RawMessage
				}
				if json.Unmarshal(replyLine, &reply) == nil && reply.ID == 1000001 && reply.Result != nil && (len(reply.Error) == 0 || string(reply.Error) == "null") {
					a.reader.setAddonDir(*reply.Result)
				}
			}
			if !ready {
				err = fmt.Errorf("hello is required first")
			} else {
				var p struct{ Section string }
				if err = json.Unmarshal(msg.Params, &p); err != nil {
					break
				}
				if p.Section == "analysis" {
					a.syncOptions()
					result, err = a.analysisEvent(msg.Params)
				} else {
					result, err = a.event(msg.Params)
				}
			}
		case "summary":
			result = M{"label": "F1 LIVE TELEMETRY & ANALYSIS", "items": []string{"v" + addonVersion + " · optional analysis recording"}}
		case "build":
			result = M{"ops": []any{}, "lines": []string{"F1 Live Telemetry & Analysis v" + addonVersion + ": Swofty Live State, with opt-in CSV recording."}, "warnings": []any{}}
		default:
			err = fmt.Errorf("unsupported launcher method")
		}
		updateDesktop()
		if len(msg.ID) > 0 {
			if err != nil {
				send(M{"id": msg.ID, "error": err.Error()})
			} else {
				send(M{"id": msg.ID, "result": result})
			}
		}
	}
}
func helperMain() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
