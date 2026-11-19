# GitHub validation checks

The single CI workflow lists each validation separately. Main
is protected by the ten add-on checks and six upstream compatibility checks.
Build shared test artifacts and Prepare Swofty validation tools are setup jobs;
their failure also fails the dependent required checks instead of skipping them.

| Required check | Scope |
| --- | --- |
| Unit tests | Packaging routing/checksums/archive preservation and remaining Go tests, including live views, tyre pressures, session state and read-only mapping lifecycle |
| Static analysis | Go vet |
| Launcher protocol compatibility | Released/future launcher handshake and unsupported API rejection |
| Live memory bounds | Bounded refresh memory |
| Live-only acquisition policy | No UDP, recording, analysis or image files; memory-only team badges |
| Native game readers | Reader, binding, Live State/CarOps acquisition and native/Go telemetry/badge shared-memory ABI tests |
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

The shared validation artifact includes the two consumed exported game-layout headers
for each release, alongside its SDK headers and validator. SDK jobs compare those
artifact copies without depending on the preparation job's source checkout.

The minimum in swofty_versions.json must match the manifest's launcher minimum.
Latest stable is resolved once per run, and its immutable commit and release tag
are recorded in the validation artifact and printed by each compatibility check.
Only the consumed native ABI and exported game fields are compared: a higher API version or appended
unused fields need not break compatibility. These checks do not run the actual
game or copy Swofty's unrelated application, recorder or network test suites.

Normal developer use of native/build.py still builds and runs its native tests.
The --build-only option is for CI's separately reported test jobs.

The package-integrity check also verifies the bundled browser icon, its size/shape, the description/category/repository fields and Steam compatibility metadata. The packaging script builds one complete ZIP for GitHub and the Addon Browser.

The additional **Linux helper and shared transport** job runs portable and Linux IPC/host-call tests plus vet on Ubuntu. The shared Windows build also cross-compiles the native Linux helper into the package. Native reader checks exercise the temporary file-mapping lifecycle; image fixtures cover both team badges and fitted tyre icons. These checks do not launch F1 25 under Proton.

Stint-counter regressions run with the native reader checks. They cover initial counts, same/different compounds, drive-throughs, invalid readings, rewinds and lifecycle resets; the real publisher test confirms counts reach the committed snapshot. Portable UI tests cover column placement, zero/unknown values, hidden-field refresh and saved-key compatibility.
