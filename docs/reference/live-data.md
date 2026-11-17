# Live data

- Driver positions and names in **SURNAME Firstname** order, such as **NORRIS Lando**. Names come from the game's localised driver records, including custom drivers where available; the existing game name is retained when separate fields are unavailable. Choose either the full grid or your car and the nearest four cars. Nearest shows two ahead and two behind, shifting towards the available cars at either end of the grid.
- Actual tyre compound labels with native coloured circles. Dry tyre colours use the current soft/medium/hard designation; intermediates are green and wets blue.
- A highlighted **PIT** marker beside a driver's name while that car is in the pit lane. It follows the game's lane-membership reading, including a stationary pit stop; a missing reading does not fabricate a marker.
- Individual FL, FR, RL and RR wear bars and percentages, plus the average when all four readings are available.
- Last completed lap and best valid lap for each driver. Unavailable values appear as a dash.
- A setup control beneath each driver's row showing wings, differentials, camber, toe, suspension, anti-roll bars, ride heights, brakes, all four configured tyre pressures and fuel remaining. Unavailable readings show a dash. Fuel remaining is the current tank value, rather than the original setup fuel load.
- Optional highlighting of your own row. Display settings are in Swofty's add-on settings.
- Driver, wear and lap-time columns automatically share the available width when the launcher is resized or maximised. Narrow windows can still truncate content; maximising gives these columns more room.
- The table stays empty at the career trackside workstation and during session loading/unloading. Values appear in the actual session's garage or grid. Pausing an active session retains current values. Session end/unload clears them; closing the game releases the shared snapshot.

## Processes and local files

Swofty starts **`telemetry-live.exe`**, a small background helper with no separate window. It communicates with Swofty through local input/output pipes. Swofty loads **`telemetry-live.dll`** inside the game using its normal native add-on system. The DLL optionally uses `SwoftyCoreLive` for player identity, position, tyre compound and pit-lane status, skipping the corresponding direct readers when valid snapshot fields exist. Missing, unsupported or inconsistent snapshots fall back to the checked binding readers. Full names, wear, setups and completed lap times retain their direct readers. Two existing lap-timing functions are observed without changing their arguments or results. The add-on does not change car settings or game files.

Live State availability depends on Swofty's verified layouts for the running game build. This version therefore does not promise a memory reduction on every build; it avoids duplicate acquisition where the launcher already provides valid data. The older telemetry API, which can include UDP data, is not used.

The DLL publishes a fixed **9 KiB shared-memory snapshot** in Windows. The helper opens it read-only and displays the latest values. There is no network listener, web server, separate recording process, Python runtime or installed Windows service. The add-on retains current values and a small timing cache, rather than recording a session history.

The helper exits when Swofty closes its communication pipe. Disabling the add-on clears its display and closes the helper's snapshot handles; Swofty may keep the helper process running. A native component already loaded in a running game remains there until that game closes. Restart the game after enabling, disabling or updating a native add-on.

v0.5.6 writes no telemetry files, images or settings of its own. Swofty manages its profile settings and normal launcher logs. Existing data in **Documents/F1 Game Telemetry** is left untouched, but this live-only version does not open or analyse those recordings. Nothing is uploaded by the add-on.
