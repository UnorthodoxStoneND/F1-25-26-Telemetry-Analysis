# Development

## Build and package

Use Go **1.27.4** and the Visual Studio C++ x64 build tools. From the add-on root, build the native DLL and run its host tests with `python native/build.py`, then build the Go helper:

```powershell
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:GOAMD64 = "v1"
go -C source build -buildvcs=false -trimpath -ldflags "-H=windowsgui" -o ../bin/telemetry-live.exe .
```

The Go helper retains symbols and DWARF debug information; path trimming removes local build paths. The native DLL build remains unchanged. Create the installable ZIP with `python source/package.py`. Antivirus results for earlier 0.4.x releases do not cover this executable or DLL.

## Local validation

```powershell
go -C source vet ./...
go -C source test -trimpath -count=1 ./...
python tools/checks/live_policy.py
python native/build.py
```

After building the helper and packaging the add-on, run:

```powershell
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
| `.github/` | CI workflow |
| `swofty.json` | Add-on manifest and launcher settings |
| `CHANGELOG.md` | Version history |
| `bin/`, `dist/`, `.verify/` | Ignored build, package and verification output |

Keep runtime code live-only: no UDP receiver or fallback, recording, post-session analysis or image generation. Do not commit game files, extracted assets, recordings or local build paths.
