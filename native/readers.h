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

template<class R> void wear(R& r,uint64_t car,livewire::Car& out) {
    uint64_t dynamics=pointer(r,car+0x900);
    if (!dynamics) return;
    for(unsigned wheel=0;wheel<4;wheel++) {
        auto tyre=pointer(r,dynamics+0x60+8*wheel);
        float fraction=0;
        if (tyre && pointer(r,tyre+0x10)==car && read(r,tyre+0x88,fraction) && range(fraction,0,1)) {
            out.wear[wheel]=fraction*100; out.wear_known|=1u<<wheel;
        }
    }
}
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
template<class R> bool protected_float(R& r,uint64_t at,float& out) {
    uint32_t value=0,key=0;
    if(!read(r,at,value)||!read(r,at+8,key)) return false;
    value^=key; std::memcpy(&out,&value,4); return std::isfinite(out);
}
struct CompletedLap {
    uint32_t ms=0;
    bool valid=false;
    uint32_t sectors[3]{};
};
inline bool same(const CompletedLap& a,const CompletedLap& b){return a.ms==b.ms&&a.valid==b.valid&&a.sectors[0]==b.sectors[0]&&a.sectors[1]==b.sectors[1]&&a.sectors[2]==b.sectors[2];}
// A running counter is positive even in the garage. The game's fastest-lap
// routine first requires all three sector times: the raw lap clock alone is
// not evidence that a lap has finished. Apply a sector-total check as well.
template<class R> CompletedLap completed_lap(R& r,uint64_t record) {
    float seconds=0,sectors[3]{};
    if(!protected_float(r,record+0xe8,seconds)||seconds<=5||seconds>=3600) return {};
    for(unsigned i=0;i<3;i++)
        if(!protected_float(r,record+0x48+16*i,sectors[i])||sectors[i]<=0||sectors[i]>=3600) return {};
    if(std::fabs(double(sectors[0])+sectors[1]+sectors[2]-seconds)>.010) return {};
    uint64_t bits=0;uint8_t key=0;
    if(!read(r,record+0xe0,bits)||!read(r,record+0xd8,key)||key>1) return {};
    return {uint32_t(seconds*1000.f),(bits&1)==key,{uint32_t(sectors[0]*1000.f),uint32_t(sectors[1]*1000.f),uint32_t(sectors[2]*1000.f)}};
}
struct TimingCache {
    uint64_t history=0;
    unsigned index=0;
    CompletedLap tip{};
    uint32_t last=0,best=0,known=0;
    uint32_t best_sectors[3]{};
};
template<class R> void timing(R& r,uint64_t system,unsigned car,livewire::Car& out,TimingCache* cache=nullptr) {
    if(!system || car>=livewire::CARS) return;
    auto list=pointer(r,system+0x58);uint64_t end=0;
    if(!list||!read(r,system+0x60,end)||end<list||(end-list)%8||(end-list)/8>64||car>=(end-list)/8) return;
    auto history=pointer(r,list+car*8);uint8_t current=0;
    if(!history||!read(r,history+0x38,current)||current>=200) return;
    auto tip=completed_lap(r,history+200*current);
    // Changes in an unfinished counter do not invalidate completed timings.
    if(cache&&cache->history==history&&cache->index==current&&same(cache->tip,tip)) {
        out.last_ms=cache->last;out.best_ms=cache->best;out.known|=cache->known;
        std::memcpy(out.best_sectors,cache->best_sectors,sizeof out.best_sectors);return;
    }
    uint32_t last=0,best=0;
    uint32_t best_sectors[3]{};
    for(unsigned lap=0;lap<=current;lap++) {
        auto value=lap==current?tip:completed_lap(r,history+lap*200);
        if(!value.ms) continue;
        last=value.ms;
        if(value.valid&&(!best||value.ms<best)) {
            best=value.ms;std::memcpy(best_sectors,value.sectors,sizeof best_sectors);
        }
    }
    uint8_t after=0;
    if(!read(r,history+0x38,after)||after!=current||pointer(r,list+car*8)!=history||!same(tip,completed_lap(r,history+200*current))) return;
    uint32_t known=(last?livewire::LAST:0)|(best?(livewire::BEST|livewire::BEST_SECTORS):0);
    out.last_ms=last;out.best_ms=best;out.known|=known;
    std::memcpy(out.best_sectors,best_sectors,sizeof best_sectors);
    if(cache) {
        *cache={history,current,tip,last,best,known,{}};
        std::memcpy(cache->best_sectors,best_sectors,sizeof best_sectors);
    }
}
}
