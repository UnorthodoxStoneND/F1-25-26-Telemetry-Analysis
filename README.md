# F1 25/26 Telemetry & Analysis [Swofty Add-On]

## Description

**v0.5.0 is live display only.** View current driver positions, tyres, wear, lap times and car setups inside Swofty. This version replaces the UDP receiver with read-only game-engine readers using Swofty's named binding system and session callbacks. It contains no session recorder or Telemetry Analysis page.

## Features

- Driver positions and names, with either the full grid or your car and the nearest four cars. Nearest shows two ahead and two behind, shifting towards the available cars at either end of the grid.
- Actual tyre compound labels with native coloured circles. Dry tyre colours use the current soft/medium/hard designation; intermediates are green and wets blue.
- Individual FL, FR, RL and RR wear bars and percentages, plus the average when all four readings are available.
- Last completed lap and best valid lap for each driver. Unavailable values appear as a dash.
- A setup control beneath each driver's row showing the player-facing setup values that can be read. Fuel is labelled **Fuel remaining**, because the live tank reading is not the original setup fuel load.
- Optional highlighting of your own row. Display settings are in Swofty's add-on settings.
- Pausing retains current values. Session end/unload clears them; closing the game releases the shared snapshot.

## Install guide

1. Download **`F1_Telemetry_v0.5.0.zip`** from the release containing this version. Use the named ZIP, rather than GitHub's automatic Source code download. The add-on is not listed in Swofty's Addon Browser.
2. Close the game. In Swofty, open **Addons → Install addon from computer** and select the ZIP.
3. Enable **F1 Live Telemetry** in your active profile. Configure **Drivers shown** and **Highlight my row** in its settings.
4. Restart Swofty after replacing an earlier version, then launch the game through Swofty so its native component is loaded. Open **Live Telemetry** in the launcher.

No game UDP settings or port configuration are needed. Grid and garage values appear as their underlying game systems become available; a lap time cannot appear before a completed lap exists.

### Processes and local files

Swofty starts **`telemetry-live.exe`**, a small background helper with no separate window. It communicates with Swofty through local input/output pipes. Swofty loads **`telemetry-live.dll`** inside the game using its normal native add-on system. The DLL resolves named game bindings through Swofty, reads the current objects and observes two existing lap-timing functions without changing their arguments or results. It does not depend on the separate `SwoftyCoreLive` snapshot service and does not change car settings or game files.

The DLL publishes a fixed **9 KiB shared-memory snapshot** in Windows. The helper opens it read-only and displays the latest values. There is no network listener, web server, separate recording process, Python runtime or installed Windows service. The add-on retains current values and a small timing cache, rather than recording a session history.

The helper exits when Swofty closes its communication pipe. Disabling the add-on clears its display and closes the helper's snapshot handles; Swofty may keep the helper process running. A native component already loaded in a running game remains there until that game closes. Restart the game after enabling, disabling or updating a native add-on.

v0.5.0 writes no telemetry files, images or settings of its own. Swofty manages its profile settings and normal launcher logs. Existing data in **Documents/F1 Game Telemetry** is left untouched, but this live-only version does not open or analyse those recordings. Nothing is uploaded by the add-on.

## Supported platforms

- **Operating system:** Windows x64.
- **Launcher:** Swofty 1.9.55 and compatible later versions retaining launcher API 1, native binding API 9, session callbacks and memory-check API 1. No maximum launcher version is imposed; future breaking API changes may require an update.
- **Game:** F1 25 on Steam, including the 2026 Season Pack and My Team. Object readers support the inspected Steam layouts with PE timestamps `1789118616` and `1794312636`. Swofty resolves the addresses for the running build; object ownership, vehicle identity and vector bounds are checked before values are shown. Unsupported builds or missing bindings produce an explicit message.

The native component and helper have automated host tests, including a core with no snapshot-service export. The current executable's timing and driver-name access paths were inspected statically; in-game verification of this corrected acquisition path is still required before release.
