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
        m.put(susp+0x70+8*w,wheel);m.put(wheel+0x64,w<2?22.f:24.f);
    }
    m.put(aero+0x19b4,uint32_t(20));m.put(aero+0x19c0,uint32_t(25));m.put(brake+0x21dc,.98f);m.put(brake+0x21d8,.4f);m.put(fuel+0x100,50.f);
    for(auto s:readers::SLIDERS) m.put(tune+s.offset,.5f);
    m.put(tune+0x248,.5f);m.put(tune+0x24c,.5f);
    livewire::Car c{};readers::wear(m,car,c);readers::setup(m,car,c);
    assert(c.wear_known==15 && std::fabs(c.wear[2]-30)<.001);
    assert(c.setup_known==(1u<<21)-1 && c.setup[2]==55 && c.setup[4]==-3 && c.setup[12]==25 && c.setup[15]==58);
    assert(c.setup[16]==24 && c.setup[18]==22);
    // Invalid ownership and a missing wheel must never turn into a known zero.
    m.put(0x800000+0x10,uint64_t(car+8));m.put(0x801000+0x88,std::numeric_limits<float>::quiet_NaN());
    c={};readers::wear(m,car,c);assert(c.wear_known==12);
    m.put(tune+8,uint64_t(car+8));c={};readers::setup(m,car,c);assert(c.setup_known==0);
    // All cars use independent official records; invalid laps can be last, never best.
    const uint64_t system=0xb00000,list=0xc00000;
    m.block(system,0x100);m.block(list,32*8);m.put(system+0x58,list);m.put(system+0x60,list+32*8);
    for(unsigned id=0;id<32;id++) {
        uint64_t h=0xd00000+0x10000*id;m.block(h,0x1000);m.put(list+id*8,h);m.put(h+0x38,uint8_t(2));
        for(unsigned lap=0;lap<3;lap++){auto rec=h+lap*200;m.xfloat(rec+0xe8,80.f+float(id)-float(lap));m.put(rec+0xe0,uint64_t(lap==2?1:0));m.put(rec+0xd8,uint8_t(0));}
        c={};readers::timing(m,system,id,c);assert(c.last_ms==(78+id)*1000 && c.best_ms==(79+id)*1000);
        m.put(h+0x38,uint8_t(0));c={};readers::timing(m,system,id,c);assert(c.last_ms==(80+id)*1000 && c.best_ms==c.last_ms);
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
    std::cout<<"PASS: all 32 drivers, partial wear, setup units, invalid laps, rewind, pause, end/start, 9216-byte ABI\n";
}
