// SPDX-License-Identifier: MIT
#define WIN32_LEAN_AND_MEAN
#define NOMINMAX
#include <windows.h>
#include <cstdio>
#include <cstring>
#include <initializer_list>
#include "swofty_api.h"
#include "readers.h"
#include "session.h"
#include "builds.h"
#include "direct_live.h"
#include "session_type.h"
#include "session_metrics.h"
#include "badges.h"
#include "proton_transport.h"
#include "outlap.h"

namespace {
const SwoftyApi* api;

const SwoftyMem* memory;
const SwoftyLive* live;
const SwoftyCarOps* car_ops;
HANDLE mapping;
livewire::Frame* shared;
HANDLE badge_mapping;
badgewire::Frame* badge_shared;
badges::Publisher badge_publisher;
protontransport::Mapping proton_mapping;
livewire::Session session;
bool direct;
char binding_error[192]{};
uint32_t game_stamp=0;
directlive::Sites sites;
directlive::Context direct_context;
outlap::Tracker outlaps;
sessionmetrics::Sites metric_sites;
uint64_t next_sample=0;
uint32_t context_kind=livewire::SESSION_UNKNOWN;

bool copy_memory(uint64_t at,void* out,size_t len) {
    if(!memory || !memory->readable(reinterpret_cast<const void*>(at),len)) return false;
    // Protect against an object/page being freed after the readability check.
    __try { std::memcpy(out,reinterpret_cast<const void*>(at),len); return true; }
    __except(EXCEPTION_EXECUTE_HANDLER) { return false; }
}
bool time_remaining(uint64_t fn,uint64_t rules,float& seconds) {
    // Verified getter, called on the game's flow thread; never set its clock.
    __try { seconds=reinterpret_cast<float(*)(void*)>(fn)(reinterpret_cast<void*>(rules));return true; }
    __except(EXCEPTION_EXECUTE_HANDLER) { return false; }
}
bool verified_layout() {
    auto read=copy_memory;
    const auto base=api->module_base(); IMAGE_DOS_HEADER dos{}; IMAGE_NT_HEADERS64 nt{};
    if(!read(base,&dos,sizeof dos)||dos.e_magic!=IMAGE_DOS_SIGNATURE||dos.e_lfanew<=0||dos.e_lfanew>=0x10000||
       !read(base+dos.e_lfanew,&nt,sizeof nt)||nt.Signature!=IMAGE_NT_SIGNATURE||nt.FileHeader.Machine!=IMAGE_FILE_MACHINE_AMD64) return false;
    game_stamp=nt.FileHeader.TimeDateStamp;
    return builds::supported(game_stamp);
}
uint64_t resolve_binding(const char* name) {
    auto address=api->resolve(name);
    if(!address) {
        api->log("[telemetry] binding unavailable: %s",name);
        if(!binding_error[0]) std::snprintf(binding_error,sizeof binding_error,"Game binding unavailable: %s.",name);
    }
    return address;
}
template<class T> const T* core_export(const char* name) {
    auto mod=GetModuleHandleW(L"swofty_core.dll");
    auto getter=mod?reinterpret_cast<const T*(*)()>(GetProcAddress(mod,name)):nullptr;
    return getter?getter():nullptr;
}
void publish() {
    if(!shared) return;
    session.stamp(GetTickCount64());
    // Prepare privately while readers can still use the previous complete view.
    if(badge_shared) badge_publisher.sample(live,session.frame,api?api->log:nullptr);
    auto publish_pair=[&](livewire::Frame* target,badgewire::Frame* image_target) {
    if(!target) return;
    const auto committed=InterlockedIncrement64(&target->sequence)+1;
    if(image_target) InterlockedExchange64(&image_target->sequence,committed-1);
    std::memcpy(target,&session.frame,offsetof(livewire::Frame,sequence));
    std::memcpy(reinterpret_cast<char*>(target)+24,reinterpret_cast<const char*>(&session.frame)+24,sizeof(*target)-24);
    if(image_target) {
        const auto& frame=badge_publisher.frame;
        std::memcpy(image_target,&frame,offsetof(badgewire::Frame,sequence));
        std::memcpy(reinterpret_cast<char*>(image_target)+24,reinterpret_cast<const char*>(&frame)+24,sizeof(frame)-24);
        InterlockedExchange64(&image_target->sequence,committed);
    }
    // The primary sequence commits the telemetry and optional badges together.
    InterlockedExchange64(&target->sequence,committed);
    };
    publish_pair(shared,badge_shared);
    if(proton_mapping.view) publish_pair(static_cast<livewire::Frame*>(proton_mapping.view),
        reinterpret_cast<badgewire::Frame*>(static_cast<char*>(proton_mapping.view)+sizeof(livewire::Frame)));
}

void on_event(const SwoftyEvent* event,void*) {
    if(!event||!event->name) return;
    if(!std::strcmp(event->name,"flow.state")) {
        const bool ready=session.gameplay;
        if(event->data) {
            const auto& flow=*static_cast<const SwoftyFlowState*>(event->data);
            session.flow(flow.hash,flow.to);
        }
        if(ready!=session.gameplay) publish();
        return;
    }
    session.event(event->name);
    if(!std::strcmp(event->name,"session.start")||session.ended) {
        context_kind=livewire::SESSION_UNKNOWN;
    }
    publish();
}
int on_session_context(void*,SwoftyHookCall* call) {
    if(call&&session.entered&&!session.ended)
        context_kind=sessioninfo::kind(api->hooks,call->context);
    return 0;
}
// Same read-only wrapper ABI as the released core's ui.cpp open_hook. Do not
// use SessionHost.on_screen: it is a single listener owned by other core clients.
using OpenScreenFn=uint64_t(void*,void*,void*,void*);
OpenScreenFn* original_screen;
uint64_t open_screen(void* scene,void* screen,void* a3,void* a4) {
    const auto result=original_screen(scene,screen,a3,a4);
    if(session.opened_screen(copy_memory,reinterpret_cast<uint64_t>(screen))) publish();
    return result;
}
void on_frame(double,void*) {
    const auto now=GetTickCount64();
    if(now<next_sample) return;
    next_sample=now+200; // UI refreshes at 1 Hz; bounded 5 Hz native reads suffice.
    badge_publisher.prepare(live);
    if(!session.permitted) badge_publisher.prefetch(live);
    if(session.ended) { publish(); return; }
    if(!direct) {
        std::snprintf(session.frame.diagnostic,sizeof session.frame.diagnostic,
            "Live data unavailable: no supported game-binding layout for build %u.",game_stamp);
        if(binding_error[0]) std::snprintf(session.frame.diagnostic,sizeof session.frame.diagnostic,"%s",binding_error);
        publish();return;
    }
    uint32_t status=0;uint64_t age=0;
    auto hash=[](const char* text){uint32_t h=5381;for(;*text;text++)h=h*33+uint8_t(*text);return h;};
    auto flow=api->career && api->career->flow_status?api->career->flow_status:nullptr;
    int flow_result=flow?flow(hash("StateSession"),&status,&age):SWOFTY_ASSET_UNAVAILABLE;
    bool in_session=flow_result==SWOFTY_ASSET_OK && (status==1||status==2),blocked=false;
    for(const char* name:{"StateLoadSession","StateUnloadSession","SessionStateCareerHubUpdate"}) {
        status=0;
        if(flow && flow(hash(name),&status,&age)==SWOFTY_ASSET_OK && status!=0) blocked=true;
    }
    if(!session.allow(in_session,blocked)) {
        session.frame.diagnostic[0]=0;
        if(flow_result==SWOFTY_ASSET_UNAVAILABLE)
            std::snprintf(session.frame.diagnostic,sizeof session.frame.diagnostic,"Swofty's session-state API is unavailable.");
        publish();return;
    }
    livewire::Frame next{};auto read=copy_memory;
    if(direct_context.fill(read,sites,next,live,car_ops)) {
        next.session_kind=sessioninfo::compatible_kind(next.session_type,context_kind);
        sessionmetrics::fill(read,metric_sites,direct_context.registry,next,live,time_remaining);
        outlaps.apply(next,session.serial);
        session.frame=next;
    } else if(!(session.frame.state&livewire::ACTIVE)) {
        session.frame=next;
    } else {
        session.frame.state|=64; // No fresh values; retain the last display until explicit end.
        std::snprintf(session.frame.diagnostic,sizeof session.frame.diagnostic,"%s",next.diagnostic);
    }
    publish();
}
void on_load() {
    live=core_export<SwoftyLive>("SwoftyCoreLive");
    badge_publisher.prepare(live);
    car_ops=core_export<SwoftyCarOps>("SwoftyCoreCarOps");
    memory=core_export<SwoftyMem>("SwoftyCoreMem");
    if(!memory || memory->version!=1) memory=nullptr;
    direct=memory && verified_layout();
    if(direct) {
        sites.manager=resolve_binding("g_Game_Manager");
        metric_sites.red_key=api->resolve("k_Live_RedFlagSystemKey");
        metric_sites.red_vt=api->resolve("vt_Sess_RedFlagSystem");
        metric_sites.race_vt=api->resolve("vt_Sess_RaceRuleset");
        metric_sites.remaining=api->resolve("Sess_RaceRuleset_GetTimeRemaining");
    }
    if(!memory) std::snprintf(binding_error,sizeof binding_error,"Swofty's memory-check API is unavailable.");
    if(protontransport::under_wine()) {
        if(!protontransport::create_for_module(proton_mapping,reinterpret_cast<const void*>(&on_load))) {
            api->log("[telemetry] Proton shared IPC mapping could not be created");
        }
        if(proton_mapping.view) api->log("[telemetry] Proton temporary shared IPC transport ready");
    }
    wchar_t name[96]; swprintf_s(name,L"Local\\USTelemetryLive-2-%lu",GetCurrentProcessId());
    mapping=CreateFileMappingW(INVALID_HANDLE_VALUE,nullptr,PAGE_READWRITE,0,sizeof(livewire::Frame),name);
    shared=mapping?static_cast<livewire::Frame*>(MapViewOfFile(mapping,FILE_MAP_WRITE,0,0,sizeof(livewire::Frame))):nullptr;
    if(!shared) {if(mapping) CloseHandle(mapping);mapping=nullptr;api->log("[telemetry] shared snapshot could not be created");return;}
    swprintf_s(name,L"Local\\USTelemetryBadges-2-%lu",GetCurrentProcessId());
    badge_mapping=CreateFileMappingW(INVALID_HANDLE_VALUE,nullptr,PAGE_READWRITE,0,sizeof(badgewire::Frame),name);
    badge_shared=badge_mapping?static_cast<badgewire::Frame*>(MapViewOfFile(badge_mapping,FILE_MAP_WRITE,0,0,sizeof(badgewire::Frame))):nullptr;
    if(!badge_shared&&badge_mapping) {CloseHandle(badge_mapping);badge_mapping=nullptr;}

    // Event tokens belong to this DLL; Swofty removes them when unloading the module.
    uint32_t token=0;
    if(!api->events || !api->events->on_frame || !api->events->subscribe) {
        std::snprintf(session.frame.diagnostic,sizeof session.frame.diagnostic,"The launcher core does not expose session events.");publish();return;
    }
    bool events_ok=true;
    for(const char* event:{"session.start","session.end","session.unload","pause","resume","flow.state"})
        events_ok &= api->events->subscribe(event,on_event,nullptr,&token)==SWOFTY_ASSET_OK;
    if(api->hooks&&api->hooks->version>=1&&api->hooks->available&&api->hooks->available()&&
       api->hooks->attach&&api->hooks->context_in) {
        for(const char* point:{"session.start","flow.state"})
            if(api->hooks->attach(point,SWOFTY_PHASE_AFTER,0,on_session_context,nullptr,&token)!=SWOFTY_ASSET_OK)
                api->log("[telemetry] session context hook unavailable: %s",point);
    }
    events_ok &= api->events->on_frame(on_frame,nullptr,&token)==SWOFTY_ASSET_OK;
    if(!events_ok) std::snprintf(session.frame.diagnostic,sizeof session.frame.diagnostic,"The launcher core could not activate live session callbacks.");
    publish();
    api->log("[telemetry] live display ready; build %u; binding readers %s; no recording or network receiver",game_stamp,direct?"on":"off");
}
}

extern "C" __declspec(dllexport) const uint32_t SwoftyAddonApi=SWOFTY_API_VERSION;
extern "C" __declspec(dllexport) int SwoftyAddonInit(const SwoftyApi* value) {
    if(!value || value->version<SWOFTY_API_VERSION || !value->resolve || !value->register_mixins || !value->module_base) return 0;
    api=value;
    // Do not wait for on_load/hook settlement: the menus may render badges first.
    live=core_export<SwoftyLive>("SwoftyCoreLive");
    badge_publisher.prepare(live);
    memory=core_export<SwoftyMem>("SwoftyCoreMem");
    if(!memory || memory->version!=1) memory=nullptr;
    // Named wrappers never alter arguments/results and are offered only when resolved.
    // Registration happens before core commit. On the known layout, declare the
    // hooks even while their code is encrypted: the core waits for decryption.
    SwoftyMixin load{nullptr,SWOFTY_MIXIN_ON_LOAD,0,reinterpret_cast<void*>(on_load),nullptr};
    if(!api->register_mixins(&load,1)) return 0;
    // Screen-open is optional: the running core's exported binding list may
    // omit this internal UI target. Declaring an absent target drops ALL of
    // this add-on's mixins during core commit, including its publisher.
    if(memory && verified_layout()) {
        if(api->resolve("Vm_Open_Screen")) {
            SwoftyMixin screen_hook{"Vm_Open_Screen",SWOFTY_MIXIN_WRAP,0,reinterpret_cast<void*>(open_screen),reinterpret_cast<void**>(&original_screen)};
            if(!api->register_mixins(&screen_hook,1))
                api->log("[telemetry] optional career-rewards screen hook registration unavailable; telemetry remains enabled");
        } else {
            api->log("[telemetry] optional career-rewards screen hook unavailable (Vm_Open_Screen); using flow/session visibility signals");
        }
    }
    return 1;
}

BOOL WINAPI DllMain(HINSTANCE,DWORD reason,LPVOID) {
    if(reason==DLL_PROCESS_DETACH) {
        if(shared) {session.clear(true);publish();}
        if(shared) UnmapViewOfFile(shared);
        if(mapping) CloseHandle(mapping);
        if(badge_shared) UnmapViewOfFile(badge_shared);
        if(badge_mapping) CloseHandle(badge_mapping);
    }
    return TRUE;
}
