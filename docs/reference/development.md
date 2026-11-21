# Development

## Build and package

Use Go **1.24 or later** (CI remains pinned to 1.27.4), the Visual Studio C++ x64 build tools and an Ubuntu 24.04/WSL desktop build environment. The local release build uses Go 1.24.5. Install `golang-go gcc-mingw-w64-x86-64 libgl1-mesa-dev xorg-dev libxkbcommon-dev pkg-config` in Ubuntu. From the add-on root:

```powershell
python native/build.py
python source/build_helper.py --target both
python source/package.py
```

The Go helper retains symbols and uncompressed DWARF debug information (`-ldflags="-compressdwarf=false"`); path trimming removes local build paths. The helper uses the console subsystem, hidden by Swofty during normal use. The native DLL uses /MD and the same VC++ v14 x64 runtime family as Swofty. A separate /MT host checks real runtime loading and its missing-dependency diagnostic. Create the installable ZIP with `python source/package.py`. Antivirus results for earlier 0.4.x releases do not cover this executable or DLL.

Distributable helpers require `-tags desktop` and `CGO_ENABLED=1`; the build script sets these explicitly. Untagged builds intentionally retain a headless entry point for protocol/acquisition tests and must not replace the packaged helpers. Windows links only system graphics/runtime DLLs, without a WebView. Linux uses PIE (`-buildmode=pie`) for address randomization, retains uncompressed debug sections and links the system OpenGL/X11/libc stack; the Linux build targets Ubuntu 24.04 rather than promising compatibility with older distributions. The same helper process owns the protocol loop and optional window. Its UI receives copied presentation values through a bounded channel; only the protocol loop owns the reader and preference state.

The desktop renderer retains its root, row and image objects for normal updates. Row identity follows the session/car identity; reordering preserves nodes and the scroll container. Structural changes rebuild the layout. Tests exercise 2,000 consecutive updates and retained control callbacks. Compare update allocations with `go -C source test -tags desktop -run '^$' -bench BenchmarkRetroLiveUpdates -benchtime=500x -benchmem`; these software-driver benchmarks do not measure OpenGL driver memory or the OS working set.

The retro body uses a VBox-sized custom layout to animate registered row nodes. One Fyne animation interpolates the moving field for 250 ms on the UI thread. New rankings cancel the previous animation and use current node positions as the next starting points. Normal value refreshes do not reset movement. Hiding/rebuilding/resizing settles or cancels the old layout; identity replacement does not animate new rows. Tests inject a manual animation starter because Fyne's test driver otherwise completes every animation immediately. They verify intermediate frames, cancellation and bounded row tracking; `BenchmarkRetroSlideFrame` measures callback allocations separately from graphics rendering.

Window placement is saved globally for the add-on in `data_dir/retro-window.json` using a stable version-1 schema, not the executable's folder or version. Geometry checks run on the UI thread once per second; two matching observations persist a settled change, while hide/close/shutdown flush immediately. Windows uses normal `WINDOWPLACEMENT` bounds with workspace/desktop conversion and the parent launcher's monitor. Linux/X11 uses EWMH client PID, frame extents, work areas and move/resize/maximize requests through a dedicated X11 connection, with RandR monitor discovery. The Linux helper additionally links system libXrandr. Global placement cannot be guaranteed under native Wayland or when the launcher is not visible to X11. Monitor-recovery and persistence tests are portable; the optional native X11 test runs against an isolated virtual-display fixture.

Fyne creates the Windows HWND lazily inside `Show`, so applying bounds only afterward allowed an initial appearance on the default monitor. Test.6 prepares the chosen bounds before Show and uses a temporary thread-local CBT creation callback for the exact retro-window title. Cleanup immediately follows Show. Native hidden-window tests verify initial coordinates, exclusion of unrelated windows and cleanup; a private inactive-desktop Fyne startup probe verifies the actual show path without moving user windows. Existing preferences are never discarded to force a fresh placement. X11 still uses post-map WM requests.

In Ubuntu, run `GOTOOLCHAIN=go1.24.5 go -C source test -tags desktop ./...` for the renderer and background integration tests. Set `RETRO_SCREENSHOTS` to an absolute output directory when running `TestRetroRenderedViews` to export synthetic test screenshots. No game is launched by these tests. Regenerate desktop dependency notices with `python source/desktop_licenses.py` when dependencies change, review public licence contacts/font assets, and update their exact audited hashes in the privacy checker. `native/DESKTOP-TOOLCHAIN-LICENSES.txt` retains the MinGW/GCC runtime notices used by this build.

Test versions such as `0.5.9-test.5` package into `../Testing/` with a matching `.zip.sha256` file. After verifying the new ZIP, the packager moves previous test ZIPs and checksums into `../Testing/Archive/`. Identical rebuilds stay in place; changed rebuilds retain the old copy in Archive. Archive collisions use a separate rebuild folder to preserve both copies. Existing Releases are never rotated. Release versions continue to use `dist/`.

Use `--output dist` for isolated CI output or `--output <temporary-folder>` for reproducibility checks. Overrides outside Testing generate a ZIP and checksum without rotating the local test builds.

## Local validation

```powershell
go -C source vet ./...
go -C source test -trimpath -count=1 ./...
python tools/checks/live_policy.py
python tools/checks/package_test.py
python native/build.py
```

After building the helper and packaging the add-on, run:

```powershell
python tools/checks/build_contract.py
python tools/checks/lifecycle_check.py
python tools/checks/package_check.py --check integrity
python tools/checks/package_check.py --check privacy
```

See [CI validation](../../tools/checks/README.md) for the shared-artifact pipeline and the separate minimum/latest Swofty compatibility checks. In-game results must be recorded separately from automated checks.

## Repository layout

| Path | Purpose |
| --- | --- |
| `source/` | Go live helper, Go tests and ZIP packaging script |
| `native/` | Native publisher, checked readers, SDK headers and host tests |
| `tools/checks/` | Validation tools, launcher compatibility probes and CI reference |
| `docs/guide/` | Installation and user guidance |
| `docs/reference/` | Runtime behaviour, compatibility and development |
| `.github/` | CI workflow and dependency updates |
| `swofty.json` | Add-on manifest and launcher settings |
| `CHANGELOG.md` | Version history |
| `bin/`, `dist/`, `.verify/` | Ignored build, package and verification output |

Keep runtime code live-only: no UDP receiver or fallback, recording, post-session analysis or generated telemetry charts. Team badges may be converted to PNG in memory for launcher table cells; do not write badge image files. Do not commit game files, extracted assets, recordings or local build paths.

## Linux helper validation

CI runs portable Go tests and Linux-specific IPC/host-call tests on Ubuntu. From Linux, use `go -C source test -trimpath -count=1 ./...` and `go -C source vet ./...`. From Windows, `GOOS=linux` can cross-build the untagged headless test executable; run it in an existing Linux/WSL environment. Distributable desktop helpers require the CGO build script above. Native Wine/Proton IPC backing and cleanup are also checked by `native/proton_transport_test.cpp`. An in-game Proton test is still required before claiming full Linux support.

Retro column widths are additive `widths` maps in the existing version-1 `columns-<profile>.json` preferences, keyed by session group and field. Values are logical pixels (32-1200); absent values use weighted automatic sizing. Updates and field visibility changes retain widths; Reset defaults clears the current group. No telemetry is persisted. The launcher table owns its separate resize state and exposes no width-change event to the helper.
