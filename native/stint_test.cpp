// SPDX-License-Identifier: MIT
#include "stint.h"
#include <cassert>
#include <iostream>
#include <limits>
livewire::Car car(uint32_t laps,float wear=10.f,bool pit=false) {
    livewire::Car c{};c.identity=1234;c.compound=18;c.visual=17;c.laps=laps;
    c.known=livewire::LAPS|livewire::TYRES|livewire::PIT;c.wear_known=15;
    if(pit)c.flags=livewire::IN_PIT_LANE;
    for(auto& w:c.wear)w=wear;return c;
}
void expect(stint::Counter& counter,livewire::Car& c,uint32_t laps) {
    counter.apply(c);if(!(c.known&livewire::STINT)||c.stint_laps!=laps)std::cerr<<"lap="<<c.laps<<" compound="<<c.compound<<" wear="<<c.wear[1]<<" got="<<c.stint_laps<<" expected="<<laps<<" known="<<c.known<<"\n";assert((c.known&livewire::STINT)&&c.stint_laps==laps);
}
int main() {
    stint::Counter counter;
    auto c=car(0,0);expect(counter,c,0);
    c=car(7,20);expect(counter,c,7);
    // Entering a pit lane alone does not reset a stint, nor does a drive-through.
    c=car(7,20.1f,true);expect(counter,c,7);
    c=car(8,20.2f);expect(counter,c,8);
    // Same-compound new set, reset once while stationary, then count up again.
    c=car(8,20.3f,true);expect(counter,c,8);
    c=car(8,0,true);expect(counter,c,0);
    c=car(8,.1f,true);expect(counter,c,0);
    c=car(9,.2f,true);expect(counter,c,1);
    c=car(9,0,true);expect(counter,c,1); // do not double-reset in the same stop
    c=car(9,.1f);expect(counter,c,1);
    c=car(12,8);expect(counter,c,4);
    // A visual designation change with the same physical compound is not a set.
    c.visual=16;expect(counter,c,4);
    // Changed actual compound resets even when the wear readings are missing.
    c.compound=19;c.wear_known=0;expect(counter,c,0);
    c.laps=13;expect(counter,c,1);
    // Another car's count cannot be reset by this car's pit stop.
    auto other=car(6,10);other.vehicle=1;other.identity=5678;expect(counter,other,6);
    // An unknown lap cannot fabricate a zero; publish the reset on a valid lap.
    c.compound=18;c.known&=~livewire::LAPS;counter.apply(c);assert(!(c.known&livewire::STINT));
    c.known|=livewire::LAPS;c.laps=14;expect(counter,c,0);
    c.laps=15;expect(counter,c,1);
    // Rewinds invalidate an unrecorded stint until the next change (or lap zero).
    c.laps=10;counter.apply(c);assert(!(c.known&livewire::STINT));
    c.laps=11;counter.apply(c);assert(!(c.known&livewire::STINT));
    c.compound=19;expect(counter,c,0);
    c.laps=0;expect(counter,c,0);
    // Replacement handles cannot inherit the previous driver's calculation.
    c.identity++;c.laps=7;counter.apply(c);assert(!(c.known&livewire::STINT));
    // Invalid/partial wear, wear changes away from pits, and float jitter do not reset.
    counter.reset();c=car(5,20);expect(counter,c,5);
    c=car(5,0);expect(counter,c,5);
    c=car(6,10,true);expect(counter,c,6);
    c=car(6,0,true);c.wear_known=7;expect(counter,c,6);
    c.wear_known=15;c.wear[0]=std::numeric_limits<float>::quiet_NaN();expect(counter,c,6);
    c=car(6,0,true);expect(counter,c,0);
    c=car(6,.005f);expect(counter,c,0);
    c=car(7,0,true);expect(counter,c,1);
    // Missing progress resumes without losing the current baseline.
    c.known&=~livewire::LAPS;counter.apply(c);assert(!(c.known&livewire::STINT));
    c=car(8,.1f);expect(counter,c,2);
    // Missing pit flags do not fabricate a stop.
    c=car(9,10);expect(counter,c,3);
    c=car(9,0,true);c.known&=~livewire::PIT;expect(counter,c,3);
    counter.reset();c=car(4,4);expect(counter,c,4);
    std::cout<<"PASS: in-memory stint counts, same/different compounds, pit/no-change, missing data, rewind and session/identity reset\n";
}
