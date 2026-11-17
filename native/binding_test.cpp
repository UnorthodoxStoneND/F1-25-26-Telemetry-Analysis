// SPDX-License-Identifier: MIT
#include "binding_fixture.h"
#include "builds.h"
#include <iostream>
int main() {
 assert(builds::supported(1794312636u)&&builds::supported(1789118616u)&&!builds::supported(1));
 for(unsigned count:{4u,24u,32u}) {
  BindingFixture fixture(count);auto& m=fixture.memory;directlive::Context context;livewire::Frame frame{};
  assert(context.fill(m,fixture.sites,frame)&&frame.count==count&&frame.player==int(count/2));
  for(unsigned id=0;id<count;id++) {
   const auto& c=frame.cars[id];assert(c.vehicle==id&&c.position==int(id+1)&&c.grid==int(count-id));
   assert((c.known&(livewire::NAME|livewire::TYRES|livewire::POSITION))==(livewire::NAME|livewire::TYRES|livewire::POSITION));
   assert(c.setup_known==(1u<<21)-1 && c.setup[15]==58 && c.wear_known==15 && std::fabs(c.wear[2]-7)<.001);
   // Check every corner for player and other cars, with distinct per-car values.
   const unsigned pressure_keys[]={livewire::PRESSURE_RL,livewire::PRESSURE_RR,livewire::PRESSURE_FL,livewire::PRESSURE_FR};
   for(unsigned wheel=0;wheel<4;wheel++)assert(std::fabs(c.setup[pressure_keys[wheel]]-((wheel<2?22.f:24.f)+.1f*float(wheel+1)+.1f*float(id%4)))<.001);
   assert(bool(c.known&livewire::PIT)==(id<27));
   assert(bool(c.flags&livewire::IN_PIT_LANE)==(id==1));
  }
  // Actual lane membership, not a request to pit or a stationary garage car.
  m.put(fixture.pit_records[1]+0x103,uint8_t(0));assert(context.fill(m,fixture.sites,frame));
  assert((frame.cars[1].known&livewire::PIT) && !(frame.cars[1].flags&livewire::IN_PIT_LANE));
  m.put(fixture.pit_records[1]+0x103,uint8_t(1));m.put(fixture.pit_states[1]+4,uint32_t(4));
  assert(context.fill(m,fixture.sites,frame) && (frame.cars[1].flags&livewire::IN_PIT_LANE));
  m.put(fixture.pit_states[1]+4,uint32_t(99));assert(context.fill(m,fixture.sites,frame));
  assert(!(frame.cars[1].known&livewire::PIT));
  m.put(fixture.pit_states[1]+4,uint32_t(2));m.put(fixture.pit_records[1]+0x103,uint8_t(255));
  assert(context.fill(m,fixture.sites,frame) && !(frame.cars[1].known&livewire::PIT));
  auto pit_key=fixture.sites.pit;fixture.sites.pit=0;
  assert(context.fill(m,fixture.sites,frame) && frame.count==count);
  for(unsigned id=0;id<count;id++)assert(!(frame.cars[id].known&livewire::PIT));
  fixture.sites.pit=pit_key;m.put(fixture.pit_records[1]+0x103,uint8_t(0));
  assert(!std::strcmp(frame.cars[frame.player].name,("Driver "+std::to_string(frame.player+1)+" First"+std::to_string(frame.player+1)).c_str()));assert(frame.cars[count-1].compound==7&&frame.cars[count-2].compound==8);
  m.put(fixture.drivers[0]+0xc8,m.text("Norris"));m.put(fixture.drivers[0]+0x30,m.text("Lando"));
  m.put(fixture.drivers[1]+0xc8,m.text("Button"));m.put(fixture.drivers[1]+0x30,m.text("Jenson"));
  m.put(fixture.drivers[frame.player]+0xc8,m.text("Custom Surname"));m.put(fixture.drivers[frame.player]+0x30,m.text("Custom Given"));
  assert(context.fill(m,fixture.sites,frame));
  assert(!std::strcmp(frame.cars[0].name,"Norris Lando") && (frame.cars[0].flags>>16)==6);
  assert(!std::strcmp(frame.cars[1].name,"Button Jenson"));
  assert(!std::strcmp(frame.cars[frame.player].name,"Custom Surname Custom Given"));
  m.put(fixture.drivers[0]+0x30,uint64_t(0));assert(context.fill(m,fixture.sites,frame));
  assert(!std::strcmp(frame.cars[0].name,"Norris") && !(frame.cars[0].flags>>16));
  m.put(fixture.drivers[frame.player]+0xc8,uint64_t(0));assert(context.fill(m,fixture.sites,frame));
  assert(!std::strcmp(frame.cars[frame.player].name,"Human Driver")); // Existing custom player fallback.
  // Duplicate ranks are untrustworthy; absent controllers must not guess player zero.
  m.put(fixture.results[1]+0x48,uint32_t(0x12345678));fixture.sites.controllers=0;
  assert(context.fill(m,fixture.sites,frame)&&frame.player==-1);
  for(unsigned id=0;id<count;id++)assert(!(frame.cars[id].known&livewire::POSITION));
  // A changed car slot cannot be presented as a valid vehicle.
  m.put(fixture.cars[0]+0xe8,uint8_t(31));assert(context.fill(m,fixture.sites,frame)&&frame.count==count-1);
  m.put(fixture.sites.count,uint8_t(0));assert(!context.fill(m,fixture.sites,frame)&&!frame.count);
 }
 std::cout<<"PASS: binding-only 4/24/32-car grids, names, player, compounds, wear, setup, missing controllers and invalid objects\n";
}
