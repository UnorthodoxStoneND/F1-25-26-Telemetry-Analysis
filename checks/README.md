# GitHub validation checks

The single Add-on and Swofty validation workflow lists each validation separately. Main
is protected by the ten add-on checks and six upstream compatibility checks and twelve upstream validation jobs.
Build shared test artifacts and Prepare Swofty validation tools are setup jobs;
their failure also fails the dependent required checks instead of skipping them.

| Required check | Scope |
| --- | --- |
| Unit tests | Remaining Go tests, including live views, tyre pressures, session state and read-only mapping lifecycle |
| Static analysis | Go vet |
| Launcher protocol compatibility | Released/future launcher handshake and unsupported API rejection |
| Live memory bounds | Bounded refresh memory |
| Live-only acquisition policy | No UDP, recording, analysis or image generation |
| Native game readers | Reader, binding, Live State fallback and native/Go shared-memory ABI tests |
| Native publisher lifecycle | Actual DLL with the mock core on unsupported and both supported game layouts |
| Helper lifecycle and shutdown | Actual helper protocol, disabled analysis, no data writes and EOF shutdown |
| Package integrity and versions | ZIP entries, versions, manifest and reproducibility |
| Privacy - source and ZIP | Personal paths and email checks |
| Swofty manifest - minimum supported | Manifest accepted by the pinned minimum release's real Rust validator |
| Swofty launcher views - minimum supported | Generated live views accepted by the minimum release's real view types |
| Swofty native SDK - minimum supported | Vendored SDK identity, consumed native ABI, callback signatures and constants |
| Swofty manifest - latest stable | Manifest accepted by the latest stable release's real Rust validator |
| Swofty launcher views - latest stable | Generated live views accepted by the latest stable release's real view types |
| Swofty native SDK - latest stable | Consumed native ABI, callback signatures and constants against latest stable |

Native DLLs, native test programs, the helper and the Go test executable are built
once per run and shared as artifacts. Unit tests exclude the handshake, memory
bound and native ABI tests: those run only in their dedicated checks. The ZIP is
created once; the integrity check creates one comparison ZIP to verify that it
is reproducible. Each upstream Rust validator is compiled once and reused by its
separate manifest and view checks. Identical minimum/latest commits share that
build, while both support roles remain explicitly listed.

The minimum in swofty_versions.json must match the manifest's launcher minimum.
Latest stable is resolved once per run, and its immutable commit and release tag
are recorded in the validation artifact and printed by each compatibility check.
Only the consumed native ABI is compared: a higher API version or appended
unused fields need not break compatibility. The add-on stays live-only. Upstream tests validate upstream source and do not enable UDP, recording or analysis in the add-on. Real-game tests retain upstream’s skips when F1 files are absent.

Normal developer use of native/build.py still builds and runs its native tests.
The --build-only option is for CI's separately reported test jobs.

## Complete upstream CI validation

- Swofty Rust (ubuntu-latest)
- Swofty Rust (windows-latest)
- Swofty UI (ubuntu-22.04)
- Swofty UI (windows-latest)
- Swofty compatibility gate tool
- Swofty installer (ubuntu-22.04)
- Swofty installer (windows-latest)
- Swofty documentation build
- Swofty standard library schemas
- Swofty native loader and Wine test
- Swofty release binaries - Linux
- Swofty release binaries - Windows

The source commit and complete mapping are in swofty_upstream_inventory.json. All twelve validations run for add-on PRs and main; add-on path changes cannot skip Wine or Rust tests. GitHub-hosted runners replace upstream’s AWS runner provisioning. The docs validation serves both upstream CI and Pages; deployment is not copied. Release binaries reuse each platform’s already built and tested UI executable. No release or website is published by this workflow.
