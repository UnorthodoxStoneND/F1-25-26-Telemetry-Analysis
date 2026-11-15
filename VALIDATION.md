# v0.3.9 public-build validation

Prepared on Windows x64, 15 November 2026, using Go 1.24.5 and Python 3.14.

## Version and provenance

This is the archived v0.3.9 image-chart implementation, with its public author, addon/settings/cache identity and Go module changed to UnorthodoxStoneNDmodding. It does not contain the v0.4 native-chart preview. Version 0.3.9, launcher page names and Swofty compatibility >=1.9.42 remain unchanged.

The Windows executable is rebuilt from this renamed source with CGO disabled, -trimpath and -buildvcs=false. It is not the byte-identical archived executable. The addon ID is unorthodoxstonendmodding.udp-telemetry. Disable the old addon before enabling this public build; Swofty treats it as a separate addon and launcher options may need re-entering. Recordings remain in Documents/F1 Game Telemetry.

## Checks

The publication checks cover Go tests, vet, source formatting, Windows build, executable live UDP integration (2025/22 cars and 2026/24 cars), saved-session analysis integration, deterministic packaging, ZIP integrity and scans of tracked files, release contents and reachable Git metadata for the removed identity and email addresses. Results: 69 Go top-level tests passed, zero failed, six optional fixture checks skipped; vet, formatting, build and both integration suites passed. Rebuilt executable SHA-256: `cc0a82d96e6a8debb2c31455377e45fe560ea9925d67ec42869365505ae8e586`.

Tests use isolated temporary data and loopback ports. No game is launched; installed addons, recordings and career saves are not modified. Six optional checks depend on external fixtures. Automated checks do not establish support for every game build or mode.

## Privacy and scope

Public source and release contents contain no personal Windows build paths, email addresses or previous private author identity. Git author/committer/tagger identities use UnorthodoxStoneNDmodding with an empty email field. There is no earlier public Git history in the prepared repository.

No user recordings, game files, memory dumps or credentials are included. Tests use synthetic/sample identities. Driving data comes from UDP. Team-name metadata additionally uses external read-only process access on the verified Steam build; this may be unavailable on other builds. No game injection or game-file/memory/save writes are performed.

CI builds, tests and packages; it does not publish automatically. This preparation does not upload anything.
