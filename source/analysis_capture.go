package main

import (
	"crypto/sha256"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	analysisLap = 1 << iota
	analysisSpeed
	analysisPedals
	analysisGear
	analysisRPM
	analysisProgress
	analysisGap
	analysisPit
)

type AnalysisSession struct {
	Run, Attempt, Sample uint64
	Season, Round        int32
	EndReason            uint32
	SafetyCarKnown       bool
	SafetyCar            uint8
	Chequered            bool
	Track, Career        string
}

type AnalysisCar struct {
	Known, Current, BestLap, CurrentS1, CurrentS2 uint32
	GapLeader, Gear, Lap                          int32
	Speed, Throttle, Brake, RPM, LapDistance      float32
	LapFlags, PitStatus, PitStops, Stint          uint8
	Team                                          string
}

const analysisFolder = "F1 Live Telemetry"

var sessionCSV = []string{"schema", "profile", "career_id", "season", "round", "track_key", "session", "type", "kind", "started_utc", "updated_utc", "state", "reason", "coverage", "run", "attempt", "addon_version"}
var sampleCSV = []string{"elapsed_ms", "epoch", "driver_id", "driver", "team_key", "position", "grid", "result", "lap", "laps_completed", "current_lap_ms", "current_s1_ms", "current_s2_ms", "lap_flags", "last_lap_ms", "last_s1_ms", "last_s2_ms", "last_s3_ms", "best_lap_ms", "best_lap_number", "best_s1_ms", "best_s2_ms", "best_s3_ms", "lap_distance_m", "speed_m_s", "throttle", "brake", "gear", "rpm", "gap_ahead_ms", "gap_leader_ms", "actual_compound_id", "visual_compound_id", "tyre_age_laps", "stint_laps", "wear_fl_pct", "wear_fr_pct", "wear_rl_pct", "wear_rr_pct", "in_pit_lane", "pit_status", "pit_stops", "safety_car_status", "chequered", "drive_through", "stop_go", "time_penalty_ms", "session_time_left_s", "session_total_laps"}
var lapsCSV = []string{"driver_id", "driver", "team_key", "lap", "elapsed_ms", "lap_ms", "valid", "s1_ms", "s2_ms", "s3_ms", "actual_compound_id", "visual_compound_id"}
var driversCSV = []string{"driver_id", "driver", "team_key", "laps_completed", "best_lap_ms", "compounds", "average_valid_lap_ms", "recorded_valid_laps"}

type analysisCapture struct {
	root      string
	err       error
	current   *analysisRecording
	continued map[analysisAttempt]bool
}

type analysisAttempt struct{ run, attempt uint64 }

type analysisRecording struct {
	key                 analysisAttempt
	dir, profile        string
	session, track      string
	career              string
	season, round       int32
	sessionType, kind   uint32
	started, checkpoint time.Time
	startSample, sample uint64
	epoch               int
	partial, chequered  bool
	drivers             [maxCars]analysisDriver
	file                *os.File
	samples             *csv.Writer
}

type analysisDriver struct {
	present, hasLap bool
	name, team      string
	laps            uint32
	clock           uint32
	best            uint32
	compounds       map[analysisTyre]uint32
	tyre            analysisTyre
	tyreKnown       bool
	recorded        map[uint32]analysisLapRow
}

type analysisTyre struct{ actual, visual int32 }

type analysisLapRow struct {
	time, elapsed  uint64
	valid          bool
	sectors        [3]uint32
	actual, visual string
}

func (a *analysisCapture) configure() error {
	if a.root != "" {
		return nil
	}
	base, err := analysisDocuments()
	if err != nil {
		return err
	}
	if !filepath.IsAbs(base) {
		return fmt.Errorf("no absolute Documents folder")
	}
	a.root = filepath.Join(base, analysisFolder)
	return nil
}

func analysisPathKey(value string) string {
	// Keep directories portable and collision-resistant without using external
	// strings as paths. Preserve the original names inside session.csv.
	hash := sha256.Sum256([]byte(value))
	var base strings.Builder
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			base.WriteRune(r)
		case r == '_' || r == '-' || r == ' ':
			base.WriteByte('-')
		}
		if base.Len() >= 24 {
			break
		}
	}
	return fmt.Sprintf("id-%s-%x", strings.Trim(base.String(), "-"), hash[:6])
}

// Guard user-controlled spreadsheet text. Numeric values use separate helpers.
func analysisText(value string) string {
	if v := strings.TrimLeft(value, " \t\r\n"); v != "" && strings.ContainsRune("=+-@", rune(v[0])) {
		return "'" + value
	}
	return value
}

func num[T ~int | ~int32 | ~uint8 | ~uint32 | ~uint64](n T, known bool) string {
	if !known {
		return ""
	}
	return fmt.Sprint(n)
}

func decimal(n float32, known bool) string {
	if !known || !finite(n) {
		return ""
	}
	return strconv.FormatFloat(float64(n), 'f', -1, 32)
}

func truth(value, known bool) string {
	if !known {
		return ""
	}
	return strconv.FormatBool(value)
}

func (a *analysisCapture) close(state, reason string, now time.Time) {
	if r := a.current; r != nil {
		a.current = nil
		if a.continued == nil {
			a.continued = make(map[analysisAttempt]bool)
		}
		if state == "interrupted" {
			a.continued[r.key] = true
		}
		r.samples.Flush()
		err := errors.Join(r.samples.Error(), r.file.Sync(), r.file.Close())
		if err != nil {
			state, reason = "interrupted", "samples could not be saved"
		}
		if saveErr := errors.Join(err, r.save(state, reason, now)); saveErr != nil {
			a.err = saveErr
		}
	}
}

func (a *analysisCapture) observe(profile string, f Frame, now time.Time) {
	s := f.Analysis
	key := analysisAttempt{s.Run, s.Attempt}
	if r := a.current; r != nil && key != r.key && key.run != 0 {
		a.close("interrupted", "a new attempt started before this one ended", now)
	}
	if f.State&sessionEnded != 0 {
		state, reason := "ended", "session ended before a chequered flag was observed"
		if s.EndReason == 2 {
			state, reason = "interrupted", "session restarted"
		} else if a.current != nil && a.current.chequered {
			state, reason = "finished", "chequered flag observed"
		}
		a.close(state, reason, now)
		return
	}
	if key.run == 0 || key.attempt == 0 || s.Sample == 0 ||
		f.State&(active|windowReady|paused|snapshotStale) != active|windowReady ||
		f.SessionType == 18 || f.SessionKind == 5 || (f.SessionType == 0 && f.SessionKind == 0) {
		return
	}
	if a.err != nil {
		return
	}
	if a.current == nil {
		if a.err = a.configure(); a.err != nil {
			return
		}
		r, err := a.start(profile, f, now)
		if err != nil {
			a.err = err
			return
		}
		a.current = r
	}
	r := a.current
	if r.sample >= s.Sample {
		return
	}
	if now.Sub(r.checkpoint) >= 5*time.Second {
		r.samples.Flush()
		if err := errors.Join(r.samples.Error(), r.save("recording", "", now)); err != nil {
			a.close("interrupted", "recording write failed", now)
			a.err = err
			return
		}
		r.checkpoint = now
	}
	if r.track == "" {
		r.track = s.Track
	}
	if r.career == "" {
		r.career = s.Career
	}
	if r.round < 1 && s.Round > 0 && s.Season >= 0 {
		r.season, r.round = s.Season, s.Round
	}
	if f.SessionKind != 0 {
		r.session, r.kind = sessionLabel(f), f.SessionKind
	}
	if err := r.record(f); err != nil {
		a.close("interrupted", "recording write failed", now)
		a.err = err
	}
}

func (a *analysisCapture) start(profile string, f Frame, now time.Time) (*analysisRecording, error) {
	s := f.Analysis
	r := &analysisRecording{
		key: analysisAttempt{s.Run, s.Attempt}, profile: profile,
		session: sessionLabel(f), track: s.Track, career: s.Career,
		season: s.Season, round: s.Round, sessionType: f.SessionType, kind: f.SessionKind,
		started: now, checkpoint: now, startSample: s.Sample,
	}
	r.partial = a.continued[r.key]
	career, season, weekend := "standalone", "season-unknown", fmt.Sprintf("run-%x", s.Run)
	if s.Career != "" {
		career = "career-" + analysisPathKey(s.Career)
		if s.Season >= 0 && s.Round > 0 {
			season = fmt.Sprintf("season-%03d", s.Season+1)
			weekend = fmt.Sprintf("round-%03d", s.Round)
		}
	}
	parent := filepath.Join(a.root, "Recordings", analysisPathKey(profile), career, season,
		weekend+"-"+analysisPathKey(s.Track))
	if err := os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(parent, now.UTC().Format("20060102T150405Z")+"-"+analysisPathKey(r.session)+"-")
	if err != nil {
		return nil, err
	}
	r.dir = dir
	if r.file, err = os.OpenFile(filepath.Join(dir, "samples.csv"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600); err != nil {
		return nil, err
	}
	r.samples = csv.NewWriter(r.file)
	if err = errors.Join(r.samples.Write(sampleCSV), r.save("recording", "", now)); err != nil {
		return nil, errors.Join(err, r.file.Close())
	}
	return r, nil
}

func (r *analysisRecording) record(f Frame) error {
	r.sample = f.Analysis.Sample
	elapsed := r.sample - r.startSample
	rewound := false
	for _, c := range f.Cars {
		d, extra := &r.drivers[c.Vehicle], c.Analysis
		if c.Known&knownLaps == 0 || !d.hasLap {
			continue
		}
		if c.Laps < d.laps || (int32(c.Vehicle) == f.Player && c.Laps == d.laps &&
			extra.Known&analysisLap != 0 && c.Flags&inPitLane == 0 &&
			extra.Current > 0 && extra.Current+1000 < d.clock) {
			rewound = true
			break
		}
	}
	if rewound {
		r.epoch++
		r.chequered = false
		for _, c := range f.Cars {
			d := &r.drivers[c.Vehicle]
			if c.Known&knownLaps != 0 {
				for tyre, firstLap := range d.compounds {
					if firstLap >= c.Laps {
						delete(d.compounds, tyre)
					}
				}
				for lap := range d.recorded {
					if lap >= c.Laps {
						delete(d.recorded, lap)
					}
				}
				d.laps, d.hasLap = c.Laps, true
				d.tyreKnown = false
			}
		}
	}
	r.chequered = r.chequered || f.Analysis.Chequered
	for _, c := range f.Cars {
		d, extra := &r.drivers[c.Vehicle], c.Analysis
		if !d.present {
			*d = analysisDriver{present: true, compounds: make(map[analysisTyre]uint32), recorded: make(map[uint32]analysisLapRow)}
		}
		if c.Known&knownName != 0 || d.name == "" {
			d.name = driverName(c)
		}
		if extra.Team != "" {
			d.team = extra.Team
		}
		if c.Known&knownBest != 0 {
			d.best = c.Best
		} else if rewound {
			d.best = 0
		}
		actual := num(c.Compound, c.Known&knownTyre != 0)
		visual := num(c.Visual, c.Known&knownTyre != 0)
		if c.Known&knownLaps != 0 {
			previous := d.laps
			if c.Laps > previous+1 || (!d.hasLap && c.Laps > 0) {
				r.partial = true
			}
			if c.Laps > 0 && (!d.hasLap || c.Laps > previous || rewound) {
				if c.Known&knownLast != 0 && extra.Known&analysisLap != 0 {
					lap := analysisLapRow{time: uint64(c.Last), elapsed: elapsed, valid: extra.LapFlags&2 != 0}
					if d.tyreKnown && c.Laps == previous+1 {
						lap.actual, lap.visual = num(d.tyre.actual, true), num(d.tyre.visual, true)
					} else if !rewound {
						lap.actual, lap.visual = actual, visual
					}
					if c.Known&knownLastSectors != 0 {
						lap.sectors = c.LastSectors
					}
					d.recorded[c.Laps] = lap
				} else {
					r.partial = true
				}
			}
			d.laps, d.hasLap = c.Laps, true
		}
		if c.Known&knownTyre != 0 {
			d.tyre = analysisTyre{c.Compound, c.Visual}
			if _, exists := d.compounds[d.tyre]; !exists {
				d.compounds[d.tyre] = d.laps
			}
			d.tyreKnown = true
		}
		if extra.Known&analysisLap != 0 {
			d.clock = extra.Current
		}
		if err := r.samples.Write(sampleRow(f, c, d, elapsed, r.epoch, actual, visual)); err != nil {
			return err
		}
	}
	return nil
}

func sampleRow(f Frame, c Car, d *analysisDriver, elapsed uint64, epoch int, actual, visual string) []string {
	x := c.Analysis
	timing := x.Known&analysisLap != 0
	last := c.Known&knownLastSectors != 0
	best := c.Known&knownSectors != 0
	return []string{
		num(elapsed, true), num(epoch, true), num(c.Vehicle, true), analysisText(d.name), analysisText(d.team),
		num(c.Position, c.Known&knownPosition != 0), num(c.Grid, c.Known&knownGrid != 0), num((c.Flags&resultMask)>>8, c.Known&knownResult != 0),
		num(x.Lap, x.Known&analysisProgress != 0), num(c.Laps, c.Known&knownLaps != 0), num(x.Current, timing),
		num(x.CurrentS1, timing && x.CurrentS1 > 0), num(x.CurrentS2, timing && x.CurrentS2 > 0), num(x.LapFlags, timing),
		num(c.Last, c.Known&knownLast != 0), num(c.LastSectors[0], last), num(c.LastSectors[1], last), num(c.LastSectors[2], last),
		num(c.Best, c.Known&knownBest != 0), num(x.BestLap, timing && x.BestLap > 0),
		num(c.BestSectors[0], best), num(c.BestSectors[1], best), num(c.BestSectors[2], best),
		decimal(x.LapDistance, x.Known&analysisProgress != 0), decimal(x.Speed, x.Known&analysisSpeed != 0),
		decimal(x.Throttle, x.Known&analysisPedals != 0), decimal(x.Brake, x.Known&analysisPedals != 0), num(x.Gear, x.Known&analysisGear != 0),
		decimal(x.RPM, x.Known&analysisRPM != 0), num(c.GapAhead, c.Known&knownGapAhead != 0), num(x.GapLeader, x.Known&analysisGap != 0),
		actual, visual, num(c.TyreAgeLaps, c.Known&knownTyreAge != 0), num(x.Stint, c.Known&knownTyreAge != 0),
		decimal(c.Wear[2], c.WearKnown&4 != 0), decimal(c.Wear[3], c.WearKnown&8 != 0), decimal(c.Wear[0], c.WearKnown&1 != 0), decimal(c.Wear[1], c.WearKnown&2 != 0),
		truth(c.Flags&inPitLane != 0, c.Known&knownPit != 0), num(x.PitStatus, x.Known&analysisPit != 0), num(x.PitStops, x.Known&analysisPit != 0),
		num(f.Analysis.SafetyCar, f.Analysis.SafetyCarKnown), truth(f.Analysis.Chequered, true),
		num(c.DriveThrough, c.Known&knownPenalties != 0), num(c.StopGo, c.Known&knownPenalties != 0), num(c.TimePenaltyMS, c.Known&knownTimePenalty != 0),
		num(f.Progress, f.State&sessionTimer != 0), num(f.Progress>>16, f.State&sessionLaps != 0),
	}
}

func (r *analysisRecording) save(state, reason string, now time.Time) error {
	laps := [][]string{lapsCSV}
	drivers := [][]string{driversCSV}
	for vehicle, d := range r.drivers {
		if !d.present {
			continue
		}
		id, name, team := num(vehicle, true), analysisText(d.name), analysisText(d.team)
		var sum, valid uint64
		for lap := uint32(1); lap <= d.laps; lap++ {
			l, ok := d.recorded[lap]
			if !ok {
				continue
			}
			laps = append(laps, []string{id, name, team, num(lap, true), num(l.elapsed, true), num(l.time, true), truth(l.valid, true),
				num(l.sectors[0], l.sectors[0] > 0), num(l.sectors[1], l.sectors[1] > 0), num(l.sectors[2], l.sectors[2] > 0), l.actual, l.visual})
			if l.valid {
				sum += l.time
				valid++
			}
		}
		tyres := make([]string, 0, len(d.compounds))
		for compound := range d.compounds {
			tyres = append(tyres, analysisCompound(compound.actual, compound.visual))
		}
		slices.Sort(tyres)
		tyres = slices.Compact(tyres)
		var average uint64
		if valid > 0 {
			average = (sum + valid/2) / valid
		}
		drivers = append(drivers, []string{id, name, team, num(d.laps, d.hasLap), num(d.best, d.best > 0), strings.Join(tyres, " / "), num(average, valid > 0), num(valid, true)})
	}
	if err := errors.Join(writeAnalysisCSV(r.dir, "laps.csv", laps), writeAnalysisCSV(r.dir, "drivers.csv", drivers)); err != nil {
		return err
	}
	coverage := "observed"
	if r.partial {
		coverage = "partial"
	}
	row := []string{
		"1", analysisText(r.profile), analysisText(r.career), num(r.season+1, r.season >= 0 && r.round > 0), num(r.round, r.round > 0),
		analysisText(r.track), r.session, num(r.sessionType, true), num(r.kind, true), r.started.UTC().Format(time.RFC3339Nano),
		now.UTC().Format(time.RFC3339Nano), state, reason, coverage, num(r.key.run, true), num(r.key.attempt, true), addonVersion,
	}
	return writeAnalysisCSV(r.dir, "session.csv", [][]string{sessionCSV, row})
}

func writeAnalysisCSV(dir, name string, rows [][]string) error {
	tmp, err := os.CreateTemp(dir, ".csv-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	writer := csv.NewWriter(tmp)
	err = writer.WriteAll(rows)
	if err := errors.Join(err, tmp.Close()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, name))
}
