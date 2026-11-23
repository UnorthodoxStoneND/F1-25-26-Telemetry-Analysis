# Telemetry Analysis prototype

Enable **Telemetry Analysis** under the add-on settings before starting a session. The setting is off by default and follows the active launcher profile. Recording continues while another launcher page is open, in either live display mode. Select a saved session on **Telemetry Analysis** after it ends. Practice rankings use the fastest valid lap reported by Live State. Average time uses the recorded valid completed laps.

## Data files

The helper uses the system Documents location on Windows and the XDG Documents location on Linux. Open **F1 Live Telemetry / Recordings** to browse data by launcher profile, career save, season and round when those identifiers are available. Otherwise the folder groups sessions by launcher profile, game launch and circuit. Each observed attempt has a separate directory.

| File | Contents |
| --- | --- |
| session.csv | Schema version, profile, available save and round data, circuit key, session type, UTC timestamps and recording status. |
| drivers.csv | Driver, team key, compounds observed, completed laps, best valid lap, observed average and the count used for that average. |
| laps.csv | Canonical observed completed laps with validity, available sectors and compound IDs. |
| samples.csv | Per-car Live State observations at up to 5 Hz. Includes timing, position, gaps, tyre data, pit/flag states and available speed, throttle, brake, gear and RPM channels. Schema 2 includes `red_flag_status`. |
| flags.csv | Session-level red-flag changes with elapsed milliseconds. Schema 2 records `none`, `deployed`, `suspended` and loss of the previously available signal as `unavailable`. |

Files are UTF-8 CSV. Blank cells indicate unavailable data. Headers specify units, and the sample clock measures elapsed milliseconds since the first recorded observation. Each driver ID is a vehicle slot within that attempt. On a detected Flashback, samples continue in one new epoch for the backward sequence. The rewritten laps file drops the superseded lap records and preserves the recorded compounds of completed laps that survive. Interruptions and known missed lap observations mark the recording accordingly. Closing the game or helper can leave an interrupted recording with the last checkpoint.

The flag log continues when the retro window is hidden or vehicle samples are paused, whenever Live State reports the session. The log records the first available status and subsequent changes. Earlier recordings use schema 1.

Use **Export raw data** to copy the selected session's CSV files into a new folder under **F1 Live Telemetry / Exports** and open that folder. You can also open the original files in a spreadsheet or another analysis tool.

## Prototype coverage

Practice has an overview and rankings. Qualifying, sprint qualifying, sprint and race can be recorded and exported. Weather, advanced session visuals, season analysis and visuals export are outside phase one. The circuit banner recognises stock track keys and shows the raw key for custom tracks. The overview formats known team keys from the Live State badge service. When the service lacks a key, the recorder reads the launcher-provided session-entrant team ID and uses the Swofty game database to resolve a team label when available. User-defined entrants display as My Team. Trace availability depends on the game build and which cars Swofty reports. Linux support is experimental.
