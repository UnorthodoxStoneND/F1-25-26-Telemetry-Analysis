# Installation

1. Use **`F1_Telemetry_v0.7.0-prototype.zip`** for phase-one testing. Published packages are available on GitHub Releases and in the Swofty Addon Browser.
2. Close the game. In Swofty, open **Addons**, select **Install addon from computer**, and choose the ZIP.
3. Enable **F1 Live Telemetry & Analysis** in your active profile. Configure **Drivers shown** and **Highlight my row** in its settings.
4. Choose **Launcher page** or **Retro window** under **Telemetry display**. Restart Swofty after replacing an earlier version, then launch the game through Swofty so its native component is loaded. Open **Live Telemetry** for launcher mode; window mode opens automatically once fresh session data is available. The inactive tab remains visible with the released launcher section API, without a telemetry table. Only one mode displays telemetry at a time.

No game UDP settings or port configuration are needed. Grid and garage values appear once the actual session is active and its underlying game systems become available; a lap time cannot appear before a completed lap exists.

## Updates and earlier recordings

Enable **Telemetry Analysis** in the add-on settings to capture new sessions in Documents/F1 Live Telemetry. Older 0.4.x recordings use a different format in Documents/F1 Game Telemetry. See [recordings and exports](../reference/analysis.md).

See [compatibility](../reference/compatibility.md) for supported launcher/game versions and [live data](../reference/live-data.md) for runtime behaviour.

## Microsoft Visual C++ runtime

The native DLL uses the same Visual C++ v14 x64 runtime family as Swofty Launcher. If Swofty reports a missing VCRUNTIME140.dll or DLL load error (including Windows error 126), install or repair [Microsoft's official x64 runtime](https://aka.ms/vc14/vc_redist.x64.exe), then restart Swofty and the game. The x86 package alone is insufficient. No runtime DLL or installer is bundled, and the add-on does not install it automatically.
