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
  }
  assert(!std::strcmp(frame.cars[frame.player].name,"Human Driver"));assert(frame.cars[count-1].compound==7&&frame.cars[count-2].compound==8);
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
