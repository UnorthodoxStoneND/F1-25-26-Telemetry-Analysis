# F1 25/26 Live Telemetry [Swofty Add-On]

Live telemetry and car setups inside Swofty Launcher for F1 25 and the 2026 Season Pack, including My Team. Version **0.5.8** is live-only: no UDP configuration, recording or post-session analysis.

## Features

- Live driver positions, full names, tyre compounds and pit-lane indicators.
- Individual wheel wear, last completed lap and best valid lap for each driver.
- Expandable car setups, including all four configured tyre pressures and current fuel.
- Full-grid or nearby-driver views, optional player highlighting and a resizable table.
- Data from Swofty's Live State API and checked native game bindings, with bounded shared memory and no telemetry files or uploads.

See the [live-data reference](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/docs/reference/live-data.md) for display behaviour, supported readings and process details.

## Install

Download **`F1_Telemetry_v0.5.8.zip`** from [GitHub Releases](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/releases). In Swofty, open **Addons** and select **Install addon from computer**. Enable **F1 Live Telemetry** in the active profile and restart Swofty and the game after updating.

The native DLL uses the same Microsoft Visual C++ v14 x64 runtime family as Swofty Launcher. If a runtime dependency is missing, install or repair [Microsoft's official x64 runtime](https://aka.ms/vc14/vc_redist.x64.exe).

Follow the [installation guide](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/docs/guide/installation.md) for the complete steps. This ZIP also contains the icon and metadata required for submission to Swofty's Addon Browser. Browser availability depends on approval.

## Supported platforms

- Windows x64.
- Swofty 1.9.55 and compatible later versions retaining the required APIs.
- F1 25 on Steam, including the 2026 Season Pack and My Team, on the supported game layouts.

See [compatibility and testing](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/docs/reference/compatibility.md) for the exact requirements and verification limits. For recording and post-session analysis, keep version 0.4.2; this version leaves existing recordings untouched.

## Contributing

Use Go **1.27.4**, Python and the Visual Studio C++ x64 build tools. From the repository root:

```powershell
go -C source vet ./...
go -C source test -trimpath -count=1 ./...
python native/build.py
```

See the [development guide](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/docs/reference/development.md) for build and packaging commands, the repository layout and local checks. The [CI check reference](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/tools/checks/README.md) lists each required validation. Report whether changes were tested in game when opening a PR. Use the [publishing guide](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/docs/reference/publishing.md) to prepare one complete ZIP for both GitHub Releases and the Swofty Addon Browser.

## Licence

Copyright (C) 2026 UnorthodoxStoneNDmodding. Licensed under the [MIT License](LICENSE) (`MIT`).

Third-party components retain their own licences: Go runtime and standard library (BSD-3-Clause), Swofty SDK (MIT), and Microsoft C++ Standard Library (Apache-2.0 WITH LLVM-exception). See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) and [native/SDK-LICENSE.txt](native/SDK-LICENSE.txt) for notices and Microsoft runtime details.

The installable ZIP includes the Go/native source, SDK header, build scripts and licence notices. Build instructions are provided in this README or its linked development guide. Earlier released archives retain the licence notices shipped with them.

## Credits

- [Swofty Launcher](https://github.com/Swofty-Developments/SwoftyLauncher) for the launcher, Live State API and native add-on SDK.
