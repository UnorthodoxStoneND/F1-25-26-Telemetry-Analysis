// SPDX-License-Identifier: MIT
#pragma once
#include "direct_live.h"
#include <cmath>

namespace sessionmetrics {
struct Sites {
    uint64_t red_key=0,red_vt=0,race_vt=0,remaining=0;
};
// Read-only session route documented in the session KB: the RedFlagSystem
// owns a participants pointer at +0x158; that object's +0xf0 is the ruleset.
// Check both the owner vtable and the resolved getter before any game call.
template<class R> uint64_t ruleset(R& r,const Sites& s,const directlive::Registry& registry) {
    if(!s.red_key||!s.red_vt) return 0;
    auto red=registry.find(s.red_key);
    if(!red||readers::pointer(r,red)!=s.red_vt) return 0;
    auto participants=readers::pointer(r,red+0x158);
    return participants?readers::pointer(r,participants+0xf0):0;
}
inline int leader_lap(const livewire::Frame& frame,const SwoftyLive* api) {
    unsigned leader=livewire::CARS;
    for(unsigned i=0;i<frame.count;i++) {
        const auto& c=frame.cars[i];
        if((c.known&livewire::POSITION)&&c.position==1) leader=c.vehicle;
    }
    if(leader>=livewire::CARS) return -1;
    if(api&&api->version>=1&&api->available&&api->available()&&api->car) {
        SwoftyLiveCar c{};c.version=1;c.size=sizeof c;
        if(api->car(leader,&c)==SWOFTY_ASSET_OK&&c.version>=1&&c.size>=sizeof c&&c.vehicle==leader&&
           (c.have&(SWOFTY_LIVE_HAVE_CLASS|SWOFTY_LIVE_HAVE_PROGRESS))==(SWOFTY_LIVE_HAVE_CLASS|SWOFTY_LIVE_HAVE_PROGRESS)&&
           c.position==1&&c.lap>=0&&c.lap<1000) return c.lap;
    }
    return -1;
}
template<class R,class Timer> void fill(R& r,const Sites& s,
        const directlive::Registry& registry,livewire::Frame& frame,const SwoftyLive* api,Timer&& timer) {
    frame.state&=~(livewire::SESSION_TIMER|livewire::SESSION_LAPS);frame.progress=0;
    auto type=frame.session_type;
    bool timed=(type>=1&&type<=14)||(!type&&(frame.session_kind==livewire::PRACTICE||frame.session_kind==livewire::QUALIFYING));
    bool race=(type>=15&&type<=17)||(!type&&(frame.session_kind==livewire::RACE||frame.session_kind==livewire::SPRINT));
    if(!timed&&!race) return;
    auto rules=ruleset(r,s,registry),vt=readers::pointer(r,rules);
    if(!rules||!vt) return;
    if(timed&&s.remaining&&readers::pointer(r,vt+0xd8)==s.remaining) {
        float seconds=0;
        // The getter's FLT_MAX unlimited sentinel, NaN and invalid reads stay unknown.
        if(timer(s.remaining,rules,seconds)&&std::isfinite(seconds)&&seconds>=0&&seconds<=86400) {
            frame.progress=uint32_t(std::ceil(seconds));frame.state|=livewire::SESSION_TIMER;
        }
    } else if(race&&s.race_vt&&vt==s.race_vt) {
        uint32_t total=0;
        if(!readers::read(r,rules+0x358,total)||!total||total>1000) return;
        int completed=leader_lap(frame,api);
        if(completed<0||completed>int(total)) return;
        auto current=uint32_t(completed)+1;
        if(current>total)current=total;
        frame.progress=(total<<16)|current;frame.state|=livewire::SESSION_LAPS;
    }
}
}
