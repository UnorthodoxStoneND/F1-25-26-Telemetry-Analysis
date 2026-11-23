// SPDX-License-Identifier: MIT
// Compare only the native SDK fields and constants consumed by this add-on.
#include <cstddef>
#include <cstdio>
#include <type_traits>
#include "swofty_api.h"
#include "live_images.h"

static_assert(std::is_same_v<decltype(SwoftyApi::resolve),uintptr_t(*)(const char*)>);
static_assert(std::is_same_v<decltype(SwoftyApi::module_base),uintptr_t(*)()>);
static_assert(std::is_same_v<decltype(SwoftyApi::log),void(*)(const char*,...)>);
static_assert(std::is_same_v<decltype(SwoftyApi::register_mixins),int(*)(const SwoftyMixin*,size_t)>);
static_assert(std::is_same_v<decltype(SwoftyMem::readable),int(*)(const void*,size_t)>);
static_assert(std::is_same_v<SwoftyEventFn,void(*)(const SwoftyEvent*,void*)>);
static_assert(std::is_same_v<SwoftyFrameFn,void(*)(double,void*)>);
static_assert(std::is_same_v<decltype(SwoftyEvents::available),int(*)()>);
static_assert(std::is_same_v<decltype(SwoftyEvents::subscribe),int(*)(const char*,SwoftyEventFn,void*,uint32_t*)>);
static_assert(std::is_same_v<decltype(SwoftyEvents::on_frame),int(*)(SwoftyFrameFn,void*,uint32_t*)>);
static_assert(std::is_same_v<decltype(SwoftyCareer::available),int(*)()>);
static_assert(std::is_same_v<decltype(SwoftyCareer::flow_status),int(*)(uint32_t,uint32_t*,uint64_t*)>);
static_assert(std::is_same_v<decltype(SwoftyLive::available),int(*)()>);
static_assert(std::is_same_v<decltype(SwoftyLive::session),int(*)(SwoftyLiveSession*)>);
static_assert(std::is_same_v<decltype(SwoftyLive::car),int(*)(uint32_t,SwoftyLiveCar*)>);
static_assert(std::is_same_v<decltype(SwoftyLive::car_detail),int(*)(uint32_t,SwoftyLiveCarDetail*)>);
static_assert(std::is_same_v<decltype(SwoftyLive::seq),int(*)(uint64_t*)>);
static_assert(std::is_same_v<decltype(SwoftyLive::driver_name),int(*)(uint32_t,char*,uint32_t)>);
static_assert(std::is_same_v<decltype(SwoftyLive::team_badge),int(*)(uint32_t,char*,uint32_t)>);
static_assert(std::is_same_v<decltype(SwoftyCareerSaves::current),int(*)(char*,size_t,size_t*)>);
static_assert(std::is_same_v<decltype(SwoftyCareerOps::current_round),int(*)(int32_t*,int32_t*,int32_t*)>);
static_assert(std::is_same_v<decltype(SwoftyLive::team_badge_texture),int(*)(uint32_t,char*,uint32_t)>);
static_assert(std::is_same_v<decltype(SwoftyLive::tyre_icon_texture),int(*)(uint32_t,char*,uint32_t)>);
static_assert(std::is_same_v<decltype(SwoftyLive::images),const SwoftyLiveImages*(*)()>);
static_assert(std::is_same_v<decltype(SwoftyLiveImages::start),int(*)()>);
static_assert(std::is_same_v<decltype(SwoftyLiveImages::image),int(*)(const char*,uint8_t*,uint32_t,uint32_t*,uint32_t*,uint64_t*)>);
static_assert(std::is_same_v<decltype(SwoftyCarOps::available),int(*)()>);
static_assert(std::is_same_v<decltype(SwoftyCarOps::count),int(*)(uint32_t*)>);
static_assert(std::is_same_v<decltype(SwoftyCarOps::car),int(*)(uint32_t,void**,uint32_t*)>);
static_assert(std::is_same_v<decltype(SwoftyCarOps::get),int(*)(uint32_t,const void*,uint32_t,float*)>);
static_assert(std::is_same_v<decltype(SwoftyHooks::attach),int(*)(const char*,uint32_t,int32_t,SwoftyHookFn,void*,uint32_t*)>);
static_assert(std::is_same_v<decltype(SwoftyHooks::context_in),int(*)(const void*,SwoftyText,int*)>);
static_assert(std::is_same_v<SwoftyHookFn,int(*)(void*,SwoftyHookCall*)>);

#define FIELD(t,f) std::printf(",\"" #t "." #f "\":[%zu,%zu]",offsetof(t,f),sizeof(((t*)nullptr)->f))
#define VALUE(n) std::printf(",\"" #n "\":%llu",static_cast<unsigned long long>(n))
int main() {
    std::printf("{\"api_version\":%u,\"mixin_size\":%zu",SWOFTY_API_VERSION,sizeof(SwoftyMixin));
    FIELD(SwoftyApi,version);FIELD(SwoftyApi,log);FIELD(SwoftyApi,module_base);
    FIELD(SwoftyApi,resolve);FIELD(SwoftyApi,register_mixins);FIELD(SwoftyApi,events);FIELD(SwoftyApi,career);
    FIELD(SwoftyApi,hooks);FIELD(SwoftyHooks,version);FIELD(SwoftyHooks,available);FIELD(SwoftyHooks,attach);FIELD(SwoftyHooks,context_in);
    FIELD(SwoftyHookCall,context);FIELD(SwoftyText,data);FIELD(SwoftyText,size);
    FIELD(SwoftyMixin,target);FIELD(SwoftyMixin,kind);FIELD(SwoftyMixin,priority);FIELD(SwoftyMixin,handler);FIELD(SwoftyMixin,orig);
    FIELD(SwoftyEvent,a);FIELD(SwoftyEvent,name);FIELD(SwoftyEvents,available);FIELD(SwoftyEvents,subscribe);FIELD(SwoftyEvents,on_frame);
    FIELD(SwoftyCareer,available);FIELD(SwoftyCareer,flow_status);
    FIELD(SwoftyCareerSaves,version);FIELD(SwoftyCareerSaves,current);
    FIELD(SwoftyCareerOps,version);FIELD(SwoftyCareerOps,current_round);
    FIELD(SwoftyMem,version);FIELD(SwoftyMem,readable);
    FIELD(SwoftyLive,version);FIELD(SwoftyLive,available);FIELD(SwoftyLive,session);FIELD(SwoftyLive,car);FIELD(SwoftyLive,seq);
    FIELD(SwoftyLive,driver_name);
    FIELD(SwoftyLive,car_detail);FIELD(SwoftyLive,team_badge);
    FIELD(SwoftyLiveCarDetail,version);FIELD(SwoftyLiveCarDetail,size);FIELD(SwoftyLiveCarDetail,vehicle);FIELD(SwoftyLiveCarDetail,have);
    FIELD(SwoftyLiveCarDetail,grid_position);FIELD(SwoftyLiveCarDetail,tyre_age_laps);FIELD(SwoftyLiveCarDetail,tyre_wear);
    FIELD(SwoftyLiveCarDetail,laps_completed);FIELD(SwoftyLiveCarDetail,last_lap_ms);FIELD(SwoftyLiveCarDetail,last_sector_ms);
    FIELD(SwoftyLiveCarDetail,current_lap_ms);FIELD(SwoftyLiveCarDetail,current_sector_ms);
    FIELD(SwoftyLiveCarDetail,lap_flags);FIELD(SwoftyLiveCarDetail,tyre_stint_laps);FIELD(SwoftyLiveCarDetail,best_lap_number);
    FIELD(SwoftyLiveCarDetail,best_lap_ms);FIELD(SwoftyLiveCarDetail,best_sector_ms);
    VALUE(SWOFTY_LIVE_HAVE_GRID);VALUE(SWOFTY_LIVE_HAVE_TYRE_AGE);VALUE(SWOFTY_LIVE_HAVE_TYRE_WEAR);VALUE(SWOFTY_LIVE_HAVE_LAP_TIMES);
    FIELD(SwoftyLive,team_badge_texture);FIELD(SwoftyLive,images);FIELD(SwoftyLive,tyre_icon_texture);
    FIELD(SwoftyLiveImages,version);FIELD(SwoftyLiveImages,start);FIELD(SwoftyLiveImages,image);
    FIELD(SwoftyCarOps,version);FIELD(SwoftyCarOps,available);FIELD(SwoftyCarOps,count);FIELD(SwoftyCarOps,car);FIELD(SwoftyCarOps,get);
    VALUE(SWOFTY_CAR_FUEL_MASS);VALUE(SWOFTY_CAR_BRAKES_BIAS);VALUE(SWOFTY_CAR_BRAKES_PRESSURE);
    FIELD(SwoftyLiveSession,version);FIELD(SwoftyLiveSession,size);FIELD(SwoftyLiveSession,have);FIELD(SwoftyLiveSession,cars);FIELD(SwoftyLiveSession,player);
    FIELD(SwoftyLiveSession,type);
    FIELD(SwoftyLiveSession,track_name);FIELD(SwoftyLiveSession,safety_car);FIELD(SwoftyLiveSession,chequered);
    VALUE(SWOFTY_LIVE_SHAVE_TRACK_NAME);VALUE(SWOFTY_LIVE_SHAVE_SAFETY_CAR);VALUE(SWOFTY_LIVE_LAP_LAST_VALID);
    FIELD(SwoftyLiveCar,version);FIELD(SwoftyLiveCar,size);FIELD(SwoftyLiveCar,have);FIELD(SwoftyLiveCar,vehicle);
    FIELD(SwoftyLiveCar,position);FIELD(SwoftyLiveCar,tyre_actual);FIELD(SwoftyLiveCar,tyre_visual);FIELD(SwoftyLiveCar,pit_status);
    FIELD(SwoftyLiveCar,result_status);FIELD(SwoftyLiveCar,lap_m);FIELD(SwoftyLiveCar,pit_stops);
    FIELD(SwoftyLiveCar,speed);FIELD(SwoftyLiveCar,throttle);FIELD(SwoftyLiveCar,brake);
    FIELD(SwoftyLiveCar,gear);FIELD(SwoftyLiveCar,rpm);FIELD(SwoftyLiveCar,gap_leader_ms);
    VALUE(SWOFTY_LIVE_HAVE_SPEED);VALUE(SWOFTY_LIVE_HAVE_PEDALS);VALUE(SWOFTY_LIVE_HAVE_GEAR);
    VALUE(SWOFTY_LIVE_HAVE_RPM);VALUE(SWOFTY_LIVE_HAVE_GAP_LEADER);VALUE(SWOFTY_LIVE_HAVE_PIT);
    FIELD(SwoftyLiveCar,flags);VALUE(SWOFTY_LIVE_HAVE_IDENT);VALUE(SWOFTY_LIVE_CAR_RACER);
    FIELD(SwoftyLiveCar,lap);VALUE(SWOFTY_LIVE_HAVE_PROGRESS);
    FIELD(SwoftyLiveCar,gap_ahead_ms);VALUE(SWOFTY_LIVE_HAVE_GAP_AHEAD);
    VALUE(SWOFTY_ASSET_OK);VALUE(SWOFTY_MIXIN_ON_LOAD);
    VALUE(SWOFTY_LIVE_SHAVE_CARS);VALUE(SWOFTY_LIVE_SHAVE_PLAYER);
    VALUE(SWOFTY_LIVE_SHAVE_TYPE);VALUE(SWOFTY_PHASE_AFTER);
    VALUE(SWOFTY_LIVE_HAVE_CLASS);VALUE(SWOFTY_LIVE_HAVE_TYRE);VALUE(SWOFTY_LIVE_HAVE_PIT_LANE);
    std::puts("}");
}
