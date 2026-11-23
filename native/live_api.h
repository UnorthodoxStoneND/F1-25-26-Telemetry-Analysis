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
 SwoftyLiveCarDetail details[32]{};
 char names[32][128]{};
 char teams[32][40]{};
 bool valid=false;
 void load(const SwoftyLive* api,unsigned count,bool analysis=false) {
  *this={};
  if(!api||api->version<6||!api->car_detail||!api->available||!api->session||!api->car||!api->seq||count>32||!api->available()) return;
  uint64_t before=0,after=0;
  if(api->seq(&before)!=SWOFTY_ASSET_OK) return;
  session.version=1;session.size=sizeof session;
  if(api->session(&session)!=SWOFTY_ASSET_OK||session.version<1||session.size<sizeof session) return;
  if((session.have&SWOFTY_LIVE_SHAVE_CARS)&&session.cars!=count) return;
  for(unsigned id=0;id<count;id++) {
   auto& c=cars[id];c.version=1;c.size=sizeof c;
   if(api->car(id,&c)!=SWOFTY_ASSET_OK||c.version<1||c.size<sizeof c||c.vehicle!=id) c={};
   auto& d=details[id];d.version=1;d.size=sizeof d;
   if(api->car_detail(id,&d)!=SWOFTY_ASSET_OK||d.version<1||d.size<sizeof d||d.vehicle!=id) d={};
   if(api->version>=2&&api->driver_name&&api->driver_name(id,names[id],sizeof names[id])!=SWOFTY_ASSET_OK)
    names[id][0]=0;
   if(!std::memchr(names[id],0,sizeof names[id])) names[id][0]=0;
   if(analysis&&racer_candidate(c)&&api->team_badge&&
      (api->team_badge(id,teams[id],sizeof teams[id])!=SWOFTY_ASSET_OK||
       !std::memchr(teams[id],0,sizeof teams[id]))) teams[id][0]=0;
  }
  valid=api->seq(&after)==SWOFTY_ASSET_OK&&before==after;
 }
 // Keep the core's classification, rather than rebuilding it from per-car
 // positions. Accept only a unique, complete order for the published racers.
 bool classification(livewire::Frame& frame,unsigned count)const {
  if(!valid||!(session.have&SWOFTY_LIVE_SHAVE_ORDER)||!session.classified||session.classified>32||count>32) return false;
  int index[32];for(auto& i:index)i=-1;
  for(unsigned i=0;i<frame.count;i++) {
   auto id=frame.cars[i].vehicle;
   if(id>=count||index[id]>=0) return false;
   index[id]=int(i);
  }
  livewire::Car ordered[32]{};unsigned used=0;uint32_t seen=0;
  for(unsigned i=0;i<session.classified;i++) {
   auto id=session.order[i];
   if(id>=count||(seen&(1u<<id))) return false;
   seen|=1u<<id;
   // Safety cars and racers without a current verified handle stay excluded.
   if(index[id]<0) continue;
   ordered[used++]=frame.cars[index[id]];
  }
  if(used!=frame.count) return false;
  std::memcpy(frame.cars,ordered,used*sizeof(livewire::Car));
  frame.state|=livewire::CLASSIFICATION_ORDER;return true;
 }
 bool player(int& out,unsigned count)const {
  if(!valid||!(session.have&SWOFTY_LIVE_SHAVE_PLAYER)||session.player< -1||session.player>=int(count)) return false;
  out=session.player;return true;
 }
 static bool racer_candidate(const SwoftyLiveCar& car) {
  return (car.have&SWOFTY_LIVE_HAVE_IDENT)&&(car.flags&SWOFTY_LIVE_CAR_RACER);
 }
 bool racer(unsigned id)const { return valid&&id<32&&racer_candidate(cars[id]); }
 void capture(unsigned id,livewire::AnalysisCar& out)const {
  const auto& c=cars[id];const auto& d=details[id];
  std::memcpy(out.team,teams[id],sizeof out.team);
  if(d.have&SWOFTY_LIVE_HAVE_LAP_TIMES) {
   out.known|=livewire::ANALYSIS_LAP;out.current_ms=d.current_lap_ms;out.best_lap_number=d.best_lap_number;
   std::memcpy(out.current_sectors,d.current_sector_ms,sizeof out.current_sectors);out.lap_flags=d.lap_flags;
  }
  if(d.have&SWOFTY_LIVE_HAVE_TYRE_AGE) out.stint_laps=d.tyre_stint_laps;
  if((c.have&SWOFTY_LIVE_HAVE_SPEED)&&readers::range(c.speed,0,250)) {
   out.known|=livewire::ANALYSIS_SPEED;out.speed=c.speed;
  }
  if((c.have&SWOFTY_LIVE_HAVE_PEDALS)&&readers::range(c.throttle,0,1)&&readers::range(c.brake,0,1)) {
   out.known|=livewire::ANALYSIS_PEDALS;out.throttle=c.throttle;out.brake=c.brake;
  }
  if((c.have&SWOFTY_LIVE_HAVE_GEAR)&&c.gear>=-1&&c.gear<=10) {out.known|=livewire::ANALYSIS_GEAR;out.gear=c.gear;}
  if((c.have&SWOFTY_LIVE_HAVE_RPM)&&readers::range(c.rpm,0,30000)) {out.known|=livewire::ANALYSIS_RPM;out.rpm=c.rpm;}
  if((c.have&SWOFTY_LIVE_HAVE_PROGRESS)&&c.lap>=0&&c.lap<1000&&readers::range(c.lap_m,-500,100000)) {
   out.known|=livewire::ANALYSIS_PROGRESS;out.lap=c.lap;out.lap_m=c.lap_m;
  }
  if((c.have&SWOFTY_LIVE_HAVE_GAP_LEADER)&&c.gap_leader_ms>=0&&c.gap_leader_ms<=10800000) {
   out.known|=livewire::ANALYSIS_GAP;out.gap_leader_ms=c.gap_leader_ms;
  }
  if(c.have&SWOFTY_LIVE_HAVE_PIT) {
   out.known|=livewire::ANALYSIS_PIT;out.pit_status=c.pit_status;out.pit_stops=c.pit_stops;
  }
 }
 void apply(unsigned id,livewire::Car& out,unsigned count)const {
  if(!valid||id>=32) return;
  const auto& c=cars[id];
  const auto& d=details[id];
  if((d.have&SWOFTY_LIVE_HAVE_GRID)&&d.grid_position>0&&d.grid_position<=count) {
   out.grid=d.grid_position;out.known|=livewire::GRID;
  }
  if(d.have&SWOFTY_LIVE_HAVE_TYRE_AGE) {
   out.tyre_age_laps=d.tyre_age_laps;out.known|=livewire::TYRE_AGE;
  }
  if(d.have&SWOFTY_LIVE_HAVE_TYRE_WEAR) for(unsigned wheel=0;wheel<4;wheel++) {
   if(readers::range(d.tyre_wear[wheel],0,100)) {
    out.wear[wheel]=d.tyre_wear[wheel];out.wear_known|=1u<<wheel;
   }
  }
  if(d.have&SWOFTY_LIVE_HAVE_LAP_TIMES) {
   if(d.laps_completed<1000) {out.laps=d.laps_completed;out.known|=livewire::LAPS;}
   auto lap=[&](uint32_t ms,const uint32_t* sectors,uint32_t& dest,uint32_t* dest_sectors,uint32_t time_bit,uint32_t sector_bit) {
    if(!ms||ms>=3600000) return;
    dest=ms;out.known|=time_bit;
    uint64_t sum=0;
    for(unsigned i=0;i<3;i++) {if(!sectors[i]||sectors[i]>=3600000)return;sum+=sectors[i];}
    if(sum+10<ms||sum>uint64_t(ms)+10) return;
    std::memcpy(dest_sectors,sectors,3*sizeof(uint32_t));out.known|=sector_bit;
   };
   // The core supplies the completed last lap (including invalid laps) and
   // the best valid lap's own sectors, never independent sector minima.
   lap(d.last_lap_ms,d.last_sector_ms,out.last_ms,out.last_sectors,livewire::LAST,livewire::LAST_SECTORS);
   lap(d.best_lap_ms,d.best_sector_ms,out.best_ms,out.best_sectors,livewire::BEST,livewire::BEST_SECTORS);
  }
  if((c.have&SWOFTY_LIVE_HAVE_GAP_AHEAD)&&c.gap_ahead_ms>=0&&c.gap_ahead_ms<=10800000) {
   out.gap_ahead_ms=c.gap_ahead_ms;out.known|=livewire::GAP_AHEAD;
  }
  if((c.have&SWOFTY_LIVE_HAVE_CLASS)&&readers::range(c.time_penalty_s,0,3600)) {
   out.time_penalty_ms=uint32_t(std::round(c.time_penalty_s*1000.f));out.known|=livewire::TIME_PENALTY;
  }
  if(c.have&SWOFTY_LIVE_HAVE_PENALTIES) {
   out.drive_through=c.drive_through;out.stop_go=c.stop_go;out.known|=livewire::PENALTIES;
  }
  if(names[id][0]) {std::memcpy(out.name,names[id],sizeof out.name);out.known|=livewire::NAME;}
  if((c.have&SWOFTY_LIVE_HAVE_CLASS)&&c.result_status<=7) {
   out.known|=livewire::RESULT;
   out.flags=(out.flags&~livewire::RESULT_MASK)|(uint32_t(c.result_status)<<livewire::RESULT_SHIFT);
  }
  if((c.have&SWOFTY_LIVE_HAVE_CLASS)&&c.position>0&&c.position<=int(count)) {
   out.position=c.position;out.known|=livewire::POSITION;
  }
  // These are launcher-provided GameDB stat IDs, not an F1 compound enum.
  // F2, historical and add-on tyre definitions can use other byte IDs. Keep
  // the core's unknown sentinel unknown; do not reapply an F1-only whitelist.
  if((c.have&SWOFTY_LIVE_HAVE_TYRE)&&c.tyre_actual!=0xff&&c.tyre_visual!=0xff) {
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
