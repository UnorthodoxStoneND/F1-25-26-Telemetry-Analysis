# F1 Telemetry changelog

## 0.5.8 — 18 November 2026

- Show DNF for did-not-finish and retired cars, and DSQ for disqualified cars, taking priority over PIT.
- Read result status through the Live State API with a checked native binding fallback.
- Mute affected rows and wear bars, including the player row; retain tyre colours and setup controls. Whole-row opacity requires launcher support.
- In-game testing of 0.5.8-test.1 confirmed DNF and DSQ labels, muted rows, and the normal PIT marker for active cars. The 0.5.8 package promotes that tested behaviour without further runtime changes.

## 0.5.7 — 17 November 2026

- Include the PNG add-on icon and declare Steam as the tested platform for the Swofty Addon Browser.
- Package the browser icon in the same ZIP used for GitHub Releases, with validation to prevent missing browser metadata or artwork.
- Add publishing instructions for keeping GitHub and Addon Browser packages identical.
- Retain the 0.5.6 live-only behaviour and compiler/runtime settings. No UDP, recording, analysis or team-logo prototype is added.

## 0.5.6 — 17 November 2026

- License the add-on under GNU AGPL version 3 or later, retaining the applicable third-party licences and notices.
- Build the Go helper with the console subsystem while retaining Go 1.27.4, path trimming, symbols and compressed debug information. Swofty hides the helper window during normal use.
- Build the native DLL with the dynamic Microsoft C++ runtime (`/MD`), using the same VC++ v14 x64 runtime family as Swofty Launcher. Document runtime repair and check actual DLL dependency loading without an add-on-specific numeric version cutoff.
- Preserve the 0.5.5 live acquisition, all four tyre pressures, setups and lap timing. No UDP, recording or analysis is added.
- Build settings were selected using separately scanned 0.5.5 comparisons. Those scan results do not cover the newly versioned 0.5.6 EXE or ZIP.

## 0.5.5 — 17 November 2026

- Correct tyre-pressure acquisition: read each suspension wheel's configured pressure at `+0x9c`, as written by the game's setup setter, instead of the derived spring coefficient at `+0x64`.
- Add regressions for all four corners on player and other cars, wheel-to-display order, unit normalisation, invalid/missing readings and the real DLL's published shared snapshot. Preserve unavailable values as unknown.
- Retain Go 1.27.4, symbols/debug information and the existing live-only acquisition path. No UDP, recording or analysis is introduced.
- In-game testing confirmed setups display for all cars and the player's values match the game. Update the README with installation, live features and the change from 0.4.x recording/analysis to live display only.

## 0.5.4 — 16 November 2026

- Carry over the v0.4.2 Go build improvements: pin Go 1.27.4 and retain symbols and DWARF debug information by removing `-s -w`. Keep path trimming and the Windows GUI subsystem.
- Preserve the v0.5.3 live acquisition path: Swofty's Live State API with checked binding-reader fallback and a local shared-memory snapshot. No UDP receiver or fallback, recording or analysis is introduced.
- Package a new test build. In-game validation and resource comparisons remain outstanding; v0.4.2 antivirus scan results do not cover these new binaries.

## 0.5.3 — 16 November 2026

- Prefer Swofty's shared Live State snapshot for player identity, classification position, tyre compound and pit-lane status. Skip the corresponding direct readers for each valid field; retain direct acquisition for full names, wear, setups and completed lap times.
- Fall back per field when Live State is absent, unavailable or incomplete. Reject invalid values, mismatched vehicle IDs/counts and snapshots that change during sampling. Preserve the session gate, pause handling and object checks from 0.5.2. No UDP acquisition is introduced.
- Add host tests for API priority, skipped reads, partial coverage, malformed/mixed snapshots and recovery on 4/24/32-car grids. In-game verification and any resource comparison remain outstanding.

## 0.5.2 — 16 November 2026

- Show a highlighted PIT marker beside a driver's name while the game's pit-lane membership flag is set. Read it through Swofty's pit-lane binding; keep unavailable readings unknown and respect the lane array's 27-slot bound.
- Keep the live table empty at the career trackside workstation, before a session starts and during session loading/unloading. Require the actual session flow state as well as the start callback; retain values when an active session is paused.
- Read the game's localised first and last driver names and display them as SURNAME Firstname, including custom driver records where available. Keep the game's existing name when separate fields are unavailable.
- Let driver, wear and lap-time columns share the available table width automatically as the launcher window changes size. Keep compact position, tyre and setup controls and the setup card beneath its driver.
- Retain the 0.5.1 memory optimisations, binding readers and completed-sector lap-timing validation. In-game confirmation of the new session gate, full names and window sizing remains required.

## 0.5.1 — 16 November 2026

- Reduce helper allocations by reusing snapshot storage, sharing fixed column definitions and using typed native table cells.
- Keep polling at the existing rate, but request a launcher page rebuild only when its displayed inputs change. Sample timestamps, off-screen cars and closed setup cards no longer cause redundant rebuilds.
- Preserve live values, native controls, nearest/full-grid selection, setup cards, pause retention and session clearing. The game readers and lap-timing fix are unchanged. Actual launcher memory savings still require in-game measurement.

## 0.5.0 — 16 November 2026

- Correct Last/Best Lap showing an advancing counter while cars are in the garage. Accept a lap only when all three sector times are present and add up to its total; unfinished counters leave the columns unavailable or retain the previous completed lap. Invalid completed laps update Last without becoming Best.
- Correct the test build's empty live display: use Swofty's named binding system directly for session vehicles, classification, driver names, compounds and controllers, without depending on the separate live snapshot service. Include the inspected current Steam layout (`1794312636`) and explicit diagnostics for unsupported layouts, missing bindings and failed object checks.
- Make this version live display only. Remove the recorder, analysis pages, UDP port setting and None display mode. Existing saved data is left untouched.
- Replace the UDP receiver with a native add-on using Swofty's resolved game bindings for read-only live values and official lap timing. Publish a fixed 9 KiB shared-memory snapshot to the launcher helper.
- Retain full-grid/nearest-car selection, optional player highlighting, native tyre circles, four wear bars and average wear, last/best lap times, and setup cards beneath driver rows.
- Retain paused values and clear the display on explicit session end/unload or game exit. Leave unavailable fields unknown; gate direct readers and timing hooks to the inspected Steam game layout.
- Write no recordings or generated images. Document the helper process and in-game native DLL. In-game validation is still required before release.

## 0.4.2 — 16 November 2026

- Retain standard Go symbols and DWARF debug information by removing the `-s -w` linker flags. Keep Go 1.27.4, path trimming and the Windows GUI subsystem.
- Keep telemetry, recording, analysis and read-only team-name behavior unchanged.
- Record controlled build comparisons: retaining debug information clears CrowdStrike in both the empty control and the full 0.4.2 helper. Bkav still flags the helper; complete antivirus clearance is not claimed.

## 0.4.1 — 16 November 2026

- Rebuild the verified v0.4.0 UDP telemetry and analysis add-on with an updated stable Go toolchain.
- Keep telemetry, recording, analysis and read-only team-name behavior unchanged.
- Retain source, reproducible build settings and release checksums for verification.
- This maintenance rebuild does not guarantee that third-party antivirus detections will clear.

## 0.4.0 — 15 November 2026

- Replace generated analysis images with Swofty's native chart controls across practice, qualifying, sprint qualifying, races and sprints. Retain the existing results, analysis tabs and shared driver selections.
- Require Swofty 1.9.55 or newer and negotiate native chart support through the `view.chart` capability, without an upper launcher-version limit. Show a clear message if chart support is unavailable while preserving live telemetry and recording.
- Use native coloured circles beside tyre compound names. Read the actual compound for the label and the visual compound for the dry tyre colour; intermediates remain green and wets blue. Leave unknown colours uncoloured.
- Replace podium images with native medal emoji for first, second and third, retaining numeric sorting.
- Load analysis timing data separately from selected-lap traces, cancel superseded loads and avoid resending unchanged completed-session charts on every tick.
- Document the background helper, local UDP listener, read-only team-name capture, process lifetime and local storage in the README. Continue distributing one installable ZIP.

## 0.3.9 — 14 November 2026

- Apply the new qualifying views to both qualifying and sprint qualifying. Single-stage short/one-shot formats use the practice-style results table; full formats show Q3/Q2/Q1 or SQ3/SQ2/SQ1 side by side, with recorded knockout groups and missing-stage placeholders.
- Replace qualifying Laps & Sectors with Advanced Telemetry: a stage-selectable gap timeline, knockout progression across recorded stages, and teammate head-to-head bars. Use shared teammate colors, dashed second-driver progression lines and hollow eliminated endpoints. Keep missing stages and unknown outcomes explicit.
- Compare each teammate's fastest valid lap across all recorded stages, regardless of the stage, as requested. Show source stages and retain custom names/team snapshots.
- Link stages by matching profile, UDP season/weekend identifiers, track and qualifying family, with attempt/retry boundaries. Load peers in the background and retain timing/identity rather than full driving traces. Reprocess existing recordings without modifying raw journals.
- Retain practice, race/sprint analysis and Live Telemetry.

## 0.3.8 — 14 November 2026

- Update practice Overview to POS with podium colors, Full Driver Name, Team, Compounds, Laps, Fastest Lap, Gap and Avg Time. Use valid completed laps for averages and fastest-lap gaps; preserve recorded custom identities.
- Replace practice Laps & Sectors with Session Analysis. Add a full-width Team Sector Leaders graphic with the top five teams per sector, plus driver-selectable Lap Time Distribution and Consistency Analysis graphics.
- Compare team-best sectors from valid completed laps across the full session. Use valid completed laps for distribution and consistency; show mean ± standard deviation and min/max whiskers, requiring two laps for consistency. Keep team colors stable across sectors.
- Retain qualifying lap/sector comparisons and all existing race/sprint analysis. Reprocess saved recordings without rewriting them.

## 0.3.7 — 14 November 2026

- Rebuild Pace vs Field with clean-lap points, a trailing three-lap trend, whole-field median reference and shaded middle 50%, plus per-driver average deltas. Exclude pit in/out, SC/VSC and invalid laps from this view only; keep missing-lap breaks.
- Display position flow and gap-to-leader across the page. Use straight lap-to-lap position lines and an integer tick for every position. Format the lap-time distribution axis as M:SS.
- Slim and round tyre-strategy bars. Replace pit-average solid bars with sorted line-and-dot rows, actual pit-lane means and differences from the best selected driver.
- Format Overview duration as HH:MM:SS. Replace race result columns with POS, driver full name, Team, Compounds used, Gap, Grid and +/-. Color the first three positions gold/silver/bronze; retain custom names when an identity cannot be matched. Derive gaps from classification including penalties, and show lapped/retired/disqualified states.
- Retain existing recordings, shared selections, static charts, Sprint identification and Live Telemetry.

## 0.3.6 — 14 November 2026

- Fix Sprint session labels in Telemetry Analysis. Use the transmitted weekend schedule to distinguish a Sprint reported as Race (15) from the main event reported as Race 2 (16). Preserve generic labels when the schedule does not identify a distinct Sprint.
- Save weekend schedules in new metadata and recover them from a bounded journal prefix for older race recordings. Do not infer Sprint identity from lap count or rewrite original UDP journals.

## 0.3.5 — 14 November 2026

- Give Lap-by-lap Progression its own full-width chart above Sector Duel. Use dashed line segments touching pit-in/out laps while retaining the lap times and driver colors.
- Add driver summary cards below the plot: best valid captured lap and mean of all completed timed laps, including pit/SC/VSC/invalid laps. Leave missing laps out of the average; retain blank trace gaps.

## 0.3.4 — 14 November 2026

- Replace the single-lap sector duel with a full-width, whole-race timeline: one column per race lap and three sector rows, colored for the quickest selected driver. Add per-sector win totals and support shared ties.
- Remove the sector-duel lap selector from Race Progression. Keep the shared driver selection, include pit/SC/VSC/invalid laps, and leave comparisons with missing selected-driver sector data blank.

## 0.3.3 — 14 November 2026

- Remove the eight-driver row limit. Display All/Clear and driver chips in one continuous row across the page width, retaining selected highlights and shared selection.

## 0.3.2 — 14 November 2026

- Replace the expandable driver-toggle picker with always-visible driver chips. Click each name to select or deselect it; selected chips use the launcher accent highlight.
- Keep All/Clear alongside the chips and share the selection across all four race analysis tabs. Arrange names in compact rows of up to eight drivers because native button rows do not wrap. Keep Overview unfiltered.

## 0.3.1 — 14 November 2026

- Remove pit-in, pit-out, SC, VSC and invalid-lap markers from every race chart and remove their legend. Retain the driver-color legend.
- Remove the corresponding lap-timing column and sector-duel labels. Keep all laps and underlying recorded data included. Refresh generated charts through the new version cache key.

## 0.3.0 — 14 November 2026

- Rework race/sprint analysis into Overview, Lap Analysis, Pit Stops, Race Strategy and Race Progression. Keep Overview and practice/qualifying results and lap comparisons unchanged. Remove the separate race Laps & Sectors tab.
- Share any-size driver selection across the four race analysis tabs, with All drivers, Clear selection and My driver shortcuts. Start each recording's selection with the player; leave Overview unfiltered.
- Add a race-lap selector, five separate telemetry graphics on the left, and selected-driver lap/sector timing on the right. Include race pit-lane traces and retain the first-red-light start boundary for every driver.
- Add tyre-strategy timelines, average complete pit-lane duration charts and chronological stint-length stacked columns. Use actual compounds and the recorded event's tyre colors. Exclude incomplete visits and drive-throughs from pit averages.
- Add position flow and gap-to-leader charts. Recover final position/gap from classification when possible, even if the last lap's time was not transmitted. Leave other missing measurements unknown.
- Add lap-time progression, all-selected-driver sector duels, pace against the whole field's same-lap median, and lap-time distributions. Include pit, SC, VSC and invalid laps with visible markers and a legend. Use static graphics without hover interaction.
- Reprocess existing saved UDP journals without modifying them. Retain Documents storage, launcher-safe display copies, team-name snapshots and the existing Live Telemetry screen. Validate both formats and replay the saved 24-driver race, including full-grid charts.

## 0.2.14 — 14 November 2026

- Start race and sprint driving capture at the first received red start-light event, before lights out, for all 22/24 car slots in UDP 2025/2026. Buffer the latest session/participant/setup/status context before that point and exclude formation driving measurements from new journals. Practice and qualifying remain unaffected.
- Apply the same start rule when decoding existing recordings. Retain red-light/rev-up samples at session time zero instead of deleting them at lights out; order equal-time samples by frame. Preserve the negative grid-start portion of lap one.
- Record start frame, trigger and light count. Provide explicit fallbacks for missing start events or joining an underway race; rearm if a flashback returns before the start boundary. Refresh chart-cache revision.
- Check all 24 drivers in the saved race: every lap-one trace begins at first-red-light frame 3030 and reaches the timing-line end, with no formation samples. BROWNING retains 1,934 samples including 119 red-light frames before lights out. A full forward replay through the new recorder preserves every driver's trace while excluding formation measurements.
- Validate both formats, race/sprint session types 15–17, all car slots, buffered identities, red-light samples, missing events, mid-race joining, restart/flashback handling and unchanged live display.

## 0.2.13 — 14 November 2026

- Fix McLaren and other team names containing game `{o:upper}`/`{o:lower}` formatting. Apply display cleanup to existing snapshots as well as newly captured names. Preserve literal custom-name punctuation.
- Fix race lap-one traces mixing formation and race samples when session time resets to zero at lights out. Discard the earlier untimed phase, retain negative grid-start lap distances, and refresh chart cache keys so existing recordings redraw correctly. Protect already timed race laps and qualifying attempts.
- Close a final stint reported with end lap 255 using the driver's final classification completed laps. Show an explicit ended/unknown state when no authoritative endpoint exists. Keep live stints in progress.
- Replay the saved 20-lap race read-only: BROWNING lap one now has 1,815 samples from the grid to 5,274 m; McLaren displays correctly and stint two ends on lap 20. Validate both UDP formats and retain display-cache and team-name capture fixes.

## 0.2.12 — 14 November 2026

- Fix the blank session overview strip after moving storage to Documents. Swofty only allows image paths below its addon data/cache roots; publish byte-for-byte disposable display copies into the launcher-provided cache while retaining original files and all session data in Documents\F1 Game Telemetry.
- Apply the same fix to analysis trace charts and live tyre-compound images. Use content-based cache paths to avoid stale pixels, recreate cleared/corrupt copies, and fall back to native overview values or tyre text if image publishing fails.
- Add compiled-helper checks for the actual launcher image-root restriction with separate Documents, addon data and cache directories. Retain automatic team-name capture and saved team names.

## 0.2.11 — 14 November 2026

- Automatically capture all loaded game team records during new UDP sessions, including custom names and the active My Team name, using external read-only process access on the verified Steam build.
- Resolve team-name language keys using the game baseline, active Swofty language mounts and applicable runtime text overrides; retain raw identities and process/build provenance with the session.
- Add Team to Overview/Qualifying Results. Match both UDP formats using saved team identities and native entrant/race-number data; leave unavailable or ambiguous matches unknown.
- Keep snapshots with their own recording across restarts and later career loads. Do not apply current names to old recordings that lack a snapshot. Preserve Documents\F1 Game Telemetry storage and the Live Telemetry layout.
- Include automated memory-layout, read-only access, custom-name, language override, both-format results/restart and session/profile isolation checks. The next race remains the live capture validation.

## 0.2.10 — 14 November 2026

- Store recordings, session metadata, addon preferences and generated images in Documents\F1 Game Telemetry, respecting OneDrive and redirected Documents folders.
- Automatically copy existing addon data once on first launch after updating, preserving existing destination files and retaining the originals as a backup. Copy each file atomically; prevent deleted recordings from being reimported on restart.
- Continue reading launcher profile controls from Swofty independently of the new storage location. Keep Live Telemetry and analysis layouts unchanged.

## 0.2.9 — 14 November 2026

- Move the results/statistics view to the first tab: Overview for practice and races, Qualifying Results for qualifying and sprint qualifying. Open it by default and after changing session; avoid a duplicate Session Stats tab. Keep Laps & Sectors second in practice/quali, with the other analysis tabs retained for races.
- Add an original compact session summary strip: six accent cards for Country, Circuit, Laps, Air temp, Track temp and Weather. Render it locally at high DPI using Windows UI fonts and custom icons, with native-stat fallback.
- Decode latest recorded weather, signed air/track temperatures and configured race laps from Session packets. Show Timed session for practice/quali; preserve unknown measurements and genuine 0°C. Restore earlier condition snapshots on flashback.
- Resolve country/circuit from track ID, including correct reverse-track IDs 39/40/41 and Madrid 42. Use reported weather labels rather than inferred track wetness; do not invent wind data.
- Reprocess existing journals without changing recording format. Keep the compact selection/save editor and Live Telemetry unchanged.
- Validate both formats, contextual tab labels/order/defaults, session changes, weather/temperature decoding, missing values, flashback recovery and bitmap text rendering. Visually inspect a strip generated from the original recorded qualifying session. Go checks, both executable integration suites and Swofty package validation pass; native placement after import still needs user confirmation.

## 0.2.8 — 14 November 2026

- Compact the Telemetry Analysis header. Hide the redundant Season selector for a save with only one recorded season, allowing Save, Race Weekend and Session to fit in the native three-column form. Keep Season available for multiple seasons.
- Replace the always-open save-name form and explanation with an Edit save name action. Open the compact editor on demand and close it after saving, cancelling or changing session filters.
- Put Refresh and Edit save name in one action row. Remove the separate Recorded/Status/Packets facts table; session date/status remain in the selector and full recording metadata stays saved locally.
- Preserve analysis tabs, lap comparisons, recording and Live Telemetry. Header/season/editor regressions, Go vet, Windows build, both-format compiled-helper analysis tests and Swofty package validation pass.

## 0.2.7 — 14 November 2026

- Show only Laps & Sectors and Session Stats in every practice, qualifying and sprint qualifying session. Default to Laps & Sectors, including when changing from a now-hidden tab.
- Preserve the existing sprint/race tabs, valid-lap comparisons, position-sorted results and background recording. Live Telemetry is unchanged.
- Verify the exact two-tab set and fallback for all session types 1–14, with race tab retention checked for session types 15–17. Focused analysis regressions, Go vet, Windows build and Swofty package validation pass.

## 0.2.6 — 14 November 2026

- Fix sparse comparisons appearing almost empty: continuous packet frames now connect recorded samples even when simulation time advances more than 0.3 seconds per sample. Estimate normal frame spacing per trace and retain breaks for large missing-frame gaps or backwards time/distance. Keep the conservative time-gap rule when frame continuity is unavailable.
- Show a lower-resolution notice with the sample count. Explicitly identify sparse laps whose throttle/brake fields were reported as zero throughout; do not infer or fabricate missing pedal activity.
- Draw gear changes as discrete steps, including sparse traces, rather than implying fractional gears between samples.
- Refresh the chart-cache revision so existing journals redraw with the corrected continuity rule. Preserve session tabs, valid-lap comparison, recording and Live Telemetry.
- Verify BROWNING lap 5 versus GOETHE lap 2 against the user's saved qualifying journal. GOETHE has 57 samples with consecutive frame numbers across approximately 80 seconds of game time; its speed/gear/RPM comparison is now visible. Add pixel-level sparse-curve, missing-frame-gap, normal-cadence and zero-pedal warning tests; both-format integration suites pass.

## 0.2.5 — 14 November 2026

- Hide Tyres & Pits and Positions for all practice, qualifying and sprint qualifying session types. Keep those tabs for sprint/race sessions and return to Traces if changing session leaves a hidden tab selected.
- Omit Grid from Session Stats in practice, qualifying and sprint qualifying. Sort session results numerically by position in every mode, with unknown positions last.
- Show only valid completed lap rows in Laps & Sectors. Exclude invalid, blank and unfinished laps from that table and its lap selectors.
- Add optional lap/sector comparison: select a driver and valid lap on each side to see lap time, Sector 1, Sector 2 and Sector 3 with signed deltas. Negative deltas indicate the selected lap is faster; missing sector data remains unknown. History-only laps can be compared without trace samples.
- Retain No comparison and respect changed-field events in both comparison views. Keep Live Telemetry, recording and native launcher settings unchanged.
- Validate visibility/Grid behavior across all 17 practice/qualifying/race session IDs, numerical position ordering, valid-only choices, history-only comparisons, missing sectors and minute-boundary deltas. Both-format executable UDP and Live Telemetry regressions pass.

## 0.2.4 — 14 November 2026

- Add an explicit default **Compare driver → No comparison** option. Hide the comparison-lap field and draw no reference samples while disabled; selecting a driver enables lap comparison. Choosing No comparison in either selector turns it off.
- Respect the launcher's changed-field list so stale values from other fields cannot restore a disabled comparison.
- Show the full running version in both page subtitles and the addon summary, making an old helper distinguishable after import. Use a fresh chart-cache revision.
- Include all earlier fixes for qualifying lap-number reuse and late telemetry capture. Verify a single full-lap chart against the original BROWNING lap 6 recording, without garage/out-lap overlays.
- Preserve completed timing updates when history contains a zero-time placeholder for the current partial lap. Keep protection for already completed authoritative history times.
- Add image-level tests confirming no orange reference pixels in single-driver mode, comparison enable/disable tests, and executable protocol tests for the explicit No comparison controls.

## 0.2.0-preview.3 — 14 November 2026

- Fix the recorder discarding detailed car/lap telemetry after final classification. Continue retaining all same-session UDP after classification and `SEND`, including closing events with UID 0, while keeping the recording marked completed and Live Telemetry cleared.
- Close capture on explicit menu/new-session/game-exit evidence or helper shutdown. Keep retired-session packets out of subsequent recordings and retain explicit restart handling.
- Preserve final classification, best time and stints against later live lap/history updates; preserve authoritative history lap/sector times against later lap packets.
- Store classification/end/last-car-telemetry timestamps and counts of car-telemetry packets received after classification and session end, to diagnose the game's actual end-of-session output in future runs.
- Keep progressive flushes and valid completed snapshot footers without forcing a disk sync for every late packet. Add both-format unit and executable UDP tests proving late traces survive reload, final results stay authoritative and Live Telemetry does not revive.
- Previously discarded packets cannot be reconstructed. This change does not establish whether the game continues transmitting other drivers' unfinished laps.

## 0.2.0-preview.2 — 14 November 2026

- Fix overlapping qualifying traces when the game reuses a lap number through an aborted flying lap, garage stay, out-lap and subsequent timed run. Compare the latest timed attempt, with a break on distance or lap-timer reset.
- Exclude garage/in/out-lap samples from timed comparisons while retaining the original raw UDP recording and normal on-track race samples.
- Label partial traces when the timing-line start or recorded lap completion is missing. Do not invent the end of a simulated/unfinished AI lap.
- Reprocess existing recordings and use a new chart-cache revision so the previous mixed-run image cannot remain on screen after upgrade.
- Seal completed gzip journal members at session end; accept late history/classification in additional members. A normal completed snapshot no longer reports a missing journal footer merely because the helper remains open.
- Verify the correction against the user's recorded qualifying session and add regression tests for lap-number reuse, timer/distance resets, partial coverage and late-history journal footers. Preserve the Live Telemetry layout and all saved preferences/recordings.

## 0.2.0-preview.1 — 14 November 2026

- Add the second sidebar page **Telemetry Analysis**, with Save → Season → Race weekend → Session selection. Keep the Live Telemetry layout, settings and inline setup inspector unchanged.
- Automatically record original 2025/2026 UDP datagrams to local compressed journals, with per-attempt metadata, save naming/association, periodic flushes and recovery of interrupted files. Recordings remain available after session and launcher exit.
- Add distance-based stacked speed, throttle, brake, gear and RPM charts, driver/lap comparison and quarter-lap range selection.
- Add lap/sector tables, actual compound stints, observed stationary/pit-lane timings, lap-start positions, fastest lap and final classification.
- Show race-only captured overtake totals and distinct SC/VSC affected laps. Exclude formation safety cars; avoid race-only statistics in practice and qualifying.
- Pair telemetry and lap-distance packets by frame, tolerate arrival reordering, preserve gaps, separate explicit restarts and remove superseded data after flashbacks.
- Preserve native launcher preferences and the legacy saved/custom UDP port.
- Preview limitations: native image charts rather than web hover/drag interaction; save names require local association; recording completeness depends on received UDP. Live-game/native-renderer and career-identity checks remain outstanding. No change to the native catalog-backed changelog limitation described below.

## 0.1.4 — 14 November 2026

- Fix stale telemetry after leaving a session while keeping the game open.
- Handle session-end events independently of ordinary event frame ordering, including events sharing a frozen frame and closing packets with reset frame counters or session UID 0.
- Ignore unrelated event packets instead of allowing their frame numbers to suppress lifecycle events.
- Accept explicit menu-state Session packets as additional end evidence after an established session, without treating unknown initial grid metadata as a session end.
- Continue retaining samples through pauses and temporary UDP silence. No automatic inactivity timeout was added.
- Prevent late ended-session data from restoring cleared rows, and allow explicit same-UID restarts.

## 0.1.3 — 14 November 2026

- Move Drivers shown, Highlight my row and UDP port into the native Swofty Addons → UDP Telemetry → Settings panel. Display changes are picked up when the telemetry page opens or refreshes.
- Remove the settings form and wear/setup explanation from the telemetry page.
- Keep the compact setup inspector directly underneath its driver, with the separate Setup control. Native click-to-open details have launcher-controlled placement, so inline placement takes priority.
- Remove the legacy engine-braking and ballast values from the setup inspector. Show only player-facing car setup controls: wings, differentials, camber/toe, suspension, anti-roll bars, ride heights, brake pressure/bias, tyre pressures and fuel load.
- Preserve the existing UDP port through the native Use saved port option. The launcher port selector also offers common port presets, including 20800.

## 0.1.2 — 14 November 2026

- Receive pre-grid packets even before Swofty's game-running status catches up. Unknown startup session metadata no longer ends the session.
- Preserve grid samples through the first session-start event. Use grid positions when current race positions are not yet populated; retain a known player slot through temporary unknown indices.
- Narrow the position and tyre columns and reserve most flexible width for driver names.
- Remove the YOU suffix. Add a saved **Highlight my row** toggle, off by default.
- Show filling wear bars and percentages for FL, FR, RL, RR and the four-tyre average. Bars change from green to amber at 50% and red at 75%; missing/restricted wear has no bar.
- Replace the side setup panel with a Setup switch that opens one compact, full-width inspector directly below the selected driver. All setup fields remain available on the existing page.
- Reduce the header height. No new sidebar pages or view selectors.

Grid visibility remains limited by the game's UDP output; this release does not manufacture samples before they are transmitted.

## 0.1.1 — 14 November 2026

- Fix the launcher rejecting the telemetry page with `missing field tone`.
- Supply an explicit neutral tone on every text block, including connection guidance, error messages and driver setup details.
- Add a regression check for text-block tones throughout the returned page, including nested setup panels.

## 0.1.0 — 14 November 2026

Initial preview release for Swofty Launcher 1.9.42 on Windows.

- Live telemetry page within Swofty, receiving only the game's UDP output.
- Automatic decoding of 2025 (22 slots) and 2026 (24 slots), including My Team participants.
- Nearest cars: the player plus four drivers by race position, normally two ahead and two behind; fill from the other side at the front/back of the grid.
- Full grid and None display modes.
- Driver names, race positions and actual tyre compounds in translucent coloured pills. Slick colours follow the fitted Soft/Medium/Hard designation; intermediates are green and wets blue.
- FL, FR, RL and RR tyre wear, plus their arithmetic average.
- Last lap and best valid lap in the current session, using the game's lap history.
- Click a driver to inspect their complete transmitted setup: wings, differentials, engine braking, geometry, suspension, anti-roll bars, ride heights, brakes, tyre pressures, ballast and setup fuel load.
- Keep the latest values visible while paused, in the garage, on the grid or while packets stop arriving temporarily.
- Clear the session on UDP session-end events or a confirmed game stop reported by Swofty.
- Reset on new sessions and restarts; discard stale packets and clear future best laps after flashbacks.
- Configurable UDP port, saved display preferences, restricted-wear handling and an explicit port-conflict message.

No planned pit strategies or telemetry recording. Fields the game does not supply display as unavailable.

This file is the release text to enter into Swofty's Addon Browser publishing form. Swofty 1.9.42's built-in addon changelog panel uses published catalog metadata; installing a local package does not populate that panel from this file. There is no extra changelog sidebar page and no release notes on the telemetry page.
