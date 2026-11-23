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
	RedFlag              redFlagStatus
	Track, Career        string
}

type AnalysisCar struct {
	Known, Current, BestLap, CurrentS1, CurrentS2 uint32
	GapLeader, Gear, Lap                          int32
	Speed, Throttle, Brake, RPM, LapDistance      float32
	LapFlags, PitStatus, PitStops, Stint          uint8
	Team                                          string
}

const (
	analysisFolder = "F1 Live Telemetry"
	analysisSchema = "2"
	flagsFileName  = "flags.csv"
)

type redFlagStatus uint8

const (
	redFlagUnavailable redFlagStatus = iota
	redFlagNone
	redFlagDeployed
	redFlagSuspended
)

func (s redFlagStatus) label() string {
	switch s {
	case redFlagNone:
		return "none"
	case redFlagDeployed:
		return "deployed"
	case redFlagSuspended:
		return "suspended"
	default:
		return ""
	}
}

var sessionCSV = []string{"schema", "profile", "career_id", "season", "round", "track_key", "session", "type", "kind", "started_utc", "updated_utc", "state", "reason", "coverage", "run", "attempt", "addon_version"}
var sampleCSV = []string{"elapsed_ms", "epoch", "driver_id", "driver", "team_key", "position", "grid", "result", "lap", "laps_completed", "current_lap_ms", "current_s1_ms", "current_s2_ms", "lap_flags", "last_lap_ms", "last_s1_ms", "last_s2_ms", "last_s3_ms", "best_lap_ms", "best_lap_number", "best_s1_ms", "best_s2_ms", "best_s3_ms", "lap_distance_m", "speed_m_s", "throttle", "brake", "gear", "rpm", "gap_ahead_ms", "gap_leader_ms", "actual_compound_id", "visual_compound_id", "tyre_age_laps", "stint_laps", "wear_fl_pct", "wear_fr_pct", "wear_rl_pct", "wear_rr_pct", "in_pit_lane", "pit_status", "pit_stops", "safety_car_status", "chequered", "drive_through", "stop_go", "time_penalty_ms", "session_time_left_s", "session_total_laps", "red_flag_status"}
var flagsCSV = []string{"elapsed_ms", "red_flag_status"}
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
	key                    analysisAttempt
	dir, profile           string
	session, track         string
	career                 string
	season, round          int32
	sessionType, kind      uint32
	started, checkpoint    time.Time
	startSample, sample    uint64
	epoch                  int
	lastRewind             uint64
	partial, chequered     bool
	drivers                [maxCars]analysisDriver
	samplesFile, flagsFile *os.File
	samples, flags         *csv.Writer
	lastRedFlag            redFlagStatus
}

type analysisDriver struct {
	present, hasLap bool
	name, team      string
	laps            uint32
	clock           uint32
	distance        float32
	progressKnown   bool
	wasInPit        bool
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
		err := errors.Join(r.flush(), r.samplesFile.Sync(), r.flagsFile.Sync(), r.samplesFile.Close(), r.flagsFile.Close())
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
	if key.run == 0 || key.attempt == 0 || a.err != nil {
		return
	}
	canSample := s.Sample != 0 && f.State&(active|windowReady|paused|snapshotStale) == active|windowReady &&
		f.SessionType != 18 && f.SessionKind != 5 && (f.SessionType != 0 || f.SessionKind != 0)
	if a.current == nil {
		if !canSample {
			return
		}
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
	// Session flags remain observable while paused, suspended or outside
	// the window's gameplay gate. Do not require a fresh set of driver rows.
	if err := r.observeRedFlag(f); err != nil {
		a.close("interrupted", "flag write failed", now)
		a.err = err
		return
	}
	if canSample && s.Sample > r.sample {
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
			return
		}
	}
	if now.Sub(r.checkpoint) >= 5*time.Second {
		if err := errors.Join(r.flush(), r.save("recording", "", now)); err != nil {
			a.close("interrupted", "recording write failed", now)
			a.err = err
			return
		}
		r.checkpoint = now
	}
}

func (r *analysisRecording) flush() error {
	r.samples.Flush()
	r.flags.Flush()
	return errors.Join(r.samples.Error(), r.flags.Error())
}

func (r *analysisRecording) observeRedFlag(f Frame) error {
	tick := f.Tick
	if tick == 0 {
		tick = f.Analysis.Sample
	}
	if tick < r.startSample {
		return nil
	}
	status := f.Analysis.RedFlag
	label := status.label()
	if label == "" {
		status, label = redFlagUnavailable, "unavailable"
	}
	if status == r.lastRedFlag {
		return nil
	}
	if err := r.flags.Write([]string{num(tick-r.startSample, true), label}); err != nil {
		return err
	}
	r.lastRedFlag = status
	return nil
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
	if r.samplesFile, err = os.OpenFile(filepath.Join(dir, "samples.csv"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600); err != nil {
		return nil, err
	}
	r.flagsFile, err = os.OpenFile(filepath.Join(dir, flagsFileName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, errors.Join(err, r.samplesFile.Close())
	}
	r.samples, r.flags = csv.NewWriter(r.samplesFile), csv.NewWriter(r.flagsFile)
	if err = errors.Join(r.samples.Write(sampleCSV), r.flags.Write(flagsCSV), r.flush(), r.save("recording", "", now)); err != nil {
		return nil, errors.Join(err, r.samplesFile.Close(), r.flagsFile.Close())
	}
	return r, nil
}

// A clock reset at the timing line can happen before the completed-lap
// count changes (including at the end of an out-lap).
func playerRewound(c Car, d *analysisDriver) bool {
	x := c.Analysis
	if x.Known&analysisLap == 0 || c.Flags&inPitLane != 0 || d.wasInPit ||
		x.Current == 0 || uint64(x.Current)+1000 >= uint64(d.clock) ||
		x.Known&analysisProgress == 0 || !d.progressKnown {
		return false
	}
	if x.Current <= 3000 && x.LapDistance < 250 && d.distance > 1000 {
		return false
	}
	return x.LapDistance+3 < d.distance
}

func (r *analysisRecording) rewind(f Frame) bool {
	for _, c := range f.Cars {
		d := &r.drivers[c.Vehicle]
		if c.Known&knownLaps == 0 || !d.hasLap {
			continue
		}
		if c.Laps < d.laps || (int32(c.Vehicle) == f.Player && c.Laps == d.laps && playerRewound(c, d)) {
			return true
		}
	}
	return false
}

func (r *analysisRecording) record(f Frame) error {
	r.sample = f.Analysis.Sample
	elapsed := r.sample - r.startSample
	rewound := r.rewind(f)
	if rewound {
		// While Flashback is moving backwards, it publishes several samples
		// per action. Keep these in the same epoch across short pauses.
		if r.lastRewind == 0 || r.sample-r.lastRewind >= 750 {
			r.epoch++
		}
		r.lastRewind = r.sample
		r.chequered = false
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
			r.observeLaps(c, d, elapsed, rewound, actual, visual)
		}
		if c.Known&knownTyre != 0 {
			d.tyre = analysisTyre{c.Compound, c.Visual}
			if _, exists := d.compounds[d.tyre]; !exists {
				d.compounds[d.tyre] = d.laps
			}
			d.tyreKnown = true
		}
		d.clock = 0
		if extra.Known&analysisLap != 0 {
			d.clock = extra.Current
		}
		d.distance = extra.LapDistance
		d.progressKnown = extra.Known&analysisProgress != 0
		d.wasInPit = c.Flags&inPitLane != 0
		if err := r.samples.Write(sampleRow(f, c, d, elapsed, r.epoch, actual, visual)); err != nil {
			return err
		}
	}
	return nil
}

func (r *analysisRecording) observeLaps(c Car, d *analysisDriver, elapsed uint64, rewound bool, actual, visual string) {
	previous := d.laps
	rolledBack := rewound && (!d.hasLap || c.Laps <= previous)
	if rolledBack {
		for lap := range d.recorded {
			if lap > c.Laps {
				delete(d.recorded, lap)
			}
		}
		for tyre, firstLap := range d.compounds {
			if firstLap >= c.Laps {
				delete(d.compounds, tyre)
			}
		}
		// The pre-rewind fitted set might belong to the superseded timeline.
		d.tyreKnown = false
	}
	if c.Laps > previous+1 || (!d.hasLap && c.Laps > 0) {
		r.partial = true
	}
	d.laps, d.hasLap = c.Laps, true
	if c.Laps == 0 || (c.Laps <= previous && !rolledBack) {
		return
	}
	prior, recorded := d.recorded[c.Laps]
	if c.Known&knownLast == 0 || c.Analysis.Known&analysisLap == 0 {
		if !recorded {
			r.partial = true
		}
		return
	}
	lap := analysisLapRow{time: uint64(c.Last), elapsed: elapsed, valid: c.Analysis.LapFlags&2 != 0}
	if recorded && prior.time == lap.time {
		// This completed lap survived the rollback. Keep the tyre and
		// original completion sample captured at its actual timing line.
		lap.elapsed, lap.actual, lap.visual, lap.sectors = prior.elapsed, prior.actual, prior.visual, prior.sectors
	} else if d.tyreKnown && c.Laps == previous+1 {
		lap.actual, lap.visual = num(d.tyre.actual, true), num(d.tyre.visual, true)
	} else if !rolledBack {
		lap.actual, lap.visual = actual, visual
	}
	if c.Known&knownLastSectors != 0 {
		lap.sectors = c.LastSectors
	}
	d.recorded[c.Laps] = lap
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
		num(f.Progress, f.State&sessionTimer != 0), num(f.Progress>>16, f.State&sessionLaps != 0), f.Analysis.RedFlag.label(),
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
		analysisSchema, analysisText(r.profile), analysisText(r.career), num(r.season+1, r.season >= 0 && r.round > 0), num(r.round, r.round > 0),
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
