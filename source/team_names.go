package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type CapturedTeam struct {
	Set       int    `json:"set"`
	ID        int    `json:"id"`
	Label     string `json:"label"`
	RawName   string `json:"raw_name"`
	Name      string `json:"name,omitempty"`
	ShortName string `json:"short_name,omitempty"`
	Code      string `json:"code,omitempty"`
}
type TeamSnapshot struct {
	Captured       time.Time      `json:"captured"`
	PID            uint32         `json:"pid"`
	ProcessStarted time.Time      `json:"process_started"`
	Build          string         `json:"exe_sha256"`
	Mode           string         `json:"mode"`
	Language       string         `json:"language"`
	Teams          []CapturedTeam `json:"teams"`
	// Added teams can be rewritten to stock ancestors in UDP. Persist that
	// mapping too; ambiguous ancestors must not acquire a guessed team name.
	Rewrites           map[string]int    `json:"udp_rewrites,omitempty"`
	ByteRewrites       map[string]int    `json:"udp_byte_rewrites,omitempty"`
	DriverRewrites     map[string]int    `json:"driver_rewrites,omitempty"`
	DriverByteRewrites map[string]int    `json:"driver_byte_rewrites,omitempty"`
	Entrants           []CapturedEntrant `json:"entrants,omitempty"`
	Warnings           []string          `json:"warnings,omitempty"`
}
type CapturedEntrant struct {
	DriverID int `json:"driver_id"`
	Number   int `json:"race_number"`
	Set      int `json:"team_set"`
	TeamID   int `json:"team_id"`
}

func mappedIdentity(id, format int, mapping, byteMapping map[string]int) int {
	if format == 2025 {
		id &= 255
		mapping = byteMapping
	}
	if value, ok := mapping[strconv.Itoa(id)]; ok {
		return value
	}
	return id
}

func validTeamText(s string) bool {
	if s == "" || len(s) > 512 || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 32 && r != '\t' {
			return false
		}
	}
	return true
}

func (m *AnalysisModel) teamName(d RecordedDriver) string {
	s := m.Meta.Teams
	if s == nil || !d.TeamKnown {
		return "—"
	}
	matched := map[string]bool{}
	for _, e := range s.Entrants {
		if e.Number != d.RaceNumber || (e.Set == 1) != d.MyTeam {
			continue
		}
		driver := mappedIdentity(e.DriverID, m.Meta.Format, s.DriverRewrites, s.DriverByteRewrites)
		team := mappedIdentity(e.TeamID, m.Meta.Format, s.Rewrites, s.ByteRewrites)
		unknownDriver := d.DriverID == 255 || d.DriverID == 65535
		if (!unknownDriver && driver != d.DriverID && e.DriverID != d.DriverID) || (!d.MyTeam && team != d.TeamID && e.TeamID != d.TeamID) {
			continue
		}
		matched[fmt.Sprintf("%d:%d", e.Set, e.TeamID)] = true
	}
	if len(matched) == 1 {
		for _, t := range s.Teams {
			if matched[fmt.Sprintf("%d:%d", t.Set, t.ID)] && t.Name != "" {
				return capturedTeamDisplayName(t)
			}
		}
	}
	candidates := []CapturedTeam{}
	if d.MyTeam {
		// Never treat the generic database My Team template as the saved name.
		for _, t := range s.Teams {
			if t.Set == 1 && t.Name != "" {
				candidates = append(candidates, t)
			}
		}
	} else {
		for _, t := range s.Teams {
			if t.Set != 0 {
				continue
			}
			id := mappedIdentity(t.ID, m.Meta.Format, s.Rewrites, s.ByteRewrites)
			if id == d.TeamID {
				candidates = append(candidates, t)
			}
		}
	}
	if len(candidates) == 1 && candidates[0].Name != "" {
		return capturedTeamDisplayName(candidates[0])
	}
	return "—"
}

// A snapshot belongs only to the recording that requested it. No cached names
// from a previous save, session or process are applied to another recording.
func (a *App) captureTeamNames() {
	a.mu.Lock()
	if !a.enabled || a.rec == nil || a.rec.current == nil || a.rec.current.State != "recording" || a.rec.current.Session == 0 || a.rec.current.Teams != nil {
		a.mu.Unlock()
		return
	}
	id, profile := a.rec.current.ID, a.profileName
	if a.teamAttemptID == id && time.Since(a.teamAttempt) < 5*time.Second {
		a.mu.Unlock()
		return
	}
	a.teamAttemptID, a.teamAttempt = id, time.Now()
	launcher := a.launcherData
	a.mu.Unlock()
	snapshot, err := captureLoadedTeams(launcher)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.applyTeamCapture(id, profile, snapshot, err)
}
func (a *App) applyTeamCapture(id, profile string, snapshot *TeamSnapshot, err error) {
	if a.rec == nil || a.rec.current == nil || a.rec.current.ID != id || a.profileName != profile {
		return
	}
	if err != nil {
		a.rec.current.TeamCaptureError = err.Error()
	} else {
		a.rec.current.Teams = snapshot
		a.rec.current.TeamCaptureError = ""
	}
	if e := a.rec.persist(); e != nil {
		a.recordingErr = fmt.Sprintf("Save team names: %v", e)
	}
	if model := a.analysis.Model; model != nil && model.Meta.ID == id {
		model.Meta.Teams = a.rec.current.Teams
		model.Meta.TeamCaptureError = a.rec.current.TeamCaptureError
	}
	if a.analysis.Loading == id {
		// A loader that started before capture carries an older metadata copy.
		a.analysis.Generation++
		a.analysis.Loading = ""
	}
	if a.out != nil {
		a.send(M{"method": "invalidate", "params": M{"section": "analysis"}})
	}
}

var teamCaseOperation = regexp.MustCompile(`\{o:(upper|lower)\}([^{}]*)\{/o\}`)
var teamOperationTag = regexp.MustCompile(`\{o:[^{}]*\}|\{/o\}`)

func capturedTeamDisplayName(t CapturedTeam) string {
	if strings.HasPrefix(strings.TrimPrefix(t.RawName, "@"), "lng_") || strings.Contains(t.Name, "{o:") {
		return cleanTeamName(t.Name)
	}
	return t.Name
}

func cleanTeamName(s string) string {
	// The game's names can contain text-formatting tags (e.g. McLaren's title).
	for {
		start := strings.IndexByte(s, '<')
		if start < 0 {
			break
		}
		end := strings.IndexByte(s[start:], '>')
		if end < 0 {
			break
		}
		s = s[:start] + s[start+end+1:]
	}
	for {
		updated := teamCaseOperation.ReplaceAllStringFunc(s, func(match string) string {
			parts := teamCaseOperation.FindStringSubmatch(match)
			if parts[1] == "upper" {
				return strings.ToUpper(parts[2])
			}
			return strings.ToLower(parts[2])
		})
		if updated == s {
			break
		}
		s = updated
	}
	return strings.TrimSpace(teamOperationTag.ReplaceAllString(s, ""))
}
