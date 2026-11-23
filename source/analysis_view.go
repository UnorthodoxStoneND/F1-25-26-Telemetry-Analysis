package main

import (
	"cmp"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

type analysisPage struct {
	selected string
	entries  []analysisEntry
	state    string
}

type analysisEntry struct {
	key, dir, profile, track, label, started, state, coverage string
	schema                                                    string
	practice                                                  bool
}

var analysisFiles = []string{"session.csv", "drivers.csv", "laps.csv", "samples.csv", flagsFileName}

func readAnalysisCSV(dir, name string, header []string, limit int) ([][]string, error) {
	file, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(io.LimitReader(file, 2*1024*1024))
	first, err := reader.Read()
	if err != nil || !slices.Equal(first, header) {
		return nil, fmt.Errorf("invalid %s header", name)
	}
	var rows [][]string
	for {
		row, err := reader.Read()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil || len(rows) >= limit {
			return nil, fmt.Errorf("invalid or oversized %s", name)
		}
		rows = append(rows, row)
	}
}

func (a *analysisCapture) entries() ([]analysisEntry, error) {
	root := filepath.Join(a.root, "Recordings")
	list := []analysisEntry{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Name() != "session.csv" || !entry.Type().IsRegular() {
			return nil
		}
		dir := filepath.Dir(path)
		rows, err := readAnalysisCSV(dir, "session.csv", sessionCSV, 1)
		if err != nil || len(rows) != 1 || (rows[0][0] != "1" && rows[0][0] != analysisSchema) {
			return nil
		}
		row := rows[0]
		state := row[11]
		if state == "recording" {
			if a.current != nil && a.current.dir == dir {
				return nil
			}
			state = "interrupted"
		}
		key, _ := filepath.Rel(root, dir)
		typeID, _ := strconv.Atoi(row[7])
		list = append(list, analysisEntry{key: filepath.ToSlash(key), dir: dir, schema: row[0], profile: row[1], track: row[5],
			label: row[6], started: row[9], state: state, coverage: row[13], practice: typeID >= 1 && typeID <= 4 || typeID == 0 && row[8] == "1"})
		return nil
	})
	if os.IsNotExist(err) {
		err = nil
	}
	slices.SortFunc(list, func(a, b analysisEntry) int { return cmp.Compare(b.started, a.started) })
	return list, err
}

func (a *App) analysisState() string {
	if !a.enabled || !a.analysisEnabled {
		return "disabled"
	}
	if a.capture.err != nil {
		return "error: " + a.capture.err.Error()
	}
	if r := a.capture.current; r != nil {
		return r.dir
	}
	return "idle"
}

func (a *App) analysisView() M {
	a.analysisPage.state = a.analysisState()
	view := M{"subtitle": "v" + addonVersion + " · Prototype", "tick": true}
	blocks := []M{}
	setBody := func() M {
		view["body"] = M{"blocks": blocks}
		return view
	}
	if !a.enabled || !a.analysisEnabled {
		blocks = append(blocks, textBlock("Telemetry Analysis is disabled. Enable it in the add-on settings to record sessions and view saved data."))
		return setBody()
	}
	if err := a.capture.configure(); err != nil {
		blocks = append(blocks, textBlock("Documents folder unavailable: "+err.Error()))
		return setBody()
	}
	if a.capture.err != nil {
		blocks = append(blocks, textBlock("Recording could not continue: "+a.capture.err.Error()))
	} else if r := a.capture.current; r != nil {
		blocks = append(blocks, textBlock("Recording "+r.session+". It will appear here when the session ends."))
	}
	entries, err := a.capture.entries()
	a.analysisPage.entries = entries
	if err != nil {
		blocks = append(blocks, textBlock("Could not read saved sessions: "+err.Error()))
		return setBody()
	}
	if len(entries) == 0 {
		if a.capture.current == nil {
			blocks = append(blocks, textBlock("No saved sessions. Record a session to view its data here."))
		}
		return setBody()
	}
	index := slices.IndexFunc(entries, func(e analysisEntry) bool { return e.key == a.analysisPage.selected })
	if index < 0 {
		index = 0
	}
	entry := entries[index]
	a.analysisPage.selected = entry.key
	options := make([]M, 0, len(entries))
	for _, e := range entries {
		date, _ := time.Parse(time.RFC3339Nano, e.started)
		_, circuit := analysisCircuit(e.track)
		options = append(options, M{"value": e.key, "label": fmt.Sprintf("%s · %s · %s · %s", date.Local().Format("2 Jan 2006 15:04"), e.profile, circuit, e.label)})
	}
	blocks = append(blocks, M{"form": M{"id": "analysis-session", "live": true, "fields": []M{
		{"select": M{"id": "session", "label": "Saved session", "value": entry.key, "options": options, "search": true}},
	}}})
	country, circuit := analysisCircuit(entry.track)
	blocks = append(blocks, M{"stats": []M{
		{"label": "Country", "value": country}, {"label": "Circuit", "value": circuit},
		{"label": "Session", "value": entry.label}, {"label": "Recording", "value": entry.state},
	}})
	if entry.coverage == "partial" || entry.state != "finished" {
		blocks = append(blocks, textBlock("This recording may have gaps. Average lap times use the valid laps captured."))
	}
	view["actions"] = []M{{"id": "analysis-export", "label": "Export raw data", "style": "ghost"}}
	if entry.practice {
		table, err := practiceRankings(entry.dir)
		if err != nil {
			blocks = append(blocks, textBlock("Practice rankings could not be read: "+err.Error()))
		} else {
			blocks = append(blocks, table)
		}
	} else {
		blocks = append(blocks, textBlock("Raw data is available for this session. This prototype includes rankings for practice sessions."))
	}
	return setBody()
}

func practiceRankings(dir string) (M, error) {
	drivers, err := readAnalysisCSV(dir, "drivers.csv", driversCSV, maxCars)
	if err != nil {
		return nil, err
	}
	ms := func(value string) uint32 {
		n, _ := strconv.ParseUint(value, 10, 32)
		return uint32(n)
	}
	slices.SortStableFunc(drivers, func(a, b []string) int {
		x, y := ms(a[4]), ms(b[4])
		if x == 0 && y != 0 {
			return 1
		}
		if y == 0 && x != 0 {
			return -1
		}
		return cmp.Compare(x, y)
	})
	columns := []M{}
	for i, title := range []string{"Pos", "Driver", "Team", "Compounds used", "Laps completed", "Fastest lap", "Gap", "Average time"} {
		columns = append(columns, M{"key": strconv.Itoa(i), "title": title})
	}
	rows := [][]string{}
	var leader uint32
	if len(drivers) > 0 {
		leader = ms(drivers[0][4])
	}
	for i, d := range drivers {
		best, average := ms(d[4]), ms(d[6])
		rank, gap := "—", "—"
		if best != 0 {
			rank = strconv.Itoa(i + 1)
			if best != leader {
				gap = fmt.Sprintf("+%.3f", float64(best-leader)/1000)
			}
		}
		rows = append(rows, []string{rank, d[1], analysisTeam(d[2]), dash(d[5]), dash(d[3]), lapTime(best, best > 0), gap, lapTime(average, average > 0)})
	}
	return M{"table": M{"id": "practice-rankings", "columns": columns, "rows": rows, "compact": true, "empty": "No driver data was captured."}}, nil
}

func dash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func (a *App) analysisEvent(raw json.RawMessage) (any, error) {
	var p struct {
		Event struct {
			Click  struct{ ID string }
			Submit struct {
				Form   string
				Values map[string]json.RawMessage
			}
		}
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	if !a.enabled || !a.analysisEnabled {
		return "reload", nil
	}
	if p.Event.Submit.Form == "analysis-session" {
		var key string
		if err := json.Unmarshal(p.Event.Submit.Values["session"], &key); err != nil {
			return nil, err
		}
		if !slices.ContainsFunc(a.analysisPage.entries, func(e analysisEntry) bool { return e.key == key }) {
			return M{"notice": "The selected recording is no longer available."}, nil
		}
		a.analysisPage.selected = key
		return "reload", nil
	}
	if p.Event.Click.ID == "analysis-export" {
		for _, entry := range a.analysisPage.entries {
			if entry.key == a.analysisPage.selected {
				dir, err := exportAnalysis(a.capture.root, entry)
				if err != nil {
					return M{"notice": "Export failed: " + err.Error()}, nil
				}
				return M{"open": dir}, nil
			}
		}
		return M{"notice": "Select a saved session first."}, nil
	}
	return "none", nil
}

func exportAnalysis(root string, entry analysisEntry) (string, error) {
	parent := filepath.Join(root, "Exports")
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(parent, analysisPathKey(entry.label)+"-")
	if err != nil {
		return "", err
	}
	files := analysisFiles
	if entry.schema == "1" {
		files = files[:len(files)-1] // Legacy recordings have four files.
	}
	for _, name := range files {
		if err := copyAnalysisFile(filepath.Join(entry.dir, name), filepath.Join(dir, name)); err != nil {
			for _, file := range files {
				_ = os.Remove(filepath.Join(dir, file))
			}
			_ = os.Remove(dir)
			return "", err
		}
	}
	return dir, nil
}

func copyAnalysisFile(source, destination string) error {
	stat, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !stat.Mode().IsRegular() {
		return fmt.Errorf("invalid recording file")
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func analysisCompound(actual, visual int32) string {
	switch visual {
	case 16:
		return "Soft"
	case 17:
		return "Medium"
	case 18:
		return "Hard"
	case 7:
		return "Intermediate"
	case 8:
		return "Wet"
	default:
		return fmt.Sprintf("Compound %d", actual)
	}
}

func analysisTeam(key string) string {
	names := map[string]string{
		"redbull": "Red Bull Racing", "mclaren": "McLaren", "ferrari": "Ferrari", "mercedes": "Mercedes", "alpine": "Alpine",
		"astonmartin": "Aston Martin", "racingbulls": "Racing Bulls", "rb": "Racing Bulls", "williams": "Williams", "haas": "Haas",
		"kicksauber": "Kick Sauber", "sauber": "Sauber", "audi": "Audi", "cadillac": "Cadillac", "gmcadillac": "Cadillac",
		"myteam": "My Team", "alfaromeo": "Alfa Romeo", "alphatauri": "AlphaTauri",
	}
	lookup := strings.ToLower(key)
	for _, suffix := range []string{"_2025", "_2026", "_25", "_26"} {
		lookup = strings.TrimSuffix(lookup, suffix)
	}
	if name := names[strings.ReplaceAll(lookup, "_", "")]; name != "" {
		return name
	}
	return dash(strings.ReplaceAll(key, "_", " "))
}

func analysisCircuit(key string) (string, string) {
	key = strings.ToLower(key)
	lookup := key
	for _, suffix := range []string{"_2022", "_2024", "_2025", "_2026"} {
		lookup = strings.TrimSuffix(lookup, suffix)
	}
	tracks := map[string][2]string{
		"melbourne": {"Australia", "Albert Park"}, "albert_park": {"Australia", "Albert Park"}, "shanghai": {"China", "Shanghai"},
		"bahrain": {"Bahrain", "Bahrain International Circuit"}, "catalunya": {"Spain", "Barcelona-Catalunya"},
		"monaco": {"Monaco", "Circuit de Monaco"}, "montreal": {"Canada", "Circuit Gilles Villeneuve"}, "silverstone": {"United Kingdom", "Silverstone"},
		"hungaroring": {"Hungary", "Hungaroring"}, "spa_francorchamps": {"Belgium", "Spa-Francorchamps"}, "spa": {"Belgium", "Spa-Francorchamps"},
		"monza": {"Italy", "Monza"}, "singapore": {"Singapore", "Marina Bay"}, "marina_bay": {"Singapore", "Marina Bay"},
		"suzuka": {"Japan", "Suzuka"}, "abu_dhabi": {"United Arab Emirates", "Yas Marina"},
		"texas": {"United States", "Circuit of the Americas"}, "americas": {"United States", "Circuit of the Americas"},
		"brazil": {"Brazil", "Interlagos"}, "interlagos": {"Brazil", "Interlagos"}, "austria": {"Austria", "Red Bull Ring"},
		"red_bull_ring": {"Austria", "Red Bull Ring"}, "mexico": {"Mexico", "Autódromo Hermanos Rodríguez"}, "baku": {"Azerbaijan", "Baku City Circuit"},
		"zandvoort": {"Netherlands", "Zandvoort"}, "imola": {"Italy", "Imola"}, "jeddah": {"Saudi Arabia", "Jeddah Corniche Circuit"},
		"miami": {"United States", "Miami International Autodrome"}, "las_vegas": {"United States", "Las Vegas Strip Circuit"},
		"losail": {"Qatar", "Lusail International Circuit"}, "madrid": {"Spain", "MADRING"},
		"korea":               {"South Korea", "Korea International Circuit"},
		"silverstone_reverse": {"United Kingdom", "Silverstone (Reverse)"}, "austria_reverse": {"Austria", "Red Bull Ring (Reverse)"},
		"zandvoort_reverse": {"Netherlands", "Zandvoort (Reverse)"},
	}
	if name, ok := tracks[lookup]; ok {
		return name[0], name[1]
	}
	return "—", dash(key)
}
