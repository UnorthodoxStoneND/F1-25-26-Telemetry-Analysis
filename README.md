# F1 25/26 Telemetry & Analysis [Swofty Add-On]

## Description

Live telemetry and post-session analysis for F1 25 and the 2026 Season Pack, including My Team. View your session in Swofty as you drive, then review the recorded results, laps and strategies afterwards.

## Features

- **Live telemetry:** driver positions, tyre compounds, wear on each tyre and average wear, last and best lap times, and expandable car setup details.
- **Driver display options:** show your driver plus the nearest four cars, the full grid or no rows, with optional highlighting for your own driver.
- **Practice analysis:** session results, team sector leaders, lap-time distribution and consistency analysis.
- **Qualifying and sprint qualifying:** results, qualifying timeline, knockout progression and teammate comparisons, including Q1/Q2/Q3 and SQ1/SQ2/SQ3 when recorded.
- **Race and sprint analysis:** compare drivers' speed, throttle, brake, gear and RPM; review lap and sector times, pit stops, tyre strategies, position changes, gaps to the leader and race pace.
- **Native display:** analysis charts render directly in Swofty, without generating chart images. Tyres use native coloured circles beside compound names, and the top three result positions use medal emoji.
- **Saved sessions:** recordings are organised by save, race weekend and session, and stored locally in **Documents/F1 Game Telemetry**.

## Install guide

1. Download **`UDP_Telemetry_v0.4.1.zip`** from [GitHub Releases](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/releases/latest). Choose the named ZIP, rather than GitHub's automatic **Source code** download. The add-on is not currently listed in Swofty's Addon Browser.
2. In Swofty, open **Addons â†’ Install addon from computer** and select the downloaded file.
3. Enable **UDP Telemetry** in your active profile, then restart Swofty.
4. In the game, open **Telemetry Settings** and use these settings:

| Setting | Value |
| --- | --- |
| UDP Telemetry | On |
| UDP Broadcast Mode | Off |
| UDP IP Address | `127.0.0.1` |
| UDP Port | `20777`, or the port set in the add-on's settings |
| UDP Format | `2025` or `2026`, matching your game |
| UDP Send Rate | `20 Hz` |

Keep Swofty open with the add-on enabled while you play. Use **UDP Telemetry** for the live display and **Telemetry Analysis** to review saved sessions. The UDP port must match in the game and the add-on settings.

### Processes and local files

Swofty starts the bundled **`udp-telemetry.exe`** helper in the background when it loads the add-on. It has no separate window and communicates with the launcher through local input/output pipes. No separate chart renderer, web server, Python process or installed Windows service is needed.

While enabled in the active profile, the helper listens for game UDP packets on **`127.0.0.1`**, using the configured port (default **20777**). It records sessions and supplies live values and analysis data to Swofty. Charts are drawn by Swofty's native chart controls; changing a selection does not generate image files. Analysis loads timing data first and driving traces for the selected lap, rather than retaining every lap's full traces.

The same helper also captures actual team names, including custom My Team names. This uses **read-only access to the running `F1_25.exe` process** and the game's local language files; team names are not supplied entirely by UDP. It does not inject code or modify game memory or files. This reader is checked against a verified game build; if capture is unavailable, telemetry recording continues and unavailable names remain unknown.

The helper exits when Swofty closes or shuts down the add-on, including when its launcher communication pipe closes. Disabling the add-on in a profile closes the UDP listener and finalises the current recording; Swofty may keep the helper process loaded. Ending a game session finalises that recording but leaves the helper available for the next session. Closing the game alone does not close the helper while Swofty remains open.

Recordings, session metadata, captured team-name snapshots, save associations and settings are saved locally in **Documents/F1 Game Telemetry**, respecting Windows' redirected Documents folder. Existing recordings remain readable. Older versions' image files may remain on disk, but v0.4.1 does not use or regenerate them. The helper does not upload recordings or team names.

## Antivirus notice

Some engines flagged the previous v0.4.0 helper. We reproduced that executable from source and investigated its sandbox warnings, which included normal Go metadata, Windows system DLL loads and certificate-check traffic. Version 0.4.1 rebuilds the same features with Go 1.27.4. A compiler update does not guarantee that antivirus detections will clear; the previous VirusTotal report does not cover this new executable. Keep antivirus protection enabled.

## Supported platforms

- **Operating system:** Windows x64.
- **Launcher:** Swofty Launcher 1.9.55 or newer with the `view.chart` capability. Future versions are supported while they retain the compatible add-on API and chart capability.
- **Game:** F1 25 on Steam, including the 2026 Season Pack and My Team.

## Build from source

Use Go **1.27.4** for this maintenance release. From the `source` directory, build with:

```powershell
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:GOAMD64 = "v1"
go build -buildvcs=false -trimpath -ldflags "-s -w -H=windowsgui" -o ../bin/udp-telemetry.exe .
```

Run `python source/package.py` from the add-on root to create the installable ZIP. The build has no third-party Go module dependencies.
