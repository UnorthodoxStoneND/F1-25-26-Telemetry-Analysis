// SPDX-License-Identifier: MIT
#pragma once
#include "live_wire.h"

// Bounded observations of Live State only. No lap history or game-memory reader.
namespace outlap {
struct Observation {
 uint64_t identity=0;
 uint32_t laps=0;
 bool known=false,armed=false,in_pit=false;
};
struct Tracker {
 Observation cars[livewire::CARS]{};
 uint64_t session=0;
 uint32_t type=0;
 void apply(livewire::Frame& frame,uint64_t serial) {
  if(session!=serial||type!=frame.session_type) {
   for(auto& c:cars)c={};
   session=serial;type=frame.session_type;
  }
  uint32_t seen=0;
  for(unsigned i=0;i<frame.count;i++) {
   auto& c=frame.cars[i];c.flags&=~livewire::OUT_LAP;
   if(c.vehicle>=livewire::CARS)continue;
   seen|=1u<<c.vehicle;
   auto& previous=cars[c.vehicle];
   // Only the documented practice and qualifying types, including sprint qualifying.
   if(type<1||type>14||!c.identity) {previous={};continue;}
   if(previous.identity!=c.identity)previous={c.identity,0,false,false};
   // Unknown samples break the inference: never guess an exit or lap boundary.
   if((c.known&(livewire::PIT|livewire::LAPS))!=(livewire::PIT|livewire::LAPS)) {
    previous={c.identity,0,false,false};continue;
   }
   if(previous.known&&c.laps<previous.laps)previous.armed=false;
   if(c.flags&livewire::IN_PIT_LANE) {
    previous.armed=true;
   } else if(previous.armed) {
    // Crossing the line ends the outlap. Pit entry itself always remains PIT.
    if(previous.known&&(previous.in_pit||c.laps==previous.laps))c.flags|=livewire::OUT_LAP;
    else previous.armed=false;
   }
   previous.laps=c.laps;previous.known=true;previous.in_pit=(c.flags&livewire::IN_PIT_LANE)!=0;
  }
  for(unsigned id=0;id<livewire::CARS;id++)if(!(seen&(1u<<id)))cars[id]={};
 }
};
}
