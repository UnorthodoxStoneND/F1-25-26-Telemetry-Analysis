// SPDX-License-Identifier: MIT
// Host-test core only. Never loaded into the game or included as a runtime DLL.
#include "swofty_api.h"
#include <cstdio>
namespace {
int available(){return 1;}
int session(SwoftyLiveSession* s){*s={};s->version=2;s->size=sizeof(*s);s->have=SWOFTY_LIVE_SHAVE_PLAYER|SWOFTY_LIVE_SHAVE_CARS;s->cars=24;s->racers=24;s->player=12;s->type=15;return SWOFTY_ASSET_OK;}
int car(uint32_t id,SwoftyLiveCar* c){if(id>=24)return SWOFTY_ASSET_UNAVAILABLE;*c={};c->version=2;c->size=sizeof(*c);c->vehicle=id;c->flags=SWOFTY_LIVE_CAR_RACER;c->have=SWOFTY_LIVE_HAVE_CLASS|SWOFTY_LIVE_HAVE_TYRE;c->position=int32_t(id+1);c->tyre_actual=18;c->tyre_visual=17;return SWOFTY_ASSET_OK;}
int name(uint32_t id,char* out,uint32_t cap){std::snprintf(out,cap,"Mock Driver %u",id+1);return SWOFTY_ASSET_OK;}
SwoftyLive live={2,available,session,car,nullptr,nullptr,nullptr,nullptr,nullptr,name};
int readable(const void* p,size_t n){return p && n ? 1:0;}
SwoftyMem memory={1,readable,nullptr};
}
extern "C" __declspec(dllexport) const SwoftyLive* SwoftyCoreLive(){return &live;}
extern "C" __declspec(dllexport) const SwoftyMem* SwoftyCoreMem(){return &memory;}
