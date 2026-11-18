// SPDX-License-Identifier: MIT
#include "binding_fixture.h"
#include <limits>
#include <iostream>
#include <fstream>
#include "badges.h"
namespace {
bool custom_badge=true,broken_reference=false;
int image_result=SWOFTY_ASSET_OK,start_result=SWOFTY_ASSET_OK;
uint32_t image_width=2,image_height=1;
unsigned starts=0,clears=0;
unsigned image_calls=0,prefetch_cars=3;
bool image_live=false;
int image_available(){return image_live?1:0;}
int image_start(){starts++;return start_result;}
void image_clear(){clears++;}
int badge_reference(uint32_t,char* out,uint32_t cap) {
 if(broken_reference){std::memset(out,'x',cap);return SWOFTY_ASSET_OK;}
 const char* ref=custom_badge?"eaid://ui_package/art/textures/render_badge_eUserDefined 7":"#overrides.osd.timing_tower.team_badges_hud.ferrari_2026";
 if(std::strlen(ref)>=cap)return SWOFTY_ASSET_INVALID;
 std::memcpy(out,ref,std::strlen(ref)+1);return SWOFTY_ASSET_OK;
}
int badge_image(const char* ref,uint8_t* out,uint32_t cap,uint32_t* width,uint32_t* height,uint64_t* revision) {
 image_calls++;
 assert(ref&&cap==badgewire::PIXELS);
 *width=image_width;*height=image_height;*revision=7;
 const uint8_t pixels[]={255,0,0,64,0,255,0,255};
 std::memcpy(out,pixels,sizeof pixels);return image_result;
}
SwoftyLiveImages image_table{1,image_start,nullptr,badge_image,image_clear};
const SwoftyLiveImages* image_service(){return &image_table;}
int badge_session(SwoftyLiveSession* out) {
 out->have=SWOFTY_LIVE_SHAVE_CARS;out->cars=prefetch_cars;return SWOFTY_ASSET_OK;
}
int badge_car(uint32_t id,SwoftyLiveCar* out) {
 out->vehicle=id;out->have=SWOFTY_LIVE_HAVE_IDENT;out->flags=id<2?SWOFTY_LIVE_CAR_RACER:0;
 return SWOFTY_ASSET_OK;
}
void test_badges(const char* output) {
 static badges::Publisher publisher;
 SwoftyLive api{};api.version=4;api.available=image_available;api.team_badge_texture=badge_reference;api.images=image_service;
 livewire::Frame f{};f.state=livewire::ACTIVE;f.session=42;f.count=1;f.cars[0].vehicle=0;f.cars[0].identity=1234;
 // Runtime texture capture must be armed before Live State or a session exists.
 assert(!image_available());
 start_result=SWOFTY_ASSET_UNAVAILABLE;
 assert(!publisher.prepare(&api)&&starts==1);
 start_result=SWOFTY_ASSET_OK;
 assert(publisher.prepare(&api)==&image_table&&starts==2);
 assert(publisher.prepare(&api)==&image_table&&starts==2);
 image_live=true;publisher.sample(&api,f);
 auto& image=publisher.frame.cars[0];
 assert(image.identity==1234&&image.width==2&&image.height==1&&image.revision==7&&image.rgba[3]==64&&starts==2);
 assert(!std::strcmp(image.reference,"eaid://ui_package/art/textures/render_badge_eUserDefined 7"));
 custom_badge=false;publisher.sample(&api,f);assert(image.reference[0]=='#'&&starts==2);
 image_result=SWOFTY_ASSET_LATE;publisher.sample(&api,f);assert(!image.identity&&!image.width&&!image.reference[0]);
 image_result=SWOFTY_ASSET_UNAVAILABLE;publisher.sample(&api,f);assert(!image.identity);
 image_result=SWOFTY_ASSET_OK;image_width=65;publisher.sample(&api,f);assert(!image.identity);image_width=2;
 broken_reference=true;publisher.sample(&api,f);assert(!image.identity);broken_reference=false;
 api.version=3;publisher.sample(&api,f);assert(!image.identity);api.version=4;
 image_table.version=0;publisher.sample(&api,f);assert(!image.identity);image_table.version=1;
 publisher.started=nullptr;start_result=SWOFTY_ASSET_LATE;publisher.sample(&api,f);assert(!image.identity);
 start_result=SWOFTY_ASSET_OK;publisher.sample(&api,f);assert(image.identity==1234);
 f.state=0;f.session=43;publisher.sample(&api,f);
 assert(!image.identity&&!image.width&&publisher.frame.session==43&&clears==0);
 f.state=livewire::ACTIVE|64;publisher.sample(&api,f);assert(!image.identity);
 // Start catalogue work before any driver table is permitted. Poll pending work
 // during loading, deduplicate team-mates, and never publish prefetch pixels.
 api.session=badge_session;api.car=badge_car;custom_badge=false;
 image_calls=0;image_result=SWOFTY_ASSET_LATE;
 publisher.prefetch(&api);assert(image_calls==1&&!image.identity);
 image_result=SWOFTY_ASSET_OK;
 publisher.prefetch(&api);assert(image_calls==2&&!image.identity);
 f.state=livewire::ACTIVE;publisher.sample(&api,f);
 assert(image.identity==1234&&image.reference[0]=='#');
 image_live=false;publisher.prefetch(&api);assert(image_calls==3);image_live=true;
 prefetch_cars=33;publisher.prefetch(&api);assert(image_calls==3);prefetch_cars=3;
 // Cross-language fixture: caller-owned RGBA, custom reference and alpha.
 f.state=livewire::ACTIVE;f.session=42;custom_badge=true;publisher.sample(&api,f);
 if(output){std::ofstream out(output,std::ios::binary);out.write(reinterpret_cast<const char*>(&publisher.frame),sizeof publisher.frame);assert(out.good());}
}
}

int main(int argc,char** argv) {
 test_badges(argc>1?argv[1]:nullptr);
 for(unsigned count:{4u,24u,32u}) {
  BindingFixture f(count);directlive::Context context;livewire::Frame frame{};
  // Car objects have no legacy locks, controller, tyre, brake or fuel chains.
  // Any attempt to use those removed readers fails instead of passing silently.
  unsigned legacy_reads=0;
  auto read=[&](uint64_t at,void* out,size_t n) {
   for(unsigned id=0;id<count;id++)
    if(at==f.cars[id]+0x98||at==f.cars[id]+0x380||at==f.cars[id]+0x550||at==f.cars[id]+0x540||at==f.results[id]+0x48)
     legacy_reads++;
   return f.memory(at,out,n);
  };
  auto fill=[&]{return context.fill(read,f.sites,frame,&f.live,&f.ops);};
  assert(fill()&&frame.count==count&&frame.player==f.player&&frame.session_type==1);
  for(unsigned id=0;id<count;id++) {
   auto& c=frame.cars[id];
   assert(c.position==int(id+1)&&c.grid==int(count-id)&&c.identity==f.cars[id]);
   assert(!std::strcmp(c.name,f.names[id].c_str())&&!(c.flags>>16));
   assert((c.known&livewire::PIT)&&bool(c.flags&livewire::IN_PIT_LANE)==(id==1));
   assert(c.setup_known==(1u<<21)-1&&c.setup[14]==98&&c.setup[15]==58&&c.setup[20]==50&&c.wear_known==15);
  }
  assert(legacy_reads==0);
  f.live_cars[0].have|=SWOFTY_LIVE_HAVE_PROGRESS|SWOFTY_LIVE_HAVE_GAP_AHEAD;
  f.live_cars[0].lap=0;f.live_cars[0].gap_ahead_ms=0;
  assert(fill()&&(frame.cars[0].known&livewire::LAPS)&&(frame.cars[0].known&livewire::GAP_AHEAD));
  assert(!frame.cars[0].laps&&!frame.cars[0].gap_ahead_ms);
  f.live_cars[0].lap=7;f.live_cars[0].gap_ahead_ms=1234;
  assert(fill()&&frame.cars[0].laps==7&&frame.cars[0].gap_ahead_ms==1234);
  f.live_cars[0].lap=-1;f.live_cars[0].gap_ahead_ms=-1;
  assert(fill()&&!(frame.cars[0].known&(livewire::LAPS|livewire::GAP_AHEAD)));
  // Names are the launcher's UTF-8 display names, without splitting or reformatting.
  f.names[0]="Pérez";f.names[f.player]="Custom Player";
  assert(fill()&&!std::strcmp(frame.cars[0].name,"Pérez")&&!std::strcmp(frame.cars[f.player].name,"Custom Player"));
  f.names[0].clear();assert(fill()&&!(frame.cars[0].known&livewire::NAME));
  f.names[0]=std::string(128,'x');assert(fill()&&!(frame.cars[0].known&livewire::NAME));
  // Missing fields stay unknown. Only starting grid has a typed binding fallback.
  f.live_cars[0].have=SWOFTY_LIVE_HAVE_IDENT;f.header.have=SWOFTY_LIVE_SHAVE_CARS;
  assert(fill()&&frame.player==-1&&frame.session_type==0);
  auto c=frame.cars[0];assert(!(c.known&(livewire::POSITION|livewire::PIT|livewire::TYRES|livewire::RESULT)));
  assert((c.known&livewire::GRID)&&c.grid==int(count));
  f.live_cars[0].have=SWOFTY_LIVE_HAVE_IDENT|SWOFTY_LIVE_HAVE_CLASS|SWOFTY_LIVE_HAVE_TYRE|SWOFTY_LIVE_HAVE_PIT_LANE;
  f.live_cars[0].position=99;f.live_cars[0].result_status=255;f.live_cars[0].tyre_actual=255;f.live_cars[0].pit_status=255;
  assert(fill());c=frame.cars[0];
  assert(!(c.known&(livewire::POSITION|livewire::PIT|livewire::TYRES|livewire::RESULT)));
  f.live_cars[0].position=1;f.live_cars[0].tyre_actual=18;f.live_cars[0].pit_status=0;
  for(uint8_t status:{uint8_t(4),uint8_t(5),uint8_t(7)}) {
   f.live_cars[0].result_status=status;assert(fill());
   assert((frame.cars[0].known&livewire::RESULT)&&((frame.cars[0].flags&livewire::RESULT_MASK)>>8)==status);
  }
  // One unavailable setup group does not hide other fields or invoke a raw fallback.
  f.value_results[1]=SWOFTY_ASSET_UNAVAILABLE;assert(fill());
  assert(!(frame.cars[0].setup_known&(1u<<livewire::BRAKE_BIAS)));
  assert(frame.cars[0].setup_known&(1u<<livewire::FUEL));
  f.value_results[1]=SWOFTY_ASSET_OK;f.values[0][0]=std::numeric_limits<float>::quiet_NaN();
  f.values[0][1]=2;f.values[0][2]=-1;assert(fill());
  assert(!(frame.cars[0].setup_known&((1u<<livewire::FUEL)|(1u<<livewire::BRAKE_BIAS)|(1u<<livewire::BRAKE_PRESSURE))));
  f.handle_results[0]=SWOFTY_ASSET_LATE;assert(fill()&&frame.count==count-1&&frame.cars[0].vehicle==1);
  assert(legacy_reads==0);
  f.handle_results[0]=SWOFTY_ASSET_OK;
  f.changing=true;assert(!fill()&&!frame.count&&frame.diagnostic[0]);f.changing=false;
  f.header.cars++;assert(!fill());f.header.cars--;
  f.live_enabled=false;assert(!fill());f.live_enabled=true;
  f.ops_enabled=false;assert(!fill()&&frame.diagnostic[0]);f.ops_enabled=true;
  assert(!context.fill(read,f.sites,frame,nullptr,&f.ops));
  assert(!context.fill(read,f.sites,frame,&f.live,nullptr));
  f.count=33;assert(!fill());f.count=0;assert(!fill());
 }
 {
  // 24 racers plus a safety car: include only the SDK's verified racing vehicles.
  BindingFixture f(25);directlive::Context context;livewire::Frame frame{};
  f.names[24]="Safety Car";f.live_cars[24].flags=0;f.live_cars[24].position=1;
  auto fill=[&]{return context.fill(f.memory,f.sites,frame,&f.live,&f.ops);};
  assert(fill()&&frame.count==24);
  for(unsigned n=0;n<frame.count;n++)assert(frame.cars[n].vehicle!=24&&std::strcmp(frame.cars[n].name,"Safety Car"));
  // Retirements remain racers even without a current position.
  f.live_cars[3].position=0;f.live_cars[3].result_status=7;
  assert(fill()&&frame.count==24&&((frame.cars[3].flags&livewire::RESULT_MASK)>>8)==7);
  // Do not assume the safety car is the final slot, or admit unverified identity flags.
  f.live_cars[24].flags=SWOFTY_LIVE_CAR_RACER;f.live_cars[1].flags=0;f.live_cars[24].position=25;
  assert(fill()&&frame.count==24&&frame.cars[1].vehicle==2);
  f.live_cars[2].have&=~SWOFTY_LIVE_HAVE_IDENT;
  assert(fill()&&frame.count==23&&frame.cars[1].vehicle==3);
 }
 std::cout<<"PASS: Live State and CarOps routing, no legacy reads, display names, partial/malformed data and stale handles\n";
}
