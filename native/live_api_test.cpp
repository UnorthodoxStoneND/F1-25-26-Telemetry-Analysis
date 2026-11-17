// SPDX-License-Identifier: AGPL-3.0-or-later
#include "binding_fixture.h"
#include <iostream>

namespace {
SwoftyLiveSession header{};
SwoftyLiveCar cars[32]{};
bool enabled=true,changing=false;
uint64_t sequence=1;
int available(){return enabled?1:0;}
int session(SwoftyLiveSession* out){*out=header;return SWOFTY_ASSET_OK;}
int car(uint32_t id,SwoftyLiveCar* out){*out=cars[id];return SWOFTY_ASSET_OK;}
int seq(uint64_t* out){*out=sequence;sequence+=changing?1:0;return SWOFTY_ASSET_OK;}
SwoftyLive api{1,available,session,car,nullptr,nullptr,nullptr,nullptr,seq,nullptr};
void reset(unsigned count) {
 enabled=true;changing=false;header={};header.version=1;header.size=sizeof header;
 header.have=SWOFTY_LIVE_SHAVE_PLAYER|SWOFTY_LIVE_SHAVE_CARS;header.player=0;header.cars=count;
 for(unsigned id=0;id<count;id++) {
  auto& c=cars[id];c={};c.version=1;c.size=sizeof c;c.vehicle=id;
  c.have=SWOFTY_LIVE_HAVE_CLASS|SWOFTY_LIVE_HAVE_TYRE|SWOFTY_LIVE_HAVE_PIT_LANE;
  c.position=int(count-id);c.tyre_actual=20;c.tyre_visual=16;c.pit_status=id==0?2:0;
 }
}
}
int main() {
 for(unsigned count:{4u,24u,32u}) {
  BindingFixture f(count);directlive::Context context;livewire::Frame frame{};reset(count);
  // Observe reads, including failed attempts, to prove valid API fields bypass their readers.
  unsigned redundant=0;
  auto read=[&](uint64_t at,void* out,size_t n) {
   if(at==f.sites.controllers) redundant++;
   for(unsigned id=0;id<count;id++)
    if(at==f.results[id]+0x48||at==f.results[id]+0x90||at==f.tyre_objects[id]+0x9bc0) redundant++;
   for(unsigned id=0;id<27;id++) if(f.pit_records[id]&&at==f.pit_records[id]+0x103) redundant++;
   return f.memory(at,out,n);
  };
  bool filled=context.fill(read,f.sites,frame,&api);
  if(!filled||redundant||frame.player) std::cerr<<"routing: "<<count<<" filled="<<filled<<" redundant="<<redundant<<" player="<<frame.player<<"\n";
  assert(filled&&redundant==0&&frame.player==0);
  for(unsigned id=0;id<count;id++) {
   auto& c=frame.cars[id];assert(c.position==int(count-id)&&c.compound==20);
   assert((c.known&livewire::PIT)&&bool(c.flags&livewire::IN_PIT_LANE)==(id==0));
   assert(c.wear_known==15&&c.setup_known==(1u<<21)-1&&(c.flags>>16));
  }
  // Missing individual groups fall back without losing valid groups on other cars.
  cars[0].have=SWOFTY_LIVE_HAVE_PIT;header.have=SWOFTY_LIVE_SHAVE_CARS;
  assert(context.fill(read,f.sites,frame,&api)&&redundant>0&&frame.player==f.player);
  assert(frame.cars[0].position==1&&frame.cars[0].compound!=20);
  assert(!(frame.cars[0].flags&livewire::IN_PIT_LANE));
  assert(frame.cars[1].position==int(count-1)&&frame.cars[1].compound==20);
  // Malformed values and wrong vehicle IDs must use the checked binding readers.
  reset(count);cars[0].position=99;cars[0].tyre_actual=255;cars[0].pit_status=255;
  cars[1].vehicle=31;header.player=99;
  assert(context.fill(read,f.sites,frame,&api)&&frame.player==f.player);
  assert(frame.cars[0].position==1&&frame.cars[0].compound!=20);
  assert(frame.cars[1].compound!=20);
  reset(count);cars[0].size=0;
  assert(context.fill(read,f.sites,frame,&api)&&frame.cars[0].compound!=20);
  // API absence, unavailable build, mismatched count/header and snapshot changes retain 0.5.2 acquisition.
  for(unsigned mode=0;mode<6;mode++) {
   reset(count);
   if(mode==1) enabled=false;
   if(mode==2) header.cars++;
   if(mode==3) header.size=0;
   if(mode==4) changing=true;
   if(mode==5) api.version=0;
   assert(context.fill(read,f.sites,frame,mode==0?nullptr:&api));
   assert(frame.player==f.player&&frame.cars[0].position==1&&frame.cars[0].compound!=20);
   api.version=1;
  }
  reset(count);header.player=-1;
  assert(context.fill(read,f.sites,frame,&api)&&frame.player==-1);
  // Valid snapshots recover on the next sample after an unavailable/mixed frame.
  header.player=0;assert(context.fill(read,f.sites,frame,&api)&&frame.player==0&&frame.cars[0].compound==20);
 }
 std::cout<<"PASS: optional Live State routing, skipped direct reads, partial/malformed/mixed snapshots and fallback recovery\n";
}
