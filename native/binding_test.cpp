// SPDX-License-Identifier: MIT
#include "binding_fixture.h"
#include "builds.h"
#include "session_metrics.h"
#include <limits>
#include <iostream>
int main() {
 {
  BindingFixture fixture(4);TestMemory m;sessionmetrics::Sites sites;directlive::Registry registry;
  auto red=m.alloc(0x160),participants=m.alloc(0x100),rules=m.alloc(0x360),vt=m.alloc(0x200);
  sites.red_key=123;sites.red_vt=m.alloc(8);sites.race_vt=vt;sites.remaining=m.alloc(8);
  m.put(red,sites.red_vt);m.put(red+0x158,participants);m.put(participants+0xf0,rules);
  m.put(rules,vt);m.put(vt+0xd8,sites.remaining);
  registry.count=1;registry.entries[0]={sites.red_key,red};registry.occupied[0]=1;
  livewire::Frame f{};f.state=livewire::ACTIVE;f.session_type=1;
  unsigned calls=0;float remaining=120.1f;
  auto timer=[&](uint64_t fn,uint64_t object,float& out){assert(fn==sites.remaining&&object==rules);calls++;out=remaining;return true;};
  sessionmetrics::fill(m,sites,registry,f,&fixture.live,timer);
  assert((f.state&livewire::SESSION_TIMER)&&f.progress==121&&calls==1);
  remaining=119;sessionmetrics::fill(m,sites,registry,f,&fixture.live,timer);assert(f.progress==119);
  for(float invalid:{-1.f,std::numeric_limits<float>::max(),std::numeric_limits<float>::quiet_NaN()}) {
   remaining=invalid;sessionmetrics::fill(m,sites,registry,f,&fixture.live,timer);
   assert(!(f.state&livewire::SESSION_TIMER)&&f.progress==0);
  }
  remaining=0;sessionmetrics::fill(m,sites,registry,f,&fixture.live,timer);assert((f.state&livewire::SESSION_TIMER)&&!f.progress);
  auto before=calls;m.put(red,uint64_t(0));sessionmetrics::fill(m,sites,registry,f,&fixture.live,timer);
  assert(!f.progress&&calls==before);m.put(red,sites.red_vt);
  m.put(vt+0xd8,uint64_t(0));sessionmetrics::fill(m,sites,registry,f,&fixture.live,timer);assert(calls==before);
  m.put(vt+0xd8,sites.remaining);
  m.put(rules+0x358,uint32_t(10));
  f.session_type=15;f.count=1;f.cars[0].vehicle=0;f.cars[0].position=1;f.cars[0].known=livewire::POSITION;
  sessionmetrics::fill(m,sites,registry,f,&fixture.live,timer);
  assert((f.state&livewire::SESSION_LAPS)&&!(f.state&livewire::SESSION_TIMER)&&f.progress==((10u<<16)|4));
  m.put(rules+0x358,uint32_t(3));sessionmetrics::fill(m,sites,registry,f,&fixture.live,timer);
  assert(f.progress==((3u<<16)|3)); // Finish line never displays total+1.
  fixture.live_cars[0].have&=~SWOFTY_LIVE_HAVE_PROGRESS;
  sessionmetrics::fill(m,sites,registry,f,&fixture.live,timer);
  assert(!(f.state&livewire::SESSION_LAPS)&&!f.progress);
  fixture.live_cars[0].have|=SWOFTY_LIVE_HAVE_PROGRESS;
  m.put(rules+0x358,uint32_t(0));sessionmetrics::fill(m,sites,registry,f,&fixture.live,timer);
  assert(!(f.state&livewire::SESSION_LAPS)&&f.progress==0);
  f.session_type=18;sessionmetrics::fill(m,sites,registry,f,&fixture.live,timer);assert(calls==before);
 }
 assert(builds::supported(1794312636u)&&builds::supported(1789118616u)&&!builds::supported(1));
 for(unsigned count:{4u,24u,32u}) {
  BindingFixture f(count);directlive::Context context;livewire::Frame frame{};
  f.live_cars[0].have&=~SWOFTY_LIVE_HAVE_CLASS;
  assert(context.fill(f.memory,f.sites,frame,&f.live,&f.ops));
  assert(frame.cars[0].grid==int(count)&&(frame.cars[0].known&livewire::GRID));
  f.memory.put(f.results[0],uint8_t(255));
  assert(context.fill(f.memory,f.sites,frame,&f.live,&f.ops));
  assert(!(frame.cars[0].known&livewire::GRID));
  f.memory.put(f.results[0],uint8_t(0));f.memory.put(f.results[0]+0x90,uint32_t(100));
  assert(context.fill(f.memory,f.sites,frame,&f.live,&f.ops));
  assert(!(frame.cars[0].known&livewire::GRID));
 }
 std::cout<<"PASS: exported grid bindings, session countdown and Live State race lap counter\n";
}
