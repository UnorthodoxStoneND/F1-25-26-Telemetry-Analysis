# Installation

1. Download **`F1_Telemetry_v0.5.7.zip`** from [GitHub Releases](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/releases). Use the named ZIP, rather than GitHub's automatic Source code download. The same ZIP is prepared for the Addon Browser; availability there depends on review and approval.
2. Close the game. In Swofty, open **Addons**, select **Install addon from computer**, and choose the ZIP.
3. Enable **F1 Live Telemetry** in your active profile. Configure **Drivers shown** and **Highlight my row** in its settings.
4. Restart Swofty after replacing an earlier version, then launch the game through Swofty so its native component is loaded. Open **Live Telemetry** in the launcher.

No game UDP settings or port configuration are needed. Grid and garage values appear once the actual session is active and its underlying game systems become available; a lap time cannot appear before a completed lap exists.

## Updates and earlier recordings

Version 0.5.x is live-only. Existing recordings in Documents/F1 Game Telemetry are left untouched and cannot be reviewed in this version. Keep 0.4.2 if recording or post-session analysis is needed.

See [compatibility](../reference/compatibility.md) for supported launcher/game versions and [live data](../reference/live-data.md) for runtime behaviour.

## Microsoft Visual C++ runtime

The native DLL uses the same Visual C++ v14 x64 runtime family as Swofty Launcher. If Swofty reports a missing VCRUNTIME140.dll or DLL load error (including Windows error 126), install or repair [Microsoft's official x64 runtime](https://aka.ms/vc14/vc_redist.x64.exe), then restart Swofty and the game. The x86 package alone is insufficient. No runtime DLL or installer is bundled, and the add-on does not install it automatically.
