// SPDX-License-Identifier: MIT
// Compare only the native SDK fields and constants consumed by this add-on.
#include <cstddef>
#include <cstdio>
#include <type_traits>
#include "swofty_api.h"

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
static_assert(std::is_same_v<decltype(SwoftyLive::seq),int(*)(uint64_t*)>);

#define FIELD(t,f) std::printf(",\"" #t "." #f "\":[%zu,%zu]",offsetof(t,f),sizeof(((t*)nullptr)->f))
#define VALUE(n) std::printf(",\"" #n "\":%llu",static_cast<unsigned long long>(n))
int main() {
    std::printf("{\"api_version\":%u,\"mixin_size\":%zu",SWOFTY_API_VERSION,sizeof(SwoftyMixin));
    FIELD(SwoftyApi,version);FIELD(SwoftyApi,log);FIELD(SwoftyApi,module_base);
    FIELD(SwoftyApi,resolve);FIELD(SwoftyApi,register_mixins);FIELD(SwoftyApi,events);FIELD(SwoftyApi,career);
    FIELD(SwoftyMixin,target);FIELD(SwoftyMixin,kind);FIELD(SwoftyMixin,priority);FIELD(SwoftyMixin,handler);FIELD(SwoftyMixin,orig);
    FIELD(SwoftyEvent,name);FIELD(SwoftyEvents,available);FIELD(SwoftyEvents,subscribe);FIELD(SwoftyEvents,on_frame);
    FIELD(SwoftyCareer,available);FIELD(SwoftyCareer,flow_status);
    FIELD(SwoftyMem,version);FIELD(SwoftyMem,readable);
    FIELD(SwoftyLive,version);FIELD(SwoftyLive,available);FIELD(SwoftyLive,session);FIELD(SwoftyLive,car);FIELD(SwoftyLive,seq);
    FIELD(SwoftyLiveSession,version);FIELD(SwoftyLiveSession,size);FIELD(SwoftyLiveSession,have);FIELD(SwoftyLiveSession,cars);FIELD(SwoftyLiveSession,player);
    FIELD(SwoftyLiveCar,version);FIELD(SwoftyLiveCar,size);FIELD(SwoftyLiveCar,have);FIELD(SwoftyLiveCar,vehicle);
    FIELD(SwoftyLiveCar,position);FIELD(SwoftyLiveCar,tyre_actual);FIELD(SwoftyLiveCar,tyre_visual);FIELD(SwoftyLiveCar,pit_status);
    VALUE(SWOFTY_ASSET_OK);VALUE(SWOFTY_MIXIN_ON_LOAD);VALUE(SWOFTY_MIXIN_WRAP);
    VALUE(SWOFTY_LIVE_SHAVE_CARS);VALUE(SWOFTY_LIVE_SHAVE_PLAYER);
    VALUE(SWOFTY_LIVE_HAVE_CLASS);VALUE(SWOFTY_LIVE_HAVE_TYRE);VALUE(SWOFTY_LIVE_HAVE_PIT_LANE);
    std::puts("}");
}
