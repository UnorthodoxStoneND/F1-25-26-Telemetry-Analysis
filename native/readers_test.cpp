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
    livewire::Car c{};readers::wear(m,car,c);readers::setup(m,car,c);
    assert(c.wear_known==15 && std::fabs(c.wear[2]-30)<.001);
    if(c.setup_known!=((1u<<21)-1)||std::fabs(c.setup[16]-24.3f)>.001||
       std::fabs(c.setup[17]-24.4f)>.001||std::fabs(c.setup[18]-22.1f)>.001||std::fabs(c.setup[19]-22.2f)>.001) {
        std::cerr<<"FAIL: all four setup pressures must come from wheel+0x9c in FL/FR/RL/RR display order\n";return 1;
    }
    assert(c.setup[2]==55 && c.setup[4]==-3 && c.setup[12]==25 && c.setup[15]==58);
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
    // Invalid ownership and a missing wheel must never turn into a known zero.
    m.put(0x800000+0x10,uint64_t(car+8));m.put(0x801000+0x88,std::numeric_limits<float>::quiet_NaN());
    c={};readers::wear(m,car,c);assert(c.wear_known==12);
    m.put(tune+8,uint64_t(car+8));c={};readers::setup(m,car,c);assert(c.setup_known==0);
    // All cars use independent official records; invalid laps can be last, never best.
    const uint64_t system=0xb00000,list=0xc00000;
    m.block(system,0x100);m.block(list,32*8);m.put(system+0x58,list);m.put(system+0x60,list+32*8);
    for(unsigned id=0;id<32;id++) {
        uint64_t h=0xd00000+0x10000*id;m.block(h,0x1000);m.put(list+id*8,h);m.put(h+0x38,uint8_t(2));
        for(unsigned lap=0;lap<3;lap++){auto rec=h+lap*200;float seconds=80.f+float(id)-float(lap);m.xfloat(rec+0xe8,seconds);m.xfloat(rec+0x48,20.f);m.xfloat(rec+0x58,30.f);m.xfloat(rec+0x68,seconds-50);m.put(rec+0xe0,uint64_t(lap==2?1:0));m.put(rec+0xd8,uint8_t(0));}
        c={};readers::timing(m,system,id,c);assert(c.last_ms==(78+id)*1000 && c.best_ms==(79+id)*1000);
        m.put(h+0x38,uint8_t(0));c={};readers::timing(m,system,id,c);assert(c.last_ms==(80+id)*1000 && c.best_ms==c.last_ms);
    }
    // Garage regression: everyone can have the same advancing raw clock. It
    // must never become Last/Best, even with the default lap-valid bit set.
    for(unsigned id=0;id<32;id++) {
        auto h=uint64_t(0xd00000)+0x10000*id;readers::TimingCache cache{};
        m.put(h+0x38,uint8_t(0));m.xfloat(h+0x48,0);m.xfloat(h+0x58,0);m.xfloat(h+0x68,0);
        for(unsigned tick=0;tick<50;tick++){m.xfloat(h+0xe8,17.977f+float(tick));c={};readers::timing(m,system,id,c,&cache);assert(!(c.known&(livewire::LAST|livewire::BEST)));}
        // Two sectors and a positive clock are still not a completed lap.
        m.xfloat(h+0x48,20);m.xfloat(h+0x58,30);c={};readers::timing(m,system,id,c,&cache);assert(!c.last_ms&&!c.best_ms);
        // First flying lap, then an unfinished next lap with a changing clock.
        m.xfloat(h+0xe8,80);m.xfloat(h+0x68,30);c={};readers::timing(m,system,id,c,&cache);assert(c.last_ms==80000&&c.best_ms==80000);
        m.put(h+0x38,uint8_t(1));auto next=h+200;m.xfloat(next+0x48,0);m.xfloat(next+0x58,0);m.xfloat(next+0x68,0);
        for(unsigned tick=0;tick<50;tick++){m.xfloat(next+0xe8,float(tick));c={};readers::timing(m,system,id,c,&cache);assert(c.last_ms==80000&&c.best_ms==80000);}
        // Completing an invalid lap updates Last while Best stays on the valid lap.
        m.xfloat(next+0xe8,79);m.xfloat(next+0x48,20);m.xfloat(next+0x58,30);m.xfloat(next+0x68,29);m.put(next+0xe0,uint64_t(1));
        c={};readers::timing(m,system,id,c,&cache);assert(c.last_ms==79000&&c.best_ms==80000);
        // Inconsistent total, nonfinite sector, and a rewind cannot create a lap.
        m.xfloat(next+0x68,1);assert(!readers::completed_lap(m,next).ms);
        m.xfloat(next+0x68,std::numeric_limits<float>::quiet_NaN());assert(!readers::completed_lap(m,next).ms);
        m.put(h+0x38,uint8_t(0));m.xfloat(h+0x48,0);c={};readers::timing(m,system,id,c,&cache);assert(!c.last_ms&&!c.best_ms);
    }
    livewire::Session life;life.frame.state=livewire::ACTIVE;life.frame.count=1;life.frame.player=0;
    life.event("pause");life.stamp(100);assert(life.frame.state&livewire::PAUSED && life.frame.count==1);
    life.event("resume");life.stamp(101);assert(!(life.frame.state&livewire::PAUSED));
    auto serial=life.serial;life.event("session.end");assert(life.ended && !life.frame.count && life.serial>serial);
    life.event("session.start");assert(!life.ended && !life.frame.count);
    life.frame.count=1;life.frame.state=livewire::ACTIVE|livewire::API_READY;life.frame.player=0;
    life.frame.cars[0].vehicle=0;life.frame.cars[0].identity=1234;life.frame.cars[0].known=livewire::NAME|livewire::POSITION|livewire::TYRES;
    life.frame.cars[0].position=1;life.frame.cars[0].compound=18;life.frame.cars[0].visual=17;std::memcpy(life.frame.cars[0].name,"ABI Driver",11);life.stamp(12345);
    if(argc>1){std::ofstream out(argv[1],std::ios::binary);out.write(reinterpret_cast<const char*>(&life.frame),sizeof life.frame);assert(out.good());}
    std::cout<<"PASS: 32-driver advancing garage counters rejected; complete/partial/invalid laps, rewind, wear/setup, lifecycle and ABI\n";
}
