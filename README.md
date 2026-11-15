# F1 25/26 Telemetry & Analysis [Swofty Add-On]

Live telemetry and saved-session analysis for **F1 25 and the 2026 Season Pack**, including My Team, inside **Swofty Launcher**.

**Current version: v0.3.9. Windows x64. Swofty 1.9.42 or newer.** This version uses the launcher's existing image controls for analysis charts and does not require an experimental native-chart launcher.

## Download and install

1. Open [Releases](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/releases) and download **`UDP_Telemetry_v0.3.9.swoftyaddon`**. GitHub's automatic Source code ZIP is for development; it is not the installable addon.
2. In Swofty, open **Addons -> Install addon -> Choose a file** and select the downloaded file. An equivalent ZIP is available as an alternative.
3. Enable the addon in your active profile and fully restart Swofty. The existing sections are named **UDP Telemetry** and **Telemetry Analysis**. The analysis subtitle should show **v0.3.9**.
4. In the game's telemetry settings, enable UDP, disable broadcast, set the IP to **127.0.0.1**, and use the same port as the addon (default **20777**). Select UDP format **2025** or **2026** and a **20 Hz** send rate.

No Go, Python or additional runtime is needed to use the Windows download. The public addon identity is `unorthodoxstonendmodding.udp-telemetry`. Swofty treats this as a separate addon from earlier private builds; disable the old addon before enabling this one, and re-enter its launcher settings. Existing recordings in Documents/F1 Game Telemetry remain in place. The launcher section names remain unchanged.

## Live Telemetry

- Nearest cars (you plus four), full grid or no driver rows.
- Position, driver, actual tyre compound, wear for all four tyres and average wear.
- Last lap and best valid lap times.
- Expandable player-facing car setup information.
- Optional player-row highlighting and UDP port preferences in the launcher's addon settings.

Data remains visible during pauses and clears when an end-of-session/game transition is confirmed. Availability depends on the packets the game actually sends; missing values are not invented.

## Telemetry Analysis

Sessions are saved locally and grouped by **save, race weekend and session**. Name a save once through **Edit save name**; UDP does not expose its friendly career-save name.

| Session | Analysis |
| --- | --- |
| Practice | Overview results; team sector leaders; lap-time distribution; consistency analysis |
| Qualifying and sprint qualifying | Results; single-stage or linked Q1/Q2/Q3 and SQ1/SQ2/SQ3 views; stage comparison; knockout progression; teammate comparisons |
| Race and sprint | Overview; Lap Analysis; Pit Stops; Race Strategy; Race Progression |

Race analysis includes multi-driver speed/throttle/brake/gear/RPM traces, lap/sector timing, tyre strategies, total pit-lane duration comparisons, stint lengths, position flow, gaps to the leader, lap progression, sector duels, pace versus the field and lap-time distributions. Driver selection carries across race analysis tabs; Overview shows the complete results.

Charts in this version are static images regenerated as selections change. Hover tooltips, interactive zoom and chart-click actions are not included.

## Storage and data access

Recordings, settings and captured team identities are stored under **`Documents/F1 Game Telemetry`**, using Windows' actual Documents folder (including redirected/OneDrive Documents). Launcher-safe display copies are kept in Swofty's addon cache.

The addon does not upload telemetry. Recording and chart-cache disk usage can grow over time; there is no automatic retention limit in this version. Back up each recording's JSON metadata and `.udp.gz` journal together, and close Swofty before removing old recordings manually.

Driving measurements come from UDP. Team-name metadata can additionally be read from the running game using **external read-only process access** on the verified Steam build. This is build-dependent and may be unavailable after game updates. No code is injected into the game; this addon does not modify game files, game memory, car handling or career-save contents.

## Known limits

- This targets **F1 25 plus the 2026 Season Pack**, not a separately verified standalone F1 26 executable.
- The game may omit or restrict other cars' setup, tyre or driving inputs. Some AI channels can be sparse or zero. The addon cannot recover packets it never received.
- Practice, qualifying, sprint qualifying, sprint and race use different views. Missing qualifying stages and uncertain outcomes stay explicit.
- Race driving capture starts at the first received red start-light event when available, excluding formation data. Joining late or missing that event limits the available start/lap-one data.
- Charts are image-based in v0.3.9 and can use more disk/cache space with larger selections.

See [the detailed usage guide](docs/USAGE.md), [changelog](CHANGELOG.md) and [validation notes](VALIDATION.md).

## Build from source

Requires Windows x64 and Go **1.24 or newer**. Python is needed only for integration tests and packaging; the Go project has no third-party dependencies.

```powershell
$env:CGO_ENABLED="0"
cd source
go build -buildvcs=false -trimpath -ldflags '-s -w -H=windowsgui' -o ../bin/udp-telemetry.exe .
go test ./...
go vet ./...
cd ..
python source/integration_test.py
python source/analysis_integration_test.py
python source/package.py
```

Packages and SHA-256 files are written to `dist/`. Binaries, local settings, recordings and build caches are excluded from source control. The included Windows CI builds, tests and packages the addon; it does not publish a release automatically.

## Support and licence

Report problems through [Issues](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/issues), including the addon/launcher version, game/UDP format, session type and what happened.

Licensed under the [MIT licence](LICENSE). Go runtime/standard-library notices are in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). Original addon implementation by UnorthodoxStoneNDmodding with Codex; maintained here by UnorthodoxStoneNDmodding. This is a community project and is not affiliated with EA, Formula 1 or Swofty. Publishing on GitHub does not automatically list the addon in Swofty's Addon Browser.
