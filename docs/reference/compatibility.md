# Compatibility and testing

- **Operating system:** Windows x64. **Linux support is a work in progress (WIP) and may not be fully functional.** An experimental native Linux x64 helper is included for the game running under Proton/Wine; the complete in-game path has not been validated.
- **Launcher:** Swofty 1.9.80 and compatible later versions retaining launcher API 1, native binding API 9, Live State API 5 or later, CarOps API 1 or later, session callbacks and memory-check API 1. No maximum launcher version is imposed; future breaking API changes may require an update.
- **Game:** F1 25 on Steam, including the 2026 Season Pack and My Team. Object readers support the inspected Steam layouts with PE timestamps `1789118616` and `1794312636`. Swofty resolves the addresses for the running build; object ownership, vehicle identity and vector bounds are checked before values are shown. Unsupported builds or missing bindings produce an explicit message.

Automated checks cover the live readers, Live State/CarOps acquisition, session transitions, completed lap timing, tyre-pressure wheel order, shared-memory format and launcher views. In-game testing confirmed setup values display for all cars, with the player's values checked against the game. Resource improvements have not been measured across every supported build.

## In-game verification: 0.5.9-test.9 / 0.5.9

Release 0.5.9 promotes the tested runtime behaviour of test.9; the helper version label and release documentation change. On **18 November 2026**, the user confirmed that test.9 loaded correctly into My Team qualifying, then reported after further testing that everything appeared to be working well. No further badge delay or table-format error was reported. Individual coverage of every session format and game build was not enumerated.

### Resolved in user testing: initial standard-team badge delay

Observed during in-game testing of **0.5.9-test.8 on 18 November 2026**: the qualifying table and My Team badges appeared while standard-team badge cells remained blank. The user subsequently confirmed that the standard badges appeared without a restart or code change.

Launcher logs show standard badge requests progressing from pending to successful image results. The test.6-to-test.8 package comparison found no changes in badge acquisition or image conversion. In test.9, catalogue image requests now begin during session loading when Live State exposes verified racers, before the driver table is permitted. The user's subsequent testing reported normal loading and no further issues. The original delay's duration and the improvement have not been measured.

If the delay recurs, compare the first populated table and first standard badge times with image-service availability logs on both a fresh launch and a later session.

## Automated validation

The [CI reference](../../tools/checks/README.md) lists sixteen individually reported validations. They check the add-on and its contracts against the minimum supported and latest stable Swofty releases. These checks do not replace an in-game test, antivirus scanning or validation of a future game update.

Windows remains the in-game-validated platform. The Linux helper, protocol host call, read-only IPC mapping, coherent image snapshots, session clearing and saved preferences were tested in Ubuntu WSL. Windows host tests verify shared file-mapping visibility and deletion on last close. These checks do not establish in-game Proton compatibility; a Linux tester must verify the complete launcher/game path.

## Linux helper startup report and test.3

The user supplied a report of the launcher error **F1 Live Telemetry has no Linux helper** on 19 November 2026. The previous manifest declared only a Windows program. Test.3 adds a native Linux x86-64 helper and a temporary Wine/Proton IPC mapping instead of attempting to access Win32 named mappings from native Linux. No Linux process-memory reader or network listener was added.

Swofty's `addon_game_dir` host call supplies the actual installed game-copy path. Under Wine, the DLL creates a bounded temporary `.telemetry-live-*.ipc` mapping beside itself, shared read-only with the helper and deleted by the kernel on last close. Windows named mappings remain available, including for a Windows launcher running under Wine. Restart both launcher and game after updating. Any remaining Proton/runtime or path issue requires the tester's launcher/game logs.

## In-game verification: 0.5.10-test.3 tyre icons

On 19 November 2026, the user confirmed that the service-provided icons loaded for all five tested tyres. This confirms the tyre-image display in that test; it does not confirm Linux/Proton compatibility. Full in-game Linux/Proton testing remains outstanding.
