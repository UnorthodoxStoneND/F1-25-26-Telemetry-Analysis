// SPDX-License-Identifier: MIT
#pragma once
#include "live_wire.h"
#include "swofty_game_types.hpp"
#include <cmath>
#include <cstring>
#include <limits>

// Read-only layouts for the inspected Steam builds listed in builds.h.
// Offsets and value meanings: upstream re/db/structs and native lap readers.
// These readers are independently implemented; no Swofty core source is bundled.
namespace readers {
template<class R,class T> bool read(R& r, uint64_t at, T& value) { return at>=0x10000 && r(at,&value,sizeof value); }
template<class R> uint64_t pointer(R& r,uint64_t at) { uint64_t p=0; return read(r,at,p) && p>=0x10000 && p<0x800000000000ull ? p:0; }
inline bool range(float f,float low,float high) { return std::isfinite(f) && f>=low && f<=high; }
inline void setting(livewire::Car& c,unsigned key,float v) { c.setup[key]=v; c.setup_known|=1u<<key; }

struct Slider { unsigned key,offset; float low,high,steps; };
constexpr Slider SLIDERS[] = {
    {2,0x214,10,100,18},{3,0x218,10,100,18},
    {4,0x228,-3.5f,-2.5f,10},{5,0x22c,-2,-1,10},
    {6,0x230,0,.2f,20},{7,0x234,.1f,.25f,15},
    {8,0x238,1,41,40},{9,0x23c,1,41,40},
    {10,0x250,1,21,20},{11,0x254,1,21,20}
};
template<class R> void setup(R& r,uint64_t car,livewire::Car& out,const Slider* sliders=SLIDERS,size_t count=sizeof(SLIDERS)/sizeof(*SLIDERS)) {
    auto tune=pointer(r,car+offsetof(swofty::game::CarLive,f1_tuning));
    if (!tune || pointer(r,tune+8)!=car) return;
    for(size_t n=0;n<count;n++) {
        const auto& s=sliders[n]; float f=0;
        if (read(r,tune+s.offset,f) && range(f,0,1) && s.steps>0)
            setting(out,s.key,s.low+std::round(f*s.steps)/s.steps*(s.high-s.low));
    }
    auto aero=pointer(r,car+offsetof(swofty::game::CarLive,f1_aero));
    for(unsigned n=0;n<2;n++) { uint32_t v=0;
        if(aero && pointer(r,aero+8)==car && read(r,aero+(n?0x19c0:0x19b4),v) && v<=100) setting(out,n,float(v));
    }
    uint32_t handling=0;
    if (read(r,car+offsetof(swofty::game::CarLive,handling_version),handling)) for(unsigned n=0;n<2;n++) { float f=0;
        if(read(r,tune+(handling>=7?0x248:0x240)+4*n,f) && range(f,0,1))
            setting(out,12+n,(n?40.f:15.f)+std::round(f*20));
    }
    // Brake values and fuel come exclusively from SwoftyCoreCarOps.
    auto suspension=pointer(r,car+offsetof(swofty::game::CarLive,suspension));
    if(suspension && pointer(r,suspension+8)==car) for(unsigned n=0;n<4;n++) {
        auto wheel=pointer(r,suspension+0x70+8*n); float pressure=0;
        // Car_Setup_SetTyrePressure stores the configured target at +0x9c.
        // +0x64 contains a derived spring coefficient, not tyre pressure.
        if(wheel && read(r,wheel+0x9c,pressure) && std::isfinite(pressure)) {
            // The physical wheel may use Pa, kPa, bar or psi. Reject implausible conversions.
            double psi=pressure>20000?pressure*.00014503773773:pressure>100?pressure*.14503773773:pressure<5?pressure*14.503773773:pressure;
            if(psi>=10 && psi<=50) setting(out, n<2?18+n:14+n, float(std::round(psi*10)/10));
        }
    }
}
}
