// SPDX-License-Identifier: MIT
// Synthetic host-test objects only; never instantiated by the runtime DLL.
#pragma once
#include "direct_live.h"
#include <cassert>
#include <memory>
#include <string>
#include <vector>
struct TestMemory {
 struct Block {std::unique_ptr<char[]> data;size_t size;};
 std::vector<Block> blocks;
 uint64_t alloc(size_t n) {auto p=std::make_unique<char[]>(n);auto address=reinterpret_cast<uint64_t>(p.get());blocks.push_back({std::move(p),n});return address;}
 bool operator()(uint64_t at,void* dst,size_t n) {
  for(const auto& b:blocks) {auto begin=reinterpret_cast<uint64_t>(b.data.get());if(at>=begin && at-begin<=b.size && n<=b.size-(at-begin)){std::memcpy(dst,b.data.get()+at-begin,n);return true;}}
  return false;
 }
 template<class T> void put(uint64_t at,T value){char previous[sizeof(T)];assert((*this)(at,previous,sizeof(T)));std::memcpy(reinterpret_cast<void*>(at),&value,sizeof(T));}
};
struct BindingFixture {
 inline static BindingFixture* current=nullptr;
 TestMemory memory;
 directlive::Sites sites;
 uint64_t cars[32]{};
 unsigned count;
 int player;
 SwoftyLiveSession header{};
 SwoftyLiveCar live_cars[32]{};
 SwoftyLiveCarDetail details[32]{};
 int detail_result=SWOFTY_ASSET_OK;
 std::string names[32];
 float values[32][3]{};
 int value_results[3]{SWOFTY_ASSET_OK,SWOFTY_ASSET_OK,SWOFTY_ASSET_OK};
 int handle_results[32]{};
 bool live_enabled=true,ops_enabled=true,changing=false;
 uint64_t sequence=1;
 SwoftyLive live{};
 SwoftyCarOps ops{};
 static int available_live(){return current->live_enabled;}
 static int available_ops(){return current->ops_enabled;}
 static int read_session(SwoftyLiveSession* out){*out=current->header;return SWOFTY_ASSET_OK;}
 static int read_car(uint32_t id,SwoftyLiveCar* out){if(id>=current->count)return SWOFTY_ASSET_INVALID;*out=current->live_cars[id];return SWOFTY_ASSET_OK;}
 static int read_detail(uint32_t id,SwoftyLiveCarDetail* out){if(id>=current->count)return SWOFTY_ASSET_INVALID;*out=current->details[id];return current->detail_result;}
 static int read_seq(uint64_t* out){*out=current->sequence;current->sequence+=current->changing?1:0;return SWOFTY_ASSET_OK;}
 static int read_name(uint32_t id,char* out,uint32_t cap){
  if(id>=current->count||current->names[id].empty()||current->names[id].size()>=cap)return SWOFTY_ASSET_UNAVAILABLE;
  std::memcpy(out,current->names[id].c_str(),current->names[id].size()+1);return SWOFTY_ASSET_OK;
 }
 static int read_count(uint32_t* out){*out=current->count;return SWOFTY_ASSET_OK;}
 static int read_handle(uint32_t id,void** out,uint32_t* ai){
  if(id>=current->count)return SWOFTY_ASSET_INVALID;
  if(current->handle_results[id])return current->handle_results[id];
  *out=reinterpret_cast<void*>(current->cars[id]);*ai=uint32_t(int(id)!=current->player);return SWOFTY_ASSET_OK;
 }
 static int read_value(uint32_t id,const void* handle,uint32_t field,float* out){
  if(id>=current->count||reinterpret_cast<uint64_t>(handle)!=current->cars[id])return SWOFTY_ASSET_LATE;
  unsigned key=field==SWOFTY_CAR_FUEL_MASS?0:field==SWOFTY_CAR_BRAKES_BIAS?1:field==SWOFTY_CAR_BRAKES_PRESSURE?2:3;
  if(key==3)return SWOFTY_ASSET_INVALID;
  *out=current->values[id][key];return current->value_results[key];
 }
 explicit BindingFixture(unsigned n=24):count(n),player(int(n/2)) {
  current=this;auto& m=memory;assert(n>=4&&n<=32);
  live.version=6;live.car_detail=read_detail;live.available=available_live;live.session=read_session;live.car=read_car;live.seq=read_seq;live.driver_name=read_name;
  ops.version=1;ops.available=available_ops;ops.count=read_count;ops.car=read_handle;ops.get=read_value;
  header.version=1;header.size=sizeof header;header.type=1;header.cars=n;header.player=player;
  header.have=SWOFTY_LIVE_SHAVE_CARS|SWOFTY_LIVE_SHAVE_PLAYER|SWOFTY_LIVE_SHAVE_TYPE;
  sites.manager=m.alloc(8);
  for(unsigned id=0;id<n;id++) {
   auto& c=live_cars[id];c.version=1;c.size=sizeof c;c.vehicle=id;c.position=int(id+1);c.result_status=2;c.lap=3;
   c.have=SWOFTY_LIVE_HAVE_IDENT|SWOFTY_LIVE_HAVE_CLASS|SWOFTY_LIVE_HAVE_TYRE|SWOFTY_LIVE_HAVE_PIT_LANE|SWOFTY_LIVE_HAVE_PROGRESS;
   c.flags=SWOFTY_LIVE_CAR_RACER;
   c.tyre_actual=uint8_t(id==n-1?7:id==n-2?8:18);c.tyre_visual=uint8_t(id==n-1?7:id==n-2?8:16+id%3);c.pit_status=id==1?1:0;
   auto& d=details[id];d.version=1;d.size=sizeof d;d.vehicle=id;d.grid_position=uint8_t(n-id);
   d.have=SWOFTY_LIVE_HAVE_GRID|SWOFTY_LIVE_HAVE_TYRE_WEAR|SWOFTY_LIVE_HAVE_TYRE_AGE|SWOFTY_LIVE_HAVE_LAP_TIMES;
   d.laps_completed=3;d.tyre_age_laps=7;d.tyre_stint_laps=3;
   for(unsigned wheel=0;wheel<4;wheel++)d.tyre_wear[wheel]=5.f+float(wheel);
   names[id]="Driver "+std::to_string(id+1);values[id][0]=50;values[id][1]=.4f;values[id][2]=.98f;
   auto car=m.alloc(0xa000);cars[id]=car;
   auto tune=m.alloc(0x300),aero=m.alloc(0x1a00),susp=m.alloc(0x100);
   m.put(car+0x590,tune);m.put(car+0x570,aero);m.put(car+0x580,susp);m.put(car+0x9f40,uint32_t(8));
   for(auto component:{tune,aero,susp})m.put(component+8,car);
   for(auto slider:readers::SLIDERS)m.put(tune+slider.offset,.5f);
   m.put(tune+0x248,.5f);m.put(tune+0x24c,.5f);m.put(aero+0x19b4,uint32_t(20));m.put(aero+0x19c0,uint32_t(25));
   for(unsigned wheel=0;wheel<4;wheel++){
    auto physical_wheel=m.alloc(0x100);
    m.put(susp+0x70+wheel*8,physical_wheel);m.put(physical_wheel+0x9c,(wheel<2?22.f:24.f)+.1f*float(wheel+1)+.1f*float(id%4));
   }
  }
 }
 uint64_t resolve(const char* name)const {
  if(!std::strcmp(name,"g_Game_Manager"))return sites.manager;
  return 0;
 }
};
