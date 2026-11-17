# Compatibility and testing

- **Operating system:** Windows x64.
- **Launcher:** Swofty 1.9.55 and compatible later versions retaining launcher API 1, native binding API 9, session callbacks and memory-check API 1. No maximum launcher version is imposed; future breaking API changes may require an update.
- **Game:** F1 25 on Steam, including the 2026 Season Pack and My Team. Object readers support the inspected Steam layouts with PE timestamps `1789118616` and `1794312636`. Swofty resolves the addresses for the running build; object ownership, vehicle identity and vector bounds are checked before values are shown. Unsupported builds or missing bindings produce an explicit message.

Automated checks cover the live readers, Live State fallback, session transitions, completed lap timing, tyre-pressure wheel order, shared-memory format and launcher views. In-game testing confirmed setup values display for all cars, with the player's values checked against the game. Resource improvements have not been measured across every supported build.

## Automated validation

The [CI reference](../../tools/checks/README.md) lists sixteen individually reported validations. They check the add-on and its contracts against the minimum supported and latest stable Swofty releases. These checks do not replace an in-game test, antivirus scanning or validation of a future game update.

Windows x64 is the supported platform. Automated Wine/Proton compatibility is not claimed.
