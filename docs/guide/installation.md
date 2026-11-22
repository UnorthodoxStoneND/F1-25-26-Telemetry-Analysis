# Installation

1. Download **`F1_Telemetry_v0.6.0.zip`** from the GitHub release, or install version 0.6.0 through Swofty once its browser submission is approved.
2. Close the game. In Swofty, open **Addons**, select **Install addon from computer**, and choose the ZIP.
3. Enable **F1 Live Telemetry** in your active profile. Configure **Drivers shown** and **Highlight my row** in its settings.
4. Choose **Launcher page** or **Retro window** under **Telemetry display**. Restart Swofty after replacing an earlier version, then launch the game through Swofty so its native component is loaded. Open **Live Telemetry** for launcher mode; window mode opens automatically once fresh session data is available. The inactive tab remains visible with the released launcher section API, without a telemetry table. Only one mode displays telemetry at a time.

No game UDP settings or port configuration are needed. Grid and garage values appear once the actual session is active and its underlying game systems become available; a lap time cannot appear before a completed lap exists.

## Updates and earlier recordings

Version 0.6.0 is live-only. Existing recordings in Documents/F1 Game Telemetry are left untouched and cannot be reviewed in this version. Keep 0.4.2 if recording or post-session analysis is needed.

See [compatibility](../reference/compatibility.md) for supported launcher/game versions and [live data](../reference/live-data.md) for runtime behaviour.

## Microsoft Visual C++ runtime

The native DLL uses the same Visual C++ v14 x64 runtime family as Swofty Launcher. If Swofty reports a missing VCRUNTIME140.dll or DLL load error (including Windows error 126), install or repair [Microsoft's official x64 runtime](https://aka.ms/vc14/vc_redist.x64.exe), then restart Swofty and the game. The x86 package alone is insufficient. No runtime DLL or installer is bundled, and the add-on does not install it automatically.
