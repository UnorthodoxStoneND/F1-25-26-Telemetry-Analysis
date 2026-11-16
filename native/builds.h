// SPDX-License-Identifier: MIT
#pragma once
#include <cstdint>
namespace builds {
// Only inspected executable layouts. Global/function addresses are resolved by
// Swofty for the running build; these are not RVAs copied from another build.
// Current Steam: SHA256 46009bd31f49d93a92d73cbd0b66131d2b37964a92da7293168d3c3a872d9389.
constexpr uint32_t ORIGINAL_STEAM=1789118616u,CURRENT_STEAM=1794312636u;
constexpr bool supported(uint32_t stamp){return stamp==ORIGINAL_STEAM||stamp==CURRENT_STEAM;}
}
