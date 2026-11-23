// SPDX-License-Identifier: MIT
#pragma once
#include "live_wire.h"
#include <cstring>
#include <initializer_list>

namespace livewire {
// Explicit end wins over any remaining core snapshot during unloading.
struct Session {
    Frame frame{};
    bool ended=false,paused=false,entered=false,permitted=false,gameplay=false;
    uint64_t serial=1;
    void clear(bool end) { frame=Frame{}; frame.player=-1; frame.state=end?ENDED:0; ended=end; paused=false; entered=!end; permitted=false; gameplay=false; ++serial; }
    // Career's trackside workstation is a child of StateSession too. A start
    // callback and readable cars alone therefore cannot authorise the display.
    bool allow(bool in_session,bool blocked) {
        if(blocked || !in_session) gameplay=false;
        bool next=entered && !ended && in_session && !blocked;
        if(next!=permitted) {
            frame=Frame{};frame.player=-1;if(!next)paused=false;++serial;
        }
        permitted=next;
        return next;
    }
    // StateSession and readable cars may exist behind the weekend selection UI.
    // Latch the actual game-flow node, not its SessionState* class-name hash.
    void flow(uint32_t hash,uint32_t status) {
        constexpr auto node_hash=[](const char* text) {uint32_t h=5381;for(;*text;text++)h=h*33+uint8_t(*text);return h;};
        // CareerHubUpdate may enter and leave between our 200 ms samples,
        // before the rewards screen and long before StateSession exits.
        // Consume the event, retaining the cleared latch until real entry.
        for(const char* node:{"StateLoadSession","StateUnloadSession","SessionStateCareerHubUpdate"})
            if(hash==node_hash(node) && status!=0) {gameplay=false;return;}
        if(hash==node_hash("StateSession") && (status==0||status==3)) {gameplay=false;return;}
        if(!entered || ended || (status!=1&&status!=2)) return;
        // BeginGameplay is lights-out in races. The earlier grid/pre-race
        // and garage transitions are already inside the actual session.
        for(const char* node:{"GridRundownType","PreRace","ZoomToMonitorCutscene","BeginGameplay","InGame"})
            if(hash==node_hash(node)) {gameplay=true;break;}
    }
    void event(const char* name) {
        if(!std::strcmp(name,"session.start")) clear(false);
        else if(!std::strcmp(name,"session.end")||!std::strcmp(name,"session.unload")) clear(true);
        else if(!std::strcmp(name,"pause")) paused=true;
        else if(!std::strcmp(name,"resume")) paused=false;
    }
    void stamp(uint64_t now) {
        std::memcpy(frame.magic,"USTLIVE6",8); frame.version=VERSION; frame.size=sizeof(Frame);
        frame.session=serial; frame.tick_ms=now;
        if(gameplay && permitted && (frame.state&ACTIVE)) frame.state|=WINDOW_READY; else frame.state&=~WINDOW_READY;
        if(paused && (frame.state&ACTIVE)) frame.state|=PAUSED; else frame.state&=~PAUSED;
    }
};
}
