// SPDX-License-Identifier: MIT
#pragma once
#include "readers.h"
#include "live_api.h"
#include <cstdio>

namespace directlive {
struct Sites { uint64_t manager=0; };
struct Entry {uint64_t key,value;};
struct Registry {
 Entry entries[4096]{};uint8_t occupied[4096]{};size_t count=0;
 template<class R> bool load(R& r,uint64_t slot) {
  using namespace swofty::game;
  count=0;auto manager=readers::pointer(r,slot);
  auto map=readers::pointer(r,manager+offsetof(GameManagerLive,systems_map));
  auto data=readers::pointer(r,map+offsetof(NeRobinHoodMapU64,slots));
  auto meta=readers::pointer(r,map+offsetof(NeRobinHoodMapU64,meta));
  if(!data||meta<=data||(meta-data)%sizeof(Entry)||(meta-data)/sizeof(Entry)>4096) return false;
  size_t n=size_t((meta-data)/sizeof(Entry));
  if(!r(data,entries,n*sizeof(Entry))||!r(meta,occupied,n)) return false;
  count=n;return true;
 }
 uint64_t find(uint64_t key)const {
  if(!key) return 0;
  for(size_t i=0;i<count;i++) if(occupied[i] && entries[i].key==key && entries[i].value>=0x10000) return entries[i].value;
  return 0;
 }
};
struct Context {
 Registry registry;
 template<class R> bool fill(R& r,const Sites& s,livewire::Frame& frame,const SwoftyLive* api,const SwoftyCarOps* ops,bool analysis=false) {
  frame={};frame.player=-1;
  if(!liveapi::car_ops_available(ops)) {
   std::snprintf(frame.diagnostic,sizeof frame.diagnostic,"Swofty's car operations API is unavailable.");return false;
  }
  uint32_t count=0;
  if(ops->count(&count)!=SWOFTY_ASSET_OK||count>livewire::CARS) {
   std::snprintf(frame.diagnostic,sizeof frame.diagnostic,"Swofty's vehicle count is unavailable or invalid.");return false;
  }
  if(!count) return false;
  liveapi::Snapshot snapshot;snapshot.load(api,count,analysis);
  if(!snapshot.valid) {
   std::snprintf(frame.diagnostic,sizeof frame.diagnostic,"Swofty's Live State snapshot is unavailable or changing.");return false;
  }
  if((snapshot.session.have&SWOFTY_LIVE_SHAVE_TYPE)&&snapshot.session.type>=1&&snapshot.session.type<=18)
   frame.session_type=snapshot.session.type;
  snapshot.player(frame.player,count);
  if(analysis) {
   const auto& live_session=snapshot.session;
   if(live_session.have&SWOFTY_LIVE_SHAVE_TRACK_NAME)
    std::memcpy(frame.analysis.track,live_session.track_name,sizeof frame.analysis.track);
   frame.analysis.safety_car_known=(live_session.have&SWOFTY_LIVE_SHAVE_SAFETY_CAR)!=0;
   frame.analysis.safety_car=live_session.safety_car;
   frame.analysis.chequered=live_session.chequered;
  }
  registry.load(r,s.manager);
  uint64_t positions=0,grids=0;bool duplicate_positions=false,duplicate_grids=false;
  for(unsigned id=0;id<count;id++) {
   // The vehicle table also contains the safety car. Admit verified racers only.
   if(!snapshot.racer(id)) continue;
   void* handle=nullptr;uint32_t is_ai=0;
   if(ops->car(id,&handle,&is_ai)!=SWOFTY_ASSET_OK||!handle) continue;
   auto& c=frame.cars[frame.count++];c.vehicle=id;c.identity=reinterpret_cast<uint64_t>(handle);
   snapshot.apply(id,c,count);
   if(analysis) snapshot.capture(id,frame.analysis_cars[id]);
   readers::setup(r,c.identity,c);
   liveapi::setup(ops,id,c.identity,c);
   if(c.known&livewire::POSITION) {uint64_t bit=1ull<<c.position;duplicate_positions|=(positions&bit)!=0;positions|=bit;}
  }
  for(unsigned i=0;i<frame.count;i++) {
   auto& c=frame.cars[i];
   if(c.known&livewire::GRID) {uint64_t bit=1ull<<c.grid;duplicate_grids|=(grids&bit)!=0;grids|=bit;}
  }
  for(unsigned i=0;i<frame.count;i++) {
   if(duplicate_positions) frame.cars[i].known&=~livewire::POSITION;
   if(duplicate_grids) frame.cars[i].known&=~livewire::GRID;
  }
  snapshot.classification(frame,count);
  if(frame.count) frame.state|=livewire::ACTIVE;
  else std::snprintf(frame.diagnostic,sizeof frame.diagnostic,"Swofty's car handles are unavailable.");
  return frame.count>0;
 }
};
}
