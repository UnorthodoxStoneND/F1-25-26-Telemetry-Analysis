// SPDX-License-Identifier: MIT
#pragma once
#include "readers.h"
#include "live_api.h"
#include <cstdio>

namespace directlive {
struct Sites { uint64_t manager=0,results=0; };
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
 template<class R> void grid(R& r,const Sites& s,livewire::Frame& frame)const {
  using namespace swofty::game;
  uint64_t end=0;
  auto system=registry.find(s.results);
  auto list=readers::pointer(r,system+offsetof(SessionSystem,entries_begin));
  if(!list||!readers::read(r,system+offsetof(SessionSystem,entries_end),end)||
     end<list||(end-list)%8||(end-list)/8>64) return;
  auto count=(end-list)/8;
  for(unsigned n=0;n<frame.count;n++) {
   auto& c=frame.cars[n];
   if(c.vehicle>=count) continue;
   auto record=readers::pointer(r,list+c.vehicle*8);uint8_t vehicle=255;uint32_t a=0,b=0;
   if(readers::read(r,record+offsetof(SessionVehicleEntry,vehicle_id),vehicle)&&vehicle==c.vehicle&&
      readers::read(r,record+offsetof(SessionVehicleEntry,grid_position_a),a)&&
      readers::read(r,record+offsetof(SessionVehicleEntry,grid_position_b),b)&&(a^b)<count&&(a^b)<32) {
    c.grid=int32_t((a^b)+1);c.known|=livewire::GRID;
   }
  }
 }
 template<class R> bool fill(R& r,const Sites& s,livewire::Frame& frame,const SwoftyLive* api,const SwoftyCarOps* ops) {
  frame={};frame.player=-1;
  if(!liveapi::car_ops_available(ops)) {
   std::snprintf(frame.diagnostic,sizeof frame.diagnostic,"Swofty's car operations API is unavailable.");return false;
  }
  uint32_t count=0;
  if(ops->count(&count)!=SWOFTY_ASSET_OK||count>livewire::CARS) {
   std::snprintf(frame.diagnostic,sizeof frame.diagnostic,"Swofty's vehicle count is unavailable or invalid.");return false;
  }
  if(!count) return false;
  liveapi::Snapshot snapshot;snapshot.load(api,count);
  if(!snapshot.valid) {
   std::snprintf(frame.diagnostic,sizeof frame.diagnostic,"Swofty's Live State snapshot is unavailable or changing.");return false;
  }
  if((snapshot.session.have&SWOFTY_LIVE_SHAVE_TYPE)&&snapshot.session.type>=1&&snapshot.session.type<=18)
   frame.session_type=snapshot.session.type;
  snapshot.player(frame.player,count);
  registry.load(r,s.manager);
  uint64_t positions=0,grids=0;bool duplicate_positions=false,duplicate_grids=false;
  for(unsigned id=0;id<count;id++) {
   // The vehicle table also contains the safety car. Admit verified racers only.
   if(!snapshot.racer(id)) continue;
   void* handle=nullptr;uint32_t is_ai=0;
   if(ops->car(id,&handle,&is_ai)!=SWOFTY_ASSET_OK||!handle) continue;
   auto& c=frame.cars[frame.count++];c.vehicle=id;c.identity=reinterpret_cast<uint64_t>(handle);
   snapshot.apply(id,c,count);
   readers::wear(r,c.identity,c);readers::setup(r,c.identity,c);
   liveapi::setup(ops,id,c.identity,c);
   if(c.known&livewire::POSITION) {uint64_t bit=1ull<<c.position;duplicate_positions|=(positions&bit)!=0;positions|=bit;}
  }
  // Exported starting-grid fields support race position changes and fallback ordering.
  grid(r,s,frame);
  for(unsigned i=0;i<frame.count;i++) {
   auto& c=frame.cars[i];
   if(c.known&livewire::GRID) {uint64_t bit=1ull<<c.grid;duplicate_grids|=(grids&bit)!=0;grids|=bit;}
  }
  for(unsigned i=0;i<frame.count;i++) {
   if(duplicate_positions) frame.cars[i].known&=~livewire::POSITION;
   if(duplicate_grids) frame.cars[i].known&=~livewire::GRID;
  }
  if(frame.count) frame.state|=livewire::ACTIVE;
  else std::snprintf(frame.diagnostic,sizeof frame.diagnostic,"Swofty's car handles are unavailable.");
  return frame.count>0;
 }
};
}
