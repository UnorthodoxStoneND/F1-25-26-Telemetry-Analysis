# F1 25/26 Telemetry & Analysis [Swofty Add-On]

## Description

Live telemetry and post-session analysis for F1 25 and the 2026 Season Pack, including My Team. View your session in Swofty as you drive, then review the recorded results, laps and strategies afterwards.

## Features

- **Live telemetry:** driver positions, tyre compounds, wear on each tyre and average wear, last and best lap times, and expandable car setup details.
- **Driver display options:** show your driver plus the nearest four cars, the full grid or no rows, with optional highlighting for your own driver.
- **Practice analysis:** session results, team sector leaders, lap-time distribution and consistency analysis.
- **Qualifying and sprint qualifying:** results, qualifying timeline, knockout progression and teammate comparisons, including Q1/Q2/Q3 and SQ1/SQ2/SQ3 when recorded.
- **Race and sprint analysis:** compare drivers' speed, throttle, brake, gear and RPM; review lap and sector times, pit stops, tyre strategies, position changes, gaps to the leader and race pace.
- **Saved sessions:** recordings are organised by save, race weekend and session, and stored locally in **Documents/F1 Game Telemetry**.

## Install guide

1. Download **`UDP_Telemetry_v0.3.9.zip`** from [GitHub Releases](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/releases/latest). Choose the named ZIP, rather than GitHub's automatic **Source code** download. The add-on is not currently listed in Swofty's Addon Browser.
2. In Swofty, open **Addons → Install addon from computer** and select the downloaded file.
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

## Supported platforms

- **Operating system:** Windows x64.
- **Launcher:** Swofty Launcher 1.9.42 or newer.
- **Game:** F1 25 on Steam, including the 2026 Season Pack and My Team.
