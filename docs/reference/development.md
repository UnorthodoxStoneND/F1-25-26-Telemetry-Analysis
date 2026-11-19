# Development

## Build and package

Use Go **1.27.4** and the Visual Studio C++ x64 build tools. From the add-on root, build the native DLL and run its host tests with `python native/build.py`, then build the Go helper:

```powershell
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:GOAMD64 = "v1"
go -C source build -buildvcs=false -trimpath -ldflags="-compressdwarf=false" -o ../bin/telemetry-live.exe .
$env:GOOS = "linux"
go -C source build -buildvcs=false -trimpath -buildmode=pie -ldflags="-compressdwarf=false" -o ../bin/telemetry-live-linux .
$env:GOOS = "windows"
```

The Go helper retains symbols and DWARF debug information; path trimming removes local build paths. Windows builds retain uncompressed DWARF (`-compressdwarf=false`) so the executable can be inspected without decompressing debug sections. A controlled 0.5.10 comparison retained identical executable code after the Go build ID and cleared the observed Microsoft/Bkav detections; this is not a guarantee for future binaries. The helper uses the console subsystem, hidden by Swofty during normal use. The native DLL uses /MD and the same VC++ v14 x64 runtime family as Swofty. A separate /MT host checks real runtime loading and its missing-dependency diagnostic. Create the installable ZIP with `python source/package.py`. Antivirus results for earlier releases or different builds do not cover a new executable or DLL.

Linux builds also retain uncompressed DWARF and use PIE (`-buildmode=pie`) for address randomization. The tested Linux helper requires the standard x86-64 ELF interpreter `/lib64/ld-linux-x86-64.so.2`, available in the Ubuntu WSL validation environment. This is an experimental Proton/Wine integration, not a claim of compatibility with every Linux distribution. The controlled PIE build cleared the observed Microsoft detection; every new release still needs its own validation.

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

CI runs portable Go tests and Linux-specific IPC/host-call tests on Ubuntu. From Linux, use `go -C source test -trimpath -count=1 ./...` and `go -C source vet ./...`. From Windows, `GOOS=linux` can cross-build the helper/test executable; run the test binary in an existing Linux/WSL environment. Native Wine/Proton IPC backing and cleanup are also checked by `native/proton_transport_test.cpp`. An in-game Proton test is still required before claiming full Linux support.
