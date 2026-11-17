# F1 25/26 Live Telemetry [Swofty Add-On]

Live telemetry and car setups inside Swofty Launcher for F1 25 and the 2026 Season Pack, including My Team. Version **0.5.5** is live-only: no UDP configuration, recording or post-session analysis.

## Features

- Live driver positions, full names, tyre compounds and pit-lane indicators.
- Individual wheel wear, last completed lap and best valid lap for each driver.
- Expandable car setups, including all four configured tyre pressures and current fuel.
- Full-grid or nearby-driver views, optional player highlighting and a resizable table.
- Data from Swofty's Live State API and checked native game bindings, with bounded shared memory and no telemetry files or uploads.

See the [live-data reference](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/docs/reference/live-data.md) for display behaviour, supported readings and process details.

## Install

Download **`F1_Telemetry_v0.5.5.zip`** from [GitHub Releases](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/releases), then select **Addons â†’ Install addon from computer** in Swofty. Enable **F1 Live Telemetry** in the active profile and restart Swofty and the game after updating.

Follow the [installation guide](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/docs/guide/installation.md) for the complete steps. The add-on is not listed in Swofty's Addon Browser.

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

See the [development guide](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/docs/reference/development.md) for build and packaging commands, the repository layout and local checks. The [CI check reference](https://github.com/UnorthodoxStoneND/F1-25-26-Telemetry-Analysis/blob/main/tools/checks/README.md) lists each required validation. Report whether changes were tested in game when opening a PR.

## Licence

[MIT](LICENSE). The Go runtime and vendored Swofty SDK retain their own notices; see [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) and [native/SDK-LICENSE.txt](native/SDK-LICENSE.txt).

## Credits

- [Swofty Launcher](https://github.com/Swofty-Developments/SwoftyLauncher) for the launcher, Live State API and native add-on SDK.
