// SPDX-License-Identifier: MIT
#pragma once
#include "live_wire.h"
#include <cmath>
#include <cstring>
#include <limits>

// Read-only layouts for the verified Steam game build (PE timestamp 1789118616).
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
    auto tune=pointer(r,car+0x590);
    if (!tune || pointer(r,tune+8)!=car) return;
    for(size_t n=0;n<count;n++) {
        const auto& s=sliders[n]; float f=0;
        if (read(r,tune+s.offset,f) && range(f,0,1) && s.steps>0)
            setting(out,s.key,s.low+std::round(f*s.steps)/s.steps*(s.high-s.low));
    }
    auto aero=pointer(r,car+0x570);
    for(unsigned n=0;n<2;n++) { uint32_t v=0;
        if(aero && pointer(r,aero+8)==car && read(r,aero+(n?0x19c0:0x19b4),v) && v<=100) setting(out,n,float(v));
    }
    uint32_t handling=0;
    if (read(r,car+0x9f40,handling)) for(unsigned n=0;n<2;n++) { float f=0;
        if(read(r,tune+(handling>=7?0x248:0x240)+4*n,f) && range(f,0,1))
            setting(out,12+n,(n?40.f:15.f)+std::round(f*20));
    }
    auto brake=pointer(r,car+0x550);
    if(brake && pointer(r,brake+8)==car) {
        float f=0;
        if(read(r,brake+0x21dc,f) && range(f,0,1)) setting(out,14,std::round(f*100));
        // Bias is a setup fraction, not the displayed front-axle percentage.
        if(read(r,brake+0x21d8,f) && range(f,0,1)) setting(out,15,50+std::round(f*20));
    }
    auto suspension=pointer(r,car+0x580);
    if(suspension && pointer(r,suspension+8)==car) for(unsigned n=0;n<4;n++) {
        auto wheel=pointer(r,suspension+0x70+8*n); float pressure=0;
        if(wheel && read(r,wheel+0x64,pressure) && std::isfinite(pressure)) {
            // The physical wheel may use Pa, kPa, bar or psi. Reject implausible conversions.
            double psi=pressure>20000?pressure*.00014503773773:pressure>100?pressure*.14503773773:pressure<5?pressure*14.503773773:pressure;
            if(psi>=10 && psi<=50) setting(out, n<2?18+n:14+n, float(std::round(psi*10)/10));
        }
    }
    auto fuel=pointer(r,car+0x540); float kg=0;
    if(fuel && pointer(r,fuel+8)==car && read(r,fuel+0x100,kg) && range(kg,0,200)) setting(out,20,kg);
}
template<class R> bool protected_float(R& r,uint64_t at,float& out) {
    uint32_t value=0,key=0;
    if(!read(r,at,value)||!read(r,at+8,key)) return false;
    value^=key; std::memcpy(&out,&value,4); return std::isfinite(out);
}
struct TimingCache {
    uint64_t history=0,bits=0;
    unsigned index=0;
    float tip=0;
    uint8_t key=0;
    uint32_t last=0,best=0,known=0;
};
template<class R> void timing(R& r,uint64_t system,unsigned car,livewire::Car& out,TimingCache* cache=nullptr) {
    if(!system || car>=livewire::CARS) return;
    auto list=pointer(r,system+0x58); uint64_t end=0;
    if(!list || !read(r,system+0x60,end) || end<list || (end-list)/8>64 || car>=(end-list)/8) return;
    auto history=pointer(r,list+car*8); uint8_t current=0;
    if(!history || !read(r,history+0x38,current) || current>=200) return;
    float tip=0; uint64_t tip_bits=0; uint8_t tip_key=0;
    auto at=history+200*current;
    if(!protected_float(r,at+0xe8,tip)||!read(r,at+0xe0,tip_bits)||!read(r,at+0xd8,tip_key)) return;
    if(cache && cache->history==history && cache->index==current && cache->tip==tip && cache->bits==tip_bits && cache->key==tip_key) {
        out.last_ms=cache->last;out.best_ms=cache->best;out.known|=cache->known;return;
    }
    float last=0,best=std::numeric_limits<float>::infinity();
    for(unsigned lap=0;lap<=current;lap++) {
        const uint64_t record=history+lap*200; float seconds=0;
        if(!protected_float(r,record+0xe8,seconds) || seconds<=0 || seconds>=3600) continue;
        last=seconds; uint64_t bits=0; uint8_t key=0;
        if(read(r,record+0xe0,bits) && read(r,record+0xd8,key) && (bits&1)==key && seconds<best) best=seconds;
    }
    uint8_t after=0;
    if(!read(r,history+0x38,after)||after!=current||pointer(r,list+car*8)!=history) return;
    if(last>0) { out.last_ms=uint32_t(last*1000.f);out.known|=livewire::LAST; }
    if(std::isfinite(best)) { out.best_ms=uint32_t(best*1000.f);out.known|=livewire::BEST; }
    if(cache) *cache={history,tip_bits,current,tip,tip_key,out.last_ms,out.best_ms,out.known&(livewire::LAST|livewire::BEST)};
}
}
