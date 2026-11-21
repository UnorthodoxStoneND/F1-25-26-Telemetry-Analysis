// SPDX-License-Identifier: MIT
#include "outlap.h"
#include <cassert>
#include <iostream>
int main() {
 for(unsigned type=1;type<=18;type++) {
  outlap::Tracker tracker;livewire::Frame f{};f.session_type=type;f.count=32;
  for(unsigned id=0;id<32;id++) {
   auto& c=f.cars[id];c.vehicle=id;c.identity=1000+id;c.known=livewire::PIT|livewire::LAPS;c.laps=id;
  }
  tracker.apply(f,1);for(const auto& c:f.cars)assert(!(c.flags&livewire::OUT_LAP));
  for(auto& c:f.cars)c.flags|=livewire::IN_PIT_LANE;
  tracker.apply(f,1);for(const auto& c:f.cars)assert(!(c.flags&livewire::OUT_LAP));
  for(auto& c:f.cars)c.flags=0;
  // One exit passes the timing line between samples; still starts an outlap.
  f.cars[0].laps++;
  tracker.apply(f,1);
  for(const auto& c:f.cars)assert(bool(c.flags&livewire::OUT_LAP)==(type<=14));
  tracker.apply(f,1);
  for(const auto& c:f.cars)assert(bool(c.flags&livewire::OUT_LAP)==(type<=14));
  f.cars[1].laps++;tracker.apply(f,1);assert(!(f.cars[1].flags&livewire::OUT_LAP));
  if(type<=14)assert(f.cars[2].flags&livewire::OUT_LAP);
  f.cars[2].laps--;tracker.apply(f,1);assert(!(f.cars[2].flags&livewire::OUT_LAP));
  f.cars[3].identity++;tracker.apply(f,1);assert(!(f.cars[3].flags&livewire::OUT_LAP));
  f.cars[4].known&=~livewire::LAPS;tracker.apply(f,1);assert(!(f.cars[4].flags&livewire::OUT_LAP));
  f.cars[4].known|=livewire::LAPS;tracker.apply(f,1);assert(!(f.cars[4].flags&livewire::OUT_LAP));
  f.cars[5].known&=~livewire::PIT;tracker.apply(f,1);assert(!(f.cars[5].flags&livewire::OUT_LAP));
  tracker.apply(f,2);for(const auto& c:f.cars)assert(!(c.flags&livewire::OUT_LAP));
  // Repeated pit visits and changed session type start fresh observations.
  f.cars[0].flags=livewire::IN_PIT_LANE;tracker.apply(f,2);
  f.cars[0].flags=0;tracker.apply(f,2);assert(bool(f.cars[0].flags&livewire::OUT_LAP)==(type<=14));
  f.session_type=0;tracker.apply(f,2);assert(!(f.cars[0].flags&livewire::OUT_LAP));
 }
 std::cout<<"PASS: 32 independent outlaps, pit/exit/timing-line/completion, missing data, identity, rewind and lifecycle; race/TT excluded\n";
}
