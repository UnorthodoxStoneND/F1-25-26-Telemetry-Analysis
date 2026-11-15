package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// LNGT is chunked, with big-endian sizes and key/value byte offsets. SIDA
// additionally carries a four-byte count before its size bytes of pairs.
func parseTeamLanguage(b []byte) (map[string]string, error) {
	if len(b) < 8 || string(b[:4]) != "LNGT" {
		return nil, fmt.Errorf("not a language table")
	}
	var index, keys, text []byte
	for off := 8; off < len(b); {
		if len(b)-off < 8 {
			return nil, fmt.Errorf("truncated language chunk")
		}
		tag := string(b[off : off+4])
		size := int(binary.BigEndian.Uint32(b[off+4:]))
		off += 8
		if tag == "SIDA" {
			off += 4
		}
		if off > len(b) || size > len(b)-off {
			return nil, fmt.Errorf("invalid language chunk size")
		}
		chunk := b[off : off+size]
		off += size
		switch tag {
		case "SIDA":
			index = chunk
		case "SIDB":
			keys = chunk
		case "LNGB":
			text = chunk
		}
	}
	if index == nil || keys == nil || text == nil || len(index)%8 != 0 {
		return nil, fmt.Errorf("missing language chunks")
	}
	get := func(b []byte, off uint32) (string, bool) {
		if uint64(off) >= uint64(len(b)) {
			return "", false
		}
		b = b[off:]
		n := bytes.IndexByte(b, 0)
		if n < 0 {
			return "", false
		}
		return string(b[:n]), true
	}
	out := map[string]string{}
	for i := 0; i < len(index); i += 8 {
		key, ok := get(keys, binary.BigEndian.Uint32(index[i:]))
		value, valid := get(text, binary.BigEndian.Uint32(index[i+4:]))
		if !ok || !valid {
			return nil, fmt.Errorf("invalid language string offset")
		}
		out[key] = value
	}
	return out, nil
}
func readTeamFile(path string, limit int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("team metadata file too large")
	}
	return b, e
}
func mergeTeamLanguage(path string, out map[string]string) error {
	b, e := readTeamFile(path, 32<<20)
	if e != nil {
		return e
	}
	v, e := parseTeamLanguage(b)
	if e != nil {
		return e
	}
	for k, s := range v {
		out[k] = s
	}
	return nil
}
func resolveTeamText(raw string, texts map[string]string) string {
	key := strings.TrimPrefix(raw, "@")
	if strings.HasPrefix(key, "lng_") {
		raw = cleanTeamName(texts[key])
	}
	name := strings.TrimSpace(raw)
	if !validTeamText(name) {
		return ""
	}
	return name
}

var teamLocalePattern = regexp.MustCompile(`current language ([a-z]{3})`)

func currentTeamLanguage(game string, started time.Time) string {
	path := filepath.Join(game, "swofty", "loader.log")
	f, e := os.Open(path)
	if e != nil {
		return "eng"
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || info.ModTime().Before(started) {
		return "eng"
	}
	if info.Size() > 8<<20 {
		_, _ = f.Seek(-(8 << 20), io.SeekEnd)
	}
	b, _ := io.ReadAll(io.LimitReader(f, 8<<20))
	matches := teamLocalePattern.FindAllSubmatch(b, -1)
	if len(matches) > 0 {
		return string(matches[len(matches)-1][1])
	}
	return "eng"
}

type teamLanguageTable struct {
	Table    string `json:"table"`
	Priority int    `json:"priority"`
}

func mountedTeamLanguages(game, language string) []teamLanguageTable {
	b, e := readTeamFile(filepath.Join(game, "swofty", "assets.json"), 32<<20)
	if e != nil {
		return nil
	}
	var config struct {
		Lang map[string]json.RawMessage `json:"lang"`
	}
	if json.Unmarshal(b, &config) != nil {
		return nil
	}
	tables := []teamLanguageTable{}
	for _, code := range []string{"*", language} {
		var list []teamLanguageTable
		if json.Unmarshal(config.Lang[code], &list) != nil {
			var one teamLanguageTable
			if json.Unmarshal(config.Lang[code], &one) == nil {
				list = []teamLanguageTable{one}
			}
		}
		for _, entry := range list {
			// Mount paths are relative to the game's swofty root. Do not follow arbitrary
			// paths from a corrupt manifest outside that root.
			path := filepath.Clean(filepath.FromSlash(entry.Table))
			if path == "." || filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, ".."+string(os.PathSeparator)) {
				continue
			}
			entry.Table = filepath.Join(game, "swofty", path)
			tables = append(tables, entry)
		}
	}
	sort.SliceStable(tables, func(i, j int) bool { return tables[i].Priority < tables[j].Priority })
	return tables
}
func loadTeamLanguage(root, game, mode string, started time.Time) (map[string]string, string, []string) {
	language := currentTeamLanguage(game, started)
	out := map[string]string{}
	warnings := []string{}
	// Use the baseline for this exact executable, not the newest arbitrary game
	// version. The active mount tables then supply season/mod translations.
	for _, code := range []string{"eng", language} {
		direct := filepath.Join(game, "localisation", "2025_release", "language_"+code+".lng")
		if mergeTeamLanguage(direct, out) == nil {
			continue
		}
		paths, _ := filepath.Glob(filepath.Join(root, "baseline", "*-"+verifiedTeamBuild[:12], "lang", "language_"+code+".lng"))
		for _, p := range paths {
			if mergeTeamLanguage(p, out) == nil {
				break
			}
		}
	}
	for _, table := range mountedTeamLanguages(game, language) {
		if e := mergeTeamLanguage(table.Table, out); e != nil {
			warnings = append(warnings, "A mounted team-name language table could not be read.")
		}
	}
	// Runtime text overrides have priority over language tables, just as the
	// game's Loc_LangSet_Find hook does. Ignore stale/scoped unrelated records.
	path := filepath.Join(root, "registry", "effective.json")
	if info, e := os.Stat(path); e == nil && !info.ModTime().Before(started.Add(-time.Minute)) {
		if b, e := readTeamFile(path, 64<<20); e == nil {
			applyTeamTextOverrides(b, language, mode, out)
		}
	}
	if len(out) == 0 {
		warnings = append(warnings, "Team-name language data was unavailable; unresolved names remain unknown.")
	}
	return out, language, warnings
}
func applyTeamTextOverrides(b []byte, language, mode string, out map[string]string) {
	var report struct {
		Strings map[string]struct {
			Status string `json:"status"`
			Value  struct {
				Text   map[string]string `json:"text"`
				Within map[string]string `json:"within"`
			} `json:"value"`
		} `json:"lang.string"`
	}
	if json.Unmarshal(b, &report) != nil {
		return
	}
	for key, entry := range report.Strings {
		if entry.Status != "ok" || !strings.HasPrefix(key, "game/") {
			continue
		}
		allowed := true
		for field, value := range entry.Value.Within {
			if field != "mode" || value != mode {
				allowed = false
			}
		}
		if !allowed {
			continue
		}
		if value, ok := entry.Value.Text[language]; ok {
			out[strings.TrimPrefix(key, "game/")] = value
		} else if value, ok := entry.Value.Text["*"]; ok {
			out[strings.TrimPrefix(key, "game/")] = value
		}
	}
}
func loadTeamRewrites(path string, s *TeamSnapshot) {
	b, e := readTeamFile(path, 4<<20)
	if e != nil {
		return
	}
	var mappings struct {
		Version    int            `json:"version"`
		Teams      map[string]int `json:"teams"`
		Byte       map[string]int `json:"teams_byte"`
		Drivers    map[string]int `json:"drivers"`
		DriverByte map[string]int `json:"drivers_byte"`
	}
	if json.Unmarshal(b, &mappings) != nil || mappings.Version != 1 {
		return
	}
	s.Rewrites, s.ByteRewrites, s.DriverRewrites, s.DriverByteRewrites = mappings.Teams, mappings.Byte, mappings.Drivers, mappings.DriverByte
}
