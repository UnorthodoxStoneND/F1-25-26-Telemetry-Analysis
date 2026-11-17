// SPDX-License-Identifier: AGPL-3.0-or-later
#pragma once
#include "live_wire.h"
#include <cstring>

namespace livewire {
// Explicit end wins over any remaining core snapshot during unloading.
struct Session {
    Frame frame{};
    bool ended=false,paused=false,entered=false,permitted=false;
    uint64_t serial=1;
    void clear(bool end) { frame=Frame{}; frame.player=-1; frame.state=end?ENDED:0; ended=end; paused=false; entered=!end; permitted=false; ++serial; }
    // Career's trackside workstation is a child of StateSession too. A start
    // callback and readable cars alone therefore cannot authorise the display.
    bool allow(bool in_session,bool blocked) {
        bool next=entered && !ended && in_session && !blocked;
        if(next!=permitted) {
            frame=Frame{};frame.player=-1;if(!next)paused=false;++serial;
        }
        permitted=next;
        return next;
    }
    void event(const char* name) {
        if(!std::strcmp(name,"session.start")) clear(false);
        else if(!std::strcmp(name,"session.end")||!std::strcmp(name,"session.unload")) clear(true);
        else if(!std::strcmp(name,"pause")) paused=true;
        else if(!std::strcmp(name,"resume")) paused=false;
    }
    void stamp(uint64_t now) {
        std::memcpy(frame.magic,"USTLIVE1",8); frame.version=VERSION; frame.size=sizeof(Frame);
        frame.session=serial; frame.tick_ms=now;
        if(paused && (frame.state&ACTIVE)) frame.state|=PAUSED; else frame.state&=~PAUSED;
    }
};
}
