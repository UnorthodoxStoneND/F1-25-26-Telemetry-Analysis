// SPDX-License-Identifier: MIT
#pragma once
#include "swofty_api.h"
#include "live_wire.h"
#include "readers.h"

// Cached core snapshot. Never uses the older UDP-backed telemetry API.
namespace liveapi {
struct Snapshot {
 SwoftyLiveSession session{};
 SwoftyLiveCar cars[32]{};
 char names[32][128]{};
 bool valid=false;
 void load(const SwoftyLive* api,unsigned count) {
  *this={};
  if(!api||api->version<1||!api->available||!api->session||!api->car||!api->seq||count>32||!api->available()) return;
  uint64_t before=0,after=0;
  if(api->seq(&before)!=SWOFTY_ASSET_OK) return;
  session.version=1;session.size=sizeof session;
  if(api->session(&session)!=SWOFTY_ASSET_OK||session.version<1||session.size<sizeof session) return;
  if((session.have&SWOFTY_LIVE_SHAVE_CARS)&&session.cars!=count) return;
  for(unsigned id=0;id<count;id++) {
   auto& c=cars[id];c.version=1;c.size=sizeof c;
   if(api->car(id,&c)!=SWOFTY_ASSET_OK||c.version<1||c.size<sizeof c||c.vehicle!=id) c={};
   if(api->version>=2&&api->driver_name&&api->driver_name(id,names[id],sizeof names[id])!=SWOFTY_ASSET_OK)
    names[id][0]=0;
   if(!std::memchr(names[id],0,sizeof names[id])) names[id][0]=0;
  }
  valid=api->seq(&after)==SWOFTY_ASSET_OK&&before==after;
 }
 bool player(int& out,unsigned count)const {
  if(!valid||!(session.have&SWOFTY_LIVE_SHAVE_PLAYER)||session.player< -1||session.player>=int(count)) return false;
  out=session.player;return true;
 }
 bool racer(unsigned id)const {
  return valid&&id<32&&(cars[id].have&SWOFTY_LIVE_HAVE_IDENT)&&
   (cars[id].flags&SWOFTY_LIVE_CAR_RACER);
 }
 void apply(unsigned id,livewire::Car& out,unsigned count)const {
  if(!valid||id>=32) return;
  const auto& c=cars[id];
  if((c.have&SWOFTY_LIVE_HAVE_PROGRESS)&&c.lap>=0&&c.lap<1000) {
   out.laps=uint32_t(c.lap);out.known|=livewire::LAPS;
  }
  if((c.have&SWOFTY_LIVE_HAVE_GAP_AHEAD)&&c.gap_ahead_ms>=0&&c.gap_ahead_ms<=10800000) {
   out.gap_ahead_ms=c.gap_ahead_ms;out.known|=livewire::GAP_AHEAD;
  }
  if(names[id][0]) {std::memcpy(out.name,names[id],sizeof out.name);out.known|=livewire::NAME;}
  if((c.have&SWOFTY_LIVE_HAVE_CLASS)&&c.result_status<=7) {
   out.known|=livewire::RESULT;
   out.flags=(out.flags&~livewire::RESULT_MASK)|(uint32_t(c.result_status)<<livewire::RESULT_SHIFT);
  }
  if((c.have&SWOFTY_LIVE_HAVE_CLASS)&&c.position>0&&c.position<=int(count)) {
   out.position=c.position;out.known|=livewire::POSITION;
  }
  if((c.have&SWOFTY_LIVE_HAVE_TYRE)&&((c.tyre_actual>=16&&c.tyre_actual<=20)||c.tyre_actual==7||c.tyre_actual==8)&&
     ((c.tyre_visual>=16&&c.tyre_visual<=18)||c.tyre_visual==7||c.tyre_visual==8)) {
   out.compound=c.tyre_actual;out.visual=c.tyre_visual;out.known|=livewire::TYRES;
  }
  if((c.have&SWOFTY_LIVE_HAVE_PIT_LANE)&&c.pit_status<=2) {
   out.known|=livewire::PIT;
   if(c.pit_status) out.flags|=livewire::IN_PIT_LANE;
  }
 }
};
inline bool car_ops_available(const SwoftyCarOps* ops) {
 return ops&&ops->version>=1&&ops->available&&ops->count&&ops->car&&ops->get&&ops->available();
}
inline void setup(const SwoftyCarOps* ops,unsigned id,uint64_t handle,livewire::Car& out) {
 float value=0;
 const auto* car=reinterpret_cast<const void*>(handle);
 if(ops->get(id,car,SWOFTY_CAR_BRAKES_PRESSURE,&value)==SWOFTY_ASSET_OK&&readers::range(value,0,1))
  readers::setting(out,livewire::BRAKE_PRESSURE,std::round(value*100));
 if(ops->get(id,car,SWOFTY_CAR_BRAKES_BIAS,&value)==SWOFTY_ASSET_OK&&readers::range(value,0,1))
  readers::setting(out,livewire::BRAKE_BIAS,50+std::round(value*20));
 if(ops->get(id,car,SWOFTY_CAR_FUEL_MASS,&value)==SWOFTY_ASSET_OK&&readers::range(value,0,200))
  readers::setting(out,livewire::FUEL,value);
}
}
