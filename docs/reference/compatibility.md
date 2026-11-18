# Compatibility and testing

- **Operating system:** Windows x64.
- **Launcher:** Swofty 1.9.76 and compatible later versions retaining launcher API 1, native binding API 9, Live State API 2 or later, CarOps API 1 or later, session callbacks and memory-check API 1. No maximum launcher version is imposed; future breaking API changes may require an update.
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

Windows x64 is the supported platform. Automated Wine/Proton compatibility is not claimed.
