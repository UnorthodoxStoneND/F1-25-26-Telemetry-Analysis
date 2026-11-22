// SPDX-License-Identifier: MIT
#include "readers.h"
#include "session.h"
#include <cassert>
#include <fstream>
#include <iostream>
#include <map>
#include <vector>

struct Memory {
    std::map<uint64_t,std::vector<char>> blocks;
    void block(uint64_t at,size_t len) { blocks[at].resize(len); }
    bool operator()(uint64_t at,void* to,size_t len) {
        auto i=blocks.upper_bound(at);if(i==blocks.begin()) return false;--i;
        auto offset=at-i->first;
        if(offset>i->second.size()||len>i->second.size()-offset) return false;
        std::memcpy(to,i->second.data()+offset,len);return true;
    }
    template<class T> void put(uint64_t at,T value) {
        auto i=blocks.upper_bound(at);assert(i!=blocks.begin());--i;
        assert(at-i->first+sizeof value<=i->second.size());
        std::memcpy(i->second.data()+at-i->first,&value,sizeof value);
    }
    void xfloat(uint64_t at,float value) {
        uint32_t bits=0,key=0x531a1c7;std::memcpy(&bits,&value,4);put(at,bits^key);put(at+8,key);
    }
};
int main(int argc,char** argv) {
    Memory m;
    const uint64_t car=0x100000,dynamics=0x200000,tune=0x300000,aero=0x400000,brake=0x500000,susp=0x600000,fuel=0x700000;
    m.block(car,0xa000);m.block(dynamics,0x100);m.block(tune,0x300);m.block(aero,0x1a00);m.block(brake,0x2200);m.block(susp,0x100);m.block(fuel,0x200);
    m.put(car+0x900,dynamics);m.put(car+0x590,tune);m.put(car+0x570,aero);m.put(car+0x550,brake);m.put(car+0x580,susp);m.put(car+0x540,fuel);m.put(car+0x9f40,uint32_t(8));
    for(auto p:{tune,aero,brake,susp,fuel}) m.put(p+8,car);
    for(unsigned w=0;w<4;w++) {
        uint64_t tyre=0x800000+0x1000*w,wheel=0x900000+0x1000*w;
        m.block(tyre,0x100);m.block(wheel,0x100);m.put(dynamics+0x60+8*w,tyre);m.put(tyre+0x10,car);m.put(tyre+0x88,float(w+1)*.1f);
        // Game setup writer: +0x64 is a spring coefficient; +0x9c is pressure.
        m.put(susp+0x70+8*w,wheel);m.put(wheel+0x64,0.f);
        m.put(wheel+0x9c,(w<2?22.f:24.f)+.1f*float(w+1));
    }
    m.put(aero+0x19b4,uint32_t(20));m.put(aero+0x19c0,uint32_t(25));m.put(brake+0x21dc,.98f);m.put(brake+0x21d8,.4f);m.put(fuel+0x100,50.f);
    for(auto s:readers::SLIDERS) m.put(tune+s.offset,.5f);
    m.put(tune+0x248,.5f);m.put(tune+0x24c,.5f);
    livewire::Car c{};readers::setup(m,car,c);
    constexpr uint32_t remaining_setup=((1u<<21)-1)&~((1u<<14)|(1u<<15)|(1u<<20));
    if(c.setup_known!=remaining_setup||std::fabs(c.setup[16]-24.3f)>.001||
       std::fabs(c.setup[17]-24.4f)>.001||std::fabs(c.setup[18]-22.1f)>.001||std::fabs(c.setup[19]-22.2f)>.001) {
        std::cerr<<"FAIL: all four setup pressures must come from wheel+0x9c in FL/FR/RL/RR display order\n";return 1;
    }
    assert(c.setup[2]==55 && c.setup[4]==-3 && c.setup[12]==25);
    // Unrelated coefficients must not masquerade as plausible psi readings.
    for(unsigned w=0;w<4;w++)m.put(0x900000+0x1000*w+0x64,30.f);
    c={};readers::setup(m,car,c);assert(std::fabs(c.setup[16]-24.3f)<.001&&std::fabs(c.setup[18]-22.1f)<.001);
    // Existing unit normalisation and per-wheel invalid/missing handling.
    for(float scale:{1.f,1.f/14.503773773f,1.f/.14503773773f,1.f/.00014503773773f}) {
        for(unsigned w=0;w<4;w++)m.put(0x900000+0x1000*w+0x9c,(w<2?22.f:24.f)*scale);
        c={};readers::setup(m,car,c);assert(c.setup[16]==24&&c.setup[17]==24&&c.setup[18]==22&&c.setup[19]==22);
    }
    m.put(0x900000+0x9c,std::numeric_limits<float>::quiet_NaN());
    m.put(0x901000+0x9c,0.f);c={};readers::setup(m,car,c);
    assert(!(c.setup_known&(1u<<livewire::PRESSURE_RL))&&!(c.setup_known&(1u<<livewire::PRESSURE_RR)));
    assert((c.setup_known&(1u<<livewire::PRESSURE_FL))&&(c.setup_known&(1u<<livewire::PRESSURE_FR)));
    m.put(susp+0x80,uint64_t(0));c={};readers::setup(m,car,c);
    assert(!(c.setup_known&(1u<<livewire::PRESSURE_FL))&&(c.setup_known&(1u<<livewire::PRESSURE_FR)));
    m.put(tune+8,uint64_t(car+8));c={};readers::setup(m,car,c);assert(c.setup_known==0);
    livewire::Session life;life.frame.state=livewire::ACTIVE;life.frame.count=1;life.frame.player=0;
    life.event("pause");life.stamp(100);assert(life.frame.state&livewire::PAUSED && life.frame.count==1);
    life.event("resume");life.stamp(101);assert(!(life.frame.state&livewire::PAUSED));
    auto serial=life.serial;life.event("session.end");assert(life.ended && !life.frame.count && life.serial>serial);
    life.event("session.start");assert(!life.ended && !life.frame.count);
    life.frame.count=1;life.frame.state=livewire::ACTIVE;life.frame.player=0;
    life.frame.session_type=15;life.frame.session_kind=livewire::SPRINT;
    life.frame.cars[0].vehicle=0;life.frame.cars[0].identity=1234;life.frame.cars[0].known=livewire::NAME|livewire::POSITION|livewire::TYRES;
    life.frame.cars[0].position=1;life.frame.cars[0].compound=18;life.frame.cars[0].visual=17;std::memcpy(life.frame.cars[0].name,"ABI Driver",11);life.stamp(12345);
    auto& abi=life.frame.cars[0];abi.known|=livewire::LAPS|livewire::GAP_AHEAD|livewire::BEST|livewire::BEST_SECTORS;
    abi.known|=livewire::TYRE_AGE;abi.tyre_age_laps=3;abi.laps=7;abi.gap_ahead_ms=1234;abi.best_ms=80000;abi.best_sectors[0]=20000;abi.best_sectors[1]=30000;abi.best_sectors[2]=30000;
    abi.known|=livewire::LAST|livewire::LAST_SECTORS;abi.last_ms=81000;abi.last_sectors[0]=21000;abi.last_sectors[1]=30000;abi.last_sectors[2]=30000;
    abi.known|=livewire::PENALTIES|livewire::TIME_PENALTY;abi.drive_through=2;abi.stop_go=3;abi.time_penalty_ms=5250;
    if(argc>1){std::ofstream out(argv[1],std::ios::binary);out.write(reinterpret_cast<const char*>(&life.frame),sizeof life.frame);assert(out.good());}
    std::cout<<"PASS: setup pressures, missing/invalid setup, lifecycle and IPC v5 ABI\n";
}
