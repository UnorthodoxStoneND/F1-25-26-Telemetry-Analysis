# F1 25/26 Live Telemetry [Swofty Add-On]

Live telemetry and car setups for F1 25 and the 2026 Season Pack, including My Team. Version **0.6.0** adds an optional standalone retro window to the existing launcher page. Live-only: no UDP configuration, recording or post-session analysis.

## Features

- The retro header shows the session type with its countdown or lap counter above the field. Counter visibility follows the saved setting for that session group.

- Resize retro columns by dragging a heading’s right edge; double-click the divider for automatic width. Widths are saved per profile and session group, including hidden columns. Reset defaults resets the current group’s visibility and widths.

- First-time retro placement follows the launcher's monitor. Later openings restore the saved size, position and maximized state; add-on updates retain these preferences. Missing monitors trigger a reachable placement on a current screen. This uses Windows/X11 window APIs; native Wayland compositor restrictions apply.

- The retro window reuses its rows, controls and images between updates, refreshing only changed values and preserving scroll position when cars change places.
- Retro driver rows slide to their new positions over 250 ms while values remain live. Further position changes retarget the movement; the in-launcher table keeps its existing behaviour.

- Choose **Launcher page** or **Retro window** in the add-on's **Telemetry display** setting. Only the selected display shows telemetry. The inactive launcher tab remains visible because Swofty does not expose conditional tabs.
- The retro window uses a black canvas, monospaced timing text, yellow headings/driver names, green wear/status values and optional blue player-row highlighting. It opens on a fresh active session and hides on session exit or return to the hub. Pausing keeps it open. Manually closing it suppresses reopening until the next session or a display-mode switch.
- The window runs inside the existing helper process; no browser, web server or additional companion process. Columns, session switches, team/tyre images and timed-session setups use the same data and preferences as the launcher view.

- Current session and counters in the existing page subtitle, including practice/qualifying formats and weekend-aware Sprint/Race classification.
- Live team logos between position and driver, loaded in memory through Swofty’s Live State image service.
- Team and tyre images support F2 and other launcher-provided vehicle/tyre definitions; no F1 compound-number whitelist is applied. Image availability follows Swofty's service and matching driver identity.
- Live driver positions, launcher-provided names, fitted-tyre icons from Swofty’s image service and pit-lane indicators.
- Practice: completed laps, last/best lap, status and expandable setups.
- Driver rows follow Live State’s classification order when valid, with the existing position/grid/vehicle fallback when unavailable. Full-grid and nearby-driver modes use the same order.
- **Sliding row animation (retro window)** in add-on settings is on by default. Switch it off for immediate position changes; the choice is saved with the Swofty profile and applies live.
- PEN in qualifying and races shows the current time-penalty total plus pending drive-through/stop-go counts from Live State (`5s`, `DT`, `SG`, `5s+DT`). Grid penalties are not yet exposed by this column.
- Qualifying/Sprint Qualifying: the practice columns plus gap to the current session best and S1/S2/S3 from each driver's best lap. Q1/Q2 and SQ1/SQ2 highlight the calculated elimination zone in red; Q3/SQ3 have no elimination highlight. Cutoffs are calculated pending exposure of the game's actual rule (see live-data reference).
- Sprint/Race: positions gained/lost, fitted tyre age (laps), gap ahead, last/best lap, sectors from the last completed lap, wheel/average wear percentages and status. Tyre age comes from Live State v6 and includes earlier stints on the fitted set within the current session. Qualifying-to-race usage is not carried over by the inspected launcher implementation, so a used set can show zero at race start.
- Practice and qualifying status shows OUT on observed outlaps, returning to RUN on lap completion. PIT, DNF and DSQ retain priority; Sprint/Race status is unchanged.
- Per-profile session-view switches and a header Columns selector with saved field visibility for each session group.
- Full-grid or nearby-driver views, optional player highlighting and a resizable table.
- Preserve the helper build settings from main: uncompressed debug information and Linux PIE/ASLR, alongside the retro desktop renderer.
- Data from Swofty's Live State API and checked native game bindings, with bounded shared memory and no telemetry recordings or uploads.

See the [live-data reference](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/docs/reference/live-data.md) for display behaviour, supported readings and process details.

## Install

Download **`F1_Telemetry_v0.6.0.zip`** from [GitHub Releases](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/releases/tag/v0.6.0), or install the same version through Swofty once its browser submission is approved. In Swofty, open **Addons** and select **Install addon from computer**. Enable **F1 Live Telemetry** in the active profile and restart Swofty and the game after updating. The default display is **Launcher page**; choose **Retro window** under **Telemetry display** to use the standalone window. Only the selected mode displays telemetry.

The native DLL uses the same Microsoft Visual C++ v14 x64 runtime family as Swofty Launcher. If a runtime dependency is missing, install or repair [Microsoft's official x64 runtime](https://aka.ms/vc14/vc_redist.x64.exe).

Follow the [installation guide](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/docs/guide/installation.md) for the complete steps. This ZIP also contains the icon and metadata required for submission to Swofty's Addon Browser. Browser availability depends on approval.

## Supported platforms

- Windows x64.
- Experimental native Linux x64 helper for F1 25 under Proton/Wine. Helper/transport tests pass in Ubuntu WSL; in-game Linux validation is still required.
- Desktop-enabled Linux helpers require system X11/OpenGL/libXrandr libraries and a working desktop graphics driver (the Linux binary is built on Ubuntu 24.04). Go and Fyne do not need installing. Windows uses the system OpenGL driver. This is a separate desktop window; exclusive-fullscreen overlay behaviour is not provided.
- Swofty 1.10.9 and compatible later versions retaining the required APIs.
- F1 25 on Steam, including the 2026 Season Pack and My Team, on the supported game layouts.

See [compatibility and testing](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/docs/reference/compatibility.md) for the exact requirements and verification limits. For recording and post-session analysis, keep version 0.4.2; this version leaves existing recordings untouched.

## Contributing

Use the pinned Go **1.27.4** helper toolchain (matching CI), Python and the Visual Studio C++ x64 build tools. From the repository root:

```powershell
go -C source vet ./...
go -C source test -trimpath -count=1 ./...
python native/build.py
```

See the [development guide](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/docs/reference/development.md) for build and packaging commands, the repository layout and local checks. The [CI check reference](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/tools/checks/README.md) lists each required validation. Report whether changes were tested in game when opening a PR. Use the [publishing guide](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/docs/reference/publishing.md) to prepare one complete ZIP for both GitHub Releases and the Swofty Addon Browser.

## Licence

Copyright (C) 2026 UnorthodoxStoneNDmodding. Licensed under the [MIT License](LICENSE) (`MIT`).

Third-party components retain their own licences: Go runtime and standard library (BSD-3-Clause), Swofty SDK (MIT), selected generated game type declarations (PolyForm Noncommercial 1.0.0), and Microsoft C++ Standard Library (Apache-2.0 WITH LLVM-exception). See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) and [native/SDK-LICENSE.txt](native/SDK-LICENSE.txt) for notices and Microsoft runtime details.

The desktop helper also includes Fyne and its dependencies. Their redistribution notices, including embedded fonts and C graphics code, are included in [DESKTOP_LICENSES.txt](DESKTOP_LICENSES.txt).

The installable ZIP contains the manifest, add-on icon, three runtime binaries, user/release documentation and required licence notices. Full source, SDK headers, build scripts and tests are available in the public GitHub repository and its release source archive; they are not installed with the add-on. Earlier release archives remain unchanged.

## Credits

- [Swofty Launcher](https://github.com/Swofty-Developments/SwoftyLauncher) for the launcher, Live State API and native add-on SDK.
