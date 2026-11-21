# Publishing to GitHub and the Swofty Addon Browser

Use **one immutable ZIP per version** for both destinations. Do not prepare a browser-only archive or replace a published release asset under the same version.

## Required contents

Before building, update the version in `swofty.json` and `source/main.go`, the changelog and installation download names. Keep the following in every release:

- A non-empty description, category `utility`, and the public source `repository` URL in `swofty.json`.
- `icon` naming the included PNG/JPG/WebP artwork (not SVG), no larger than 2 MB. The current square artwork is `icon.png`.
- `compat.platforms: ["steam"]`, reflecting the game platform tested. Keep the launcher compatibility requirement and core dependency.
- The compiled Windows/Linux helpers and native DLL, README, changelog, MIT licence and required third-party notices. Source, build scripts, SDK headers and tests remain in the public repository and GitHub release source archive, outside the installable ZIP.

The packaging script includes the manifest's icon. The existing package-integrity check verifies the icon, metadata and archive contents, so missing browser requirements fail the same CI job used for GitHub packaging. No extra parallel CI job is needed.

## Build and validate

Follow the [development guide](development.md) to build and test. Use Go 1.24+ (CI pins 1.27.4), console subsystem, retained symbols/uncompressed debug information, Linux PIE/ASLR and the native /MD runtime setting. Then run:

```powershell
python source/package.py
python tools/checks/package_check.py
```

For release versions, build and validate the resulting `dist/F1_Telemetry_v<version>.zip` and its generated `.zip.sha256` checksum for GitHub and the browser. Retain the validated ZIP and matching checksum under `../Releases/<version>/`, alongside a `SHA256SUMS.txt` copy for publication. Existing files in Releases are left in place. Test versions go into `../Testing/` instead, with previous test ZIP/checksum pairs moved into `../Testing/Archive/` after the new package is verified. A new version or package has a new hash; older antivirus results do not cover the new archive or rebuilt executable.

## GitHub

Create a branch, commit the source and icon, push it and open a PR against main. Wait for all required add-on/Swofty checks, then merge. Create the version tag from the merged main commit and attach the exact final ZIP and its `SHA256SUMS.txt` to the GitHub release. The release notes should identify the tested version and any outstanding validation.

Team badges in this add-on use the supported Swofty image API. Do not bundle separate experimental launcher builds or extracted game assets.

## Addon Browser

Creators need the Addon Creator role and publish at https://publish.f1.swofty.net. Upload the **same ZIP** attached to GitHub, supply the description and version changelog, and review the ownership/redistribution confirmation before submitting it yourself. The entry remains pending until reviewed. Treat an approved version's changelog as immutable and put later corrections in the next version.

The normal external VC++ v14 x64 runtime requirement remains the same as Swofty's. No runtime installer or game archives are bundled. The shared archive includes the code's MIT licence and third-party notices.

Reference: https://f1.swofty.net/reference/addons/publishing and https://f1.swofty.net/reference/addons/manifest.
