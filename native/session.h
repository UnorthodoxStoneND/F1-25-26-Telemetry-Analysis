// SPDX-License-Identifier: MIT
#pragma once
#include "live_wire.h"
#include <cstring>

namespace livewire {
// Explicit end wins over any remaining core snapshot during unloading.
struct Session {
    Frame frame{};
    bool ended=false,paused=false;
    uint64_t serial=1;
    void clear(bool end) { frame=Frame{}; frame.player=-1; frame.state=end?ENDED:0; ended=end; paused=false; ++serial; }
    void event(const char* name) {
        if(!std::strcmp(name,"session.start")) clear(false);
        else if(!std::strcmp(name,"session.end")||!std::strcmp(name,"session.unload")) clear(true);
        else if(!std::strcmp(name,"pause")) paused=true;
        else if(!std::strcmp(name,"resume")) paused=false;
    }
    void stamp(uint64_t now) {
        std::memcpy(frame.magic,"USTLIVE1",8); frame.version=VERSION; frame.size=sizeof(Frame);
        frame.session=serial; frame.tick_ms=now;
        if(paused) frame.state|=PAUSED; else frame.state&=~PAUSED;
    }
};
}
