// SPDX-License-Identifier: MIT
#pragma once
#include "swofty_api.h"
#include "live_wire.h"
#include <cstring>

namespace sessioninfo {
// Context handles are only valid during their hook call. Keep just the result,
// never the handle. The core resolves sprint weekends before matching selectors.
inline uint32_t kind(const SwoftyHooks* hooks,const void* context) {
    if(!hooks||hooks->version<1||!hooks->context_in||!context) return livewire::SESSION_UNKNOWN;
    const char* selectors[]={
        "[{\"session_type\":[\"practice\"]}]",
        "[{\"session_type\":[\"qualifying\"]}]",
        "[{\"session_type\":[\"race\"]}]",
        "[{\"session_type\":[\"sprint\"]}]",
        "[{\"session_type\":[\"time_trial\"]}]"};
    uint32_t found=livewire::SESSION_UNKNOWN;
    for(unsigned i=0;i<5;i++) {
        int matches=0;
        if(hooks->context_in(context,{selectors[i],std::strlen(selectors[i])},&matches)!=SWOFTY_ASSET_OK)
            return livewire::SESSION_UNKNOWN;
        if(matches) {
            if(found) return livewire::SESSION_UNKNOWN;
            found=i+1;
        }
    }
    return found;
}
// Discard context that disagrees with a fresh numeric type. In particular, a
// sprint flag must not leak into the later Grand Prix (race_2).
inline uint32_t compatible_kind(uint32_t type,uint32_t kind) {
    if(kind>livewire::TIME_TRIAL) return livewire::SESSION_UNKNOWN;
    if(!type) return kind;
    const auto expected=type<=4?livewire::PRACTICE:type<=14?livewire::QUALIFYING:
        type<=17?livewire::RACE:type==18?livewire::TIME_TRIAL:livewire::SESSION_UNKNOWN;
    return kind==expected||(type==15&&kind==livewire::SPRINT)?kind:uint32_t(livewire::SESSION_UNKNOWN);
}
}
