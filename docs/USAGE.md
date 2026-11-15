# UDP Telemetry v0.3.9

A UDP receiver, live page and saved **Telemetry Analysis** page for **Swofty Launcher 1.9.42+ on Windows**, targeting F1 25 and the 2026 Season Pack on Steam, including My Team. Version 0.3 adds race analysis graphics with shared multi-driver selection.

## Install

1. In Swofty, open **Addons → Install addon → Choose a file** and select `UDP_Telemetry_v0.3.9.swoftyaddon` (or drag it onto the launcher). This replaces older versions using the same addon ID and preserves the existing recordings, save names and port/display preferences. Fully restart Swofty after updating; the analysis subtitle must show **v0.3.9** to confirm the current helper is running.
2. Enable **UDP Telemetry** in your active profile. **UDP Telemetry** and **Telemetry Analysis** appear in the sidebar.
3. In the game, open **Settings → Telemetry Settings**:
   - UDP Telemetry: On
   - UDP Broadcast Mode: Off
   - UDP IP Address: `127.0.0.1`
   - UDP Port: `20777` for a new install, or the existing saved port (match **Addons → UDP Telemetry → Settings → UDP port**)
   - UDP Format: `2025` or `2026`; detected automatically
   - UDP Send Rate: `20 Hz`
4. Enter a session. Keep Swofty open and select the telemetry page to view updates.

No Python, Go or other runtime is needed to use the included Windows executable. This addon has no game-native DLL and changes no car handling, game memory, game files or career-save contents. Driving measurements still come exclusively from UDP. The Windows helper also uses read-only access to the running game to capture team identity metadata on the verified Steam build. Swofty supplies game-running/stopped status, native addon preferences and active language/ID mapping data.

## Telemetry Analysis

Version 0.3 reprocesses existing journals into the new race views; original recordings are not rewritten. The first red-light event remains the start boundary, excluding formation samples for every driver while retaining pre-lights-out rev-up samples. Equal-time samples are ordered by packet frame so the zero-time start sequence stays in order. Race lap-one traces include the grid-to-timing-line portion, shown at negative lap distance when the car starts behind the line. The final tyre stint uses the classified completed-lap count when the game still reports end lap 255; ended recordings without classification remain explicitly unknown.

Practice and qualifying recording starts automatically when valid 2025/2026 UDP packets arrive. For races and sprints, driving capture starts at the first received start-light event (STLG with a visible red light), before lights out. The latest session, participant, setup and tyre-status context is buffered beforehand; formation driving measurements are excluded from new race journals. All 22/24 car slots use the same start boundary. Red-light/rev-up samples remain part of lap one, including zero-session-time frames. If start events are lost or recording begins mid-race, capture falls back to available verified racing data and records the trigger in session metadata; it cannot reconstruct an unreceived start. Keep Swofty open with the addon enabled; either page may be closed or another launcher page selected. Nothing has been added to the Live Telemetry layout or controls.

The top selectors are **Save → Race weekend → Session**, with a **Season** selector included only when the chosen save contains multiple recorded seasons. Single-season saves use one three-column selector row. A recording initially appears under **Unassigned <identifier>**. Choose **Edit save name**, enter a friendly name and apply **Save name** once to associate that season. Enter an existing save name to associate another season with the same save. The editor is hidden normally and closes after saving, cancelling or changing the session filters. Association is stored locally and reused for subsequent sessions with the same launcher profile and UDP season link. The season selector displays the game's link identifier, not a guessed career year. Unknown weekend links remain separate recordings. **Refresh** and **Edit save name** share one small action row. The old Recorded/Status/Packets facts table is omitted; recording metadata remains saved in JSON and session date/status remain in the session selector.

This manual association is necessary because UDP does not expose the career save's friendly name. Distinct profiles/season identifiers are not merged automatically. Association across real career saves/reloads still needs verification in the game.

**Practice shows Overview first, followed by Session Analysis. Qualifying and sprint qualifying show Qualifying Results first, followed by Advanced Telemetry.** Race and sprint sessions open on **Overview**, followed by **Lap Analysis**, **Pit Stops**, **Race Strategy** and **Race Progression**. The separate race Laps & Sectors tab is removed; race timing is beside the traces. Overview keeps the existing results and statistics. Choosing a new session opens its results tab. Background recording continues as before.

Across the four race analysis tabs, **driver chips** control one shared selection. Click a name to select/deselect it; selected names use the launcher accent highlight. **All** and **Clear** sit alongside the chips. The driver chips use one continuous row across the page rather than forced groups of eight. Any subset is supported, with the player selected by default for a new recording. Overview always shows the full results and has no driver filter. Driver colors stay consistent across charts. Charts are static images; selectors update them, but there are no hover tooltips, drag zoom or clickable plot legends. Pit in/out, safety-car, VSC and invalid laps remain included without condition markers or a condition legend.

- **Lap Analysis:** choose one race lap. Five separate graphics on the left overlay the selected drivers' speed, throttle, brake, gear and RPM against lap distance. A native table on the right shows their lap time and three sectors. Race pit-lane samples remain in the traces. Missing or incomplete measurements stay unknown; unavailable driving inputs are not reconstructed.
- **Pit Stops:** a slim tyre-strategy timeline with rounded stint bars, mean total pit-lane duration per selected driver, and stacked stint-length columns. The pit-average chart sorts quickest first and uses thin lines with endpoint dots, actual means and differences from the best selected mean. The pit average includes completed visits with a recorded stationary stop and excludes drive-through events; it uses the full pit-lane timer, not just stationary service time. Compound labels are actual C numbers; colors use the event's recorded Soft/Medium/Hard mapping. Stint segments follow chronological order. Partial visits or missing timers cannot contribute a reliable pit average.
- **Race Strategy:** full-width position flow with straight lap-to-lap lines and individual position ticks, plus a full-width gap-to-leader chart. The gap uses the last valid UDP gap on that driver's lap. Missing finish gaps can use classified elapsed times plus penalties for finished cars that completed the winner's distance. This does not invent missing lap times; other unavailable gaps stay blank. Live samples are not guaranteed to coincide exactly with the timing line.
- **Race Progression:** a full-width lap-by-lap progression chart with dashed segments touching pit in/out laps and per-driver best/average cards, a full-race sector-duel timeline for all selected drivers, a full-width Pace vs Field chart with clean-lap points, a trailing three-lap trend, a zero median reference, per-driver averages and the whole field's shaded interquartile range, and lap-time distributions. Pace vs Field excludes pit in/out, SC/VSC and invalid laps from both the selected drivers and field baseline; trends restart across excluded or missing laps. The distribution keeps all completed timed laps, uses an M:SS axis and shows median, quartiles, 1.5-IQR whiskers and individual laps. The full-width sector duel has one column per race lap and three sector rows. Cell colors identify the fastest selected driver, with sector-win totals above. Tied winners share a cell and each receive a win; a missing selected-driver sector leaves that cell blank. Pit/SC/VSC/invalid laps remain included. Missing final-lap times remain absent even when classification confirms the completed race distance.

With larger selections the strategy and progression charts use full-width layouts to keep driver labels readable. Selecting the full grid can make overlaid lines busy; the selection remains available on every analysis tab.

A compact, 72-pixel-high summary strip above the tabs shows **Country, Circuit, Laps, Air temp, Track temp and Weather**. Country/circuit names are resolved from the UDP track ID. Race laps are the configured distance; timed practice/qualifying show **Timed session**. Temperatures and weather use the latest valid recorded Session packet in the current analysis snapshot, and **Refresh** updates an ongoing snapshot. Weather uses the game's Clear/Light cloud/Overcast/Light rain/Heavy rain/Storm labels, without claiming measured wet/dry surface state or wind speed. Missing measurements remain `—`; genuine zero and negative temperatures are preserved. Flashbacks restore the corresponding earlier conditions. Existing journals can be reprocessed into this summary.

The strip uses six small accent cards with original icons, Windows UI typography and a shared compact baseline. It is generated locally as a high-DPI PNG with native-stat fallback; it does not require a browser or extra runtime.

- **Practice Overview:** POS with gold/silver/bronze top-three highlights, Full Driver Name, Team, Compounds, Laps, Fastest Lap, Gap and Avg Time. Gap compares fastest valid laps. Average uses valid completed laps; the lap count includes all completed timed laps or the classified completed count when higher. Missing averages remain unknown; recorded custom names are preserved.
- **Practice Session Analysis:** Team Sector Leaders shows the top five teams in each sector, using the best captured sector from any valid completed team lap across all drivers. The fastest time is absolute and subsequent teams show the gap. Team colors are consistent across sectors, with no team logos. Driver chips control the separate full-width Lap Time Distribution and Consistency Analysis graphics. Distribution uses valid completed laps with an M:SS axis. Consistency boxes show mean ± population standard deviation with min/max whiskers, plus SD and valid-lap counts. At least two valid laps are required for a driver's consistency graphic. Team-sector leaders remain independent of the driver selection. Laps & Sectors is removed for practice, qualifying and sprint qualifying.

- **Qualifying Results (normal and sprint qualifying):** short and one-shot formats use the same eight-column table as practice. Full formats display Q3/Q2/Q1 or SQ3/SQ2/SQ1 side by side, with position, full driver name where known, team, best valid time and gap. Recorded eliminations are grouped below continuing drivers. Missing stages show Not recorded.
- **Advanced Telemetry (normal and sprint qualifying):** a qualifying timeline plots each driver's gap to the selected stage's fastest valid lap, with a stage selector when multiple stages were recorded. Observed advances/final-stage results use filled heads; recorded eliminations use hollow heads. Unknown outcomes stay neutral; a missing next stage does not establish elimination. The cutoff is based on the recorded completed next stage, rather than a fixed field-size assumption. All recorded gaps remain visible, including those over five seconds.
- **Knockout progression:** stage-by-stage ranking with shared teammate colors, dashed second-driver lines and hollow eliminated endpoints. Missing stages remain gaps. This chart is omitted for single-stage formats.
- **Teammate head-to-head:** each driver's fastest valid lap across all available stages, regardless of which stage it came from. The leading driver is named first; bar lengths show the time difference. The stage label identifies the source of each best lap. Pairs without two valid times are omitted; ambiguous/unknown team identities are not grouped automatically.

Stages are linked only within matching profile, UDP season/weekend identifiers, track and qualifying family. Normal and sprint qualifying never merge. A new Q1/SQ1 bounds another attempt; selection also avoids later retries of the selected stage and older later-stage records. Missing identifiers keep the current recording separate. Stage loading runs in the background, peer models retain timing/identity rather than driving traces, and Refresh reloads the available stage set. No original journal is rewritten.

- **Overview:** fastest lap and classification, sorted numerically by position with unknown positions last. Practice, qualifying and sprint qualifying omit the Grid column. Race-type sessions additionally show winner classification duration before penalties in HH:MM:SS, completed race laps, captured overtake events, and distinct leader laps affected by SC/VSC. Race results use POS (gold/silver/bronze for the top three), driver full name where a known surname and race number match, Team, Compounds used, Gap, Grid and +/- positions gained/lost. Custom or unidentified driver names remain as recorded. Finish gaps include classification penalties; lapped cars show laps behind and retired/disqualified cars show their status. Formation-lap safety cars are excluded. These race statistics are hidden in practice and qualifying.

Analysis is a snapshot. Use **Refresh** to reread an ongoing session or pick up late history/classification packets. Session loading runs in the background so long recordings do not block the launcher protocol. Empty/missing measurements display as `—`; event totals describe the captured portion, not guaranteed whole-session totals. Restricted remote-human traces and wear are not presented as real zero readings.

Qualifying/practice may reuse a lap number across an aborted attempt, a garage stay, an out-lap and a new flying lap. Preview 2 selects the latest timed attempt and excludes garage/in/out-lap samples from comparisons, rather than overlaying all traversals. On-track race samples remain included. Partial timed runs are labelled when the start or completion was not captured. Existing preview-1 journals are reprocessed without requiring a new session; cached preview-1 charts are not reused.

## Saved team names

Game text operations such as McLaren's `{o:upper}`/`{o:lower}` formatting are converted to plain display text, including in snapshots recorded before v0.2.13. Literal custom names retain their own punctuation.

New recordings automatically capture all loaded team records, including custom constructor names and the active My Team name. The full identity snapshot is saved in the session JSON under `team_snapshot`, alongside process/build provenance and the active game mode. Names are resolved using the installed game language baseline, mounted Swofty language tables and applicable runtime string overrides. Raw name keys are retained as well.

Overview and Qualifying Results include a **Team** column. UDP participant identities are matched to the saved records; native entrant identities and race numbers help disambiguate Swofty-rewritten IDs. Unavailable or ambiguous matches remain unknown. Names are retained with that session after the game closes or a different save is loaded. Older recordings without a snapshot are not retrospectively assigned names from the current save.

Identity capture is external and read-only, restricted to the previously verified Steam executable (SHA-256 `d873a1c8d2044cef2fd76c99c9d5a97649c5b815d055540995105f46619865ae`). Unsupported builds, access failures or temporarily unavailable records do not interrupt UDP recording. Capture retries while the session is recording. The Live Telemetry screen is unchanged. The finished reader still needs its next live-race check; automated tests use synthetic memory and saved snapshots, not a reopened game.

## Local storage and recovery

Swofty 1.9.42 only displays images inside its own addon data/cache roots. The helper therefore creates disposable, content-addressed display copies of overview strips, trace charts and tyre pills in the launcher-provided cache. The original images and all recordings, metadata and preferences remain in Documents\F1 Game Telemetry. Display copies are recreated after cache clearing; failed overview-cache writes fall back to the six native statistics rather than a blank bar. These cache files are not session backups.

All addon-created recordings, settings and generated images live in **Documents\F1 Game Telemetry**. The Windows Known Folder API locates Documents correctly when it is redirected or managed by OneDrive. Recordings are in its `recordings` subfolder. On the first launch after updating, existing addon data is copied from Swofty's old addon-data folder without overwriting files in the new location. Original files remain as a backup. A migration marker prevents deleted recordings from being reimported on later starts. Launcher-owned profile settings remain managed by Swofty. Each attempt has a JSON metadata file and a compressed `.udp.gz` journal containing the original datagrams. They survive session exit and launcher restart. No telemetry is uploaded. Back up both files together; to remove old recordings manually, close Swofty first. There is no automatic retention limit in this preview, so disk usage grows with recording time.

Journals are flushed and synced approximately every second, and closed on normal helper shutdown. A crash can lose the unsynced tail. Recovery reads complete records up to an incomplete tail; sessions left recording are marked interrupted. `SEND`/final classification marks completion, explicit restarts create separate attempts, and flashback analysis removes superseded traces/events/pit timings. UDP silence alone never finalizes a session, because it could be a pause. Confirmed game exit or a new session closes an unfinished attempt as interrupted.

From preview 3, completion is not a UDP capture cutoff. The recorder retains all arriving same-session packets after final classification and `SEND`, until explicit menu/new-session/game-exit evidence or helper shutdown closes that journal. Live Telemetry still clears at session end. Late lap packets cannot overwrite final classification or authoritative history times. Completed snapshots seal their gzip footer; later packets append another member. Existing sessions cannot recover packets discarded by older previews.

New recording metadata includes `classification_time`, `session_end_time`, `last_telemetry_time`, `telemetry_packets_after_classification` and `telemetry_packets_after_session_end` when available. The counters measure car-telemetry packets arriving after those events, including delayed packets; they do not prove the game continued simulating cars beyond session end. These diagnostics are stored in the JSON, with no extra Live Telemetry controls.

The saved 24-car, 20-lap race has been replayed read-only for the new analysis views, including full-grid chart generation. Native v0.3 page layout and interaction still need a check after importing into Swofty; no new race recording is required to view the new charts.

## Display

Change **Drivers shown**, **Highlight my row** and **UDP port** in **Addons → UDP Telemetry → Settings**, on Swofty's native addon panel. These controls are not on the telemetry page. Native display defaults are Nearest and highlight off. Changes take effect on the telemetry page's next refresh. The UDP port dropdown provides common presets; **Use saved port** keeps the port from earlier versions, including a custom one. New installs use 20777. The helper reads the profile's saved options and never writes the launcher profile.

- **Nearest cars:** you plus four drivers by classification, two ahead/two behind where possible. At either end of the grid, fill from the other side. Before positions arrive, rows use participant order. With fewer than five participants, show those available.
- **Full grid:** all session participants, ordered by position.
- **None:** no driver rows or setup panels.
- Each row shows position, driver, actual compound, four individual tyre wear percentages with filling bars, average wear with a filling bar, last lap and session-best valid lap. Bars show wear used: green below 50%, amber from 50%, red from 75%.
- **Highlight my row** optionally accents the player's row. It is off by default and saved in Swofty's active profile. Driver names have no YOU suffix.
- Compound pills show C0–C6, INT or WET. A C3 can be red, yellow or white depending on whether it is Soft, Medium or Hard at this event.
- Use a driver's **Setup** switch to expand a compact inspector directly beneath that driver. Only one setup is open at a time. It shows player-facing setup fields in paired setting/value columns. Legacy UDP-only engine braking and ballast fields are omitted. The inspector does not open a side panel, add a page or change the car. This separate control is retained because Swofty 1.9.42 does not let addons force native click-to-open table details underneath a row on wide windows.

The receiver accepts grid data as soon as UDP arrives, even while Swofty's running status is catching up. Unknown initial metadata does not end a session, and the first start event preserves already received grid samples. Grid positions are used until race positions are available. When the player index is still unknown, Nearest temporarily shows the received grid rather than hiding everyone. No receiver can show measurements the game has not transmitted.

The receiver continues when you navigate elsewhere in Swofty. The page refreshes approximately once a second. Pauses and temporary packet silence retain the most recent sample; the header makes that clear. Confirmed game exit and UDP `SEND` clear session data; a new session starts cleanly. A lost UDP session-end packet cannot be distinguished from an indefinite pause while the game remains running; a subsequent session or confirmed game exit clears it.

## Availability and limitations

- The game must transmit the relevant packet. Until then, unknown data displays as `—`. A freshly fitted tyre's genuine zero wear displays as `0.0%`.
- Restricted remote human tyre wear is unavailable. Your own car is readable regardless of its privacy setting. AI values reflect what the game simulates and transmits.
- Best laps come from **Session History**, including valid completed laps sent after joining an existing session. They may take several seconds to arrive across the grid. Invalid laps do not count.
- Setup information may be unavailable for some cars/modes. The addon does not invent missing values. Setup fuel load is the setup value, not current fuel in the tank.
- The game’s unsupported compound IDs display as `—` rather than guessing a C compound.
- Use a different port if another telemetry receiver already owns `20777`; change it both here and in the game. This version binds only to loopback on this PC.
- Automated synthetic-packet and launcher-protocol tests pass. The actual live display, My Team game output and game-status transitions still need an in-game check on Steam/Swofty 1.9.42.

## Changelog in Swofty

The release notes are in **CHANGELOG.md**, ready to use as the version's changelog when publishing through Swofty's Addon Browser. Swofty 1.9.42 gets its native addon changelog panel from the published catalog; it does not read this file for locally installed packages. This package does not create a separate changelog page. It has not been published to the Addon Browser.

## Build and test

Source is in `source/`, with no third-party dependencies. With Go 1.24+:

```powershell
cd source
go test ./...
go vet ./...
go build -trimpath -ldflags '-s -w -H=windowsgui' -o ../bin/udp-telemetry.exe .
```

`source/integration_test.py` tests the built executable with synthetic UDP and a simulated launcher host. It requires Python 3 for development only. Run `python source/integration_test.py` from the addon root.

`source/analysis_integration_test.py` additionally checks recording, hierarchical selection, save association, both-format charts/tables and process restart. It uses private loopback ports and isolated data folders, leaving the installed addon/profile untouched.

## References

- [EA UDP specifications for F1 25 and the 2026 Season Pack](https://forums.ea.com/blog/f1-games-game-info-hub-en/ea-sports%E2%84%A2-f1%C2%AE25-2026-season-pack-udp-specification/12187347)
- [Swofty launcher plugin protocol](https://f1.swofty.net/reference/addons/launcher-api/protocol.html)
- [Live pages](https://f1.swofty.net/reference/addons/launcher-api/live-pages.html)
- [Tables and expandable rows](https://f1.swofty.net/reference/addons/launcher-api/tables.html)
- [Built-in addon changelogs](https://f1.swofty.net/releases/1.8.12.html)

Original addon implementation by UnorthodoxStoneNDmodding with Codex. Not affiliated with EA or Swofty.
