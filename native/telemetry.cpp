// SPDX-License-Identifier: MIT
#define WIN32_LEAN_AND_MEAN
#define NOMINMAX
#include <windows.h>
#include <atomic>
#include <cstdio>
#include <cstring>
#include <initializer_list>
#include "swofty_api.h"
#include "readers.h"
#include "session.h"

namespace {
const SwoftyApi* api;
const SwoftyLive* live;
const SwoftyMem* memory;
HANDLE mapping;
livewire::Frame* shared;
livewire::Session session;
uintptr_t table;
bool direct;
std::atomic<uint64_t> lap_system{0};
uint64_t next_sample=0;
readers::TimingCache timing_cache[livewire::CARS]{};
uint64_t car_identity[livewire::CARS]{};

bool copy_memory(uint64_t at,void* out,size_t len) {
    if(!memory || !memory->readable(reinterpret_cast<const void*>(at),len)) return false;
    // Protect against an object/page being freed after the readability check.
    __try { std::memcpy(out,reinterpret_cast<const void*>(at),len); return true; }
    __except(EXCEPTION_EXECUTE_HANDLER) { return false; }
}
bool verified_layout() {
    auto read=copy_memory;
    const auto base=api->module_base(); IMAGE_DOS_HEADER dos{}; IMAGE_NT_HEADERS64 nt{};
    return read(base,&dos,sizeof dos) && dos.e_magic==IMAGE_DOS_SIGNATURE && dos.e_lfanew>0 && dos.e_lfanew<0x10000 &&
        read(base+dos.e_lfanew,&nt,sizeof nt) && nt.Signature==IMAGE_NT_SIGNATURE &&
        nt.FileHeader.Machine==IMAGE_FILE_MACHINE_AMD64 && nt.FileHeader.TimeDateStamp==1789118616u;
}
template<class T> const T* core_export(const char* name) {
    auto mod=GetModuleHandleW(L"swofty_core.dll");
    auto getter=mod?reinterpret_cast<const T*(*)()>(GetProcAddress(mod,name)):nullptr;
    return getter?getter():nullptr;
}
void publish() {
    if(!shared) return;
    session.stamp(GetTickCount64());
    InterlockedIncrement64(&shared->sequence);
    std::memcpy(shared,&session.frame,offsetof(livewire::Frame,sequence));
    std::memcpy(reinterpret_cast<char*>(shared)+24,reinterpret_cast<const char*>(&session.frame)+24,sizeof(*shared)-24);
    InterlockedIncrement64(&shared->sequence);
}
void on_event(const SwoftyEvent* event,void*) {
    if(!event||!event->name) return;
    session.event(event->name);
    if(!std::strcmp(event->name,"session.start")||session.ended) {
        lap_system.store(0);
        for(auto& cache:timing_cache) cache={};
        std::memset(car_identity,0,sizeof car_identity);
    }
    publish();
}
using GetLapFn=void*(void*,uint8_t);
GetLapFn* original_get;
void* get_lap(void* self,uint8_t car) {
    void* result=original_get(self,car);
    lap_system.store(reinterpret_cast<uint64_t>(self),std::memory_order_relaxed);
    return result;
}
using ReportFn=uint64_t(void*,void*);
ReportFn* original_report;
uint64_t report_lap(void* self,void* message) {
    auto result=original_report(self,message);
    lap_system.store(reinterpret_cast<uint64_t>(self),std::memory_order_relaxed);
    return result;
}
void on_frame(double,void*) {
    const auto now=GetTickCount64();
    if(now<next_sample) return;
    next_sample=now+200; // UI refreshes at 1 Hz; bounded 5 Hz native reads suffice.
    if(session.ended) { publish(); return; }
    SwoftyLiveSession info{};
    if(!live || live->version!=2 || !live->available() || live->session(&info)!=SWOFTY_ASSET_OK) {
        // Pauses/temporary missing snapshots retain the previous values, until explicit end.
        publish(); return;
    }
    auto read=copy_memory;
    livewire::Frame next{}; next.player=(info.have&SWOFTY_LIVE_SHAVE_PLAYER)?info.player:-1; next.session_type=info.type;
    next.state=livewire::API_READY|(direct?livewire::DIRECT_READY:0);
    if(lap_system.load()) next.state|=livewire::TIMING_READY;
    for(unsigned id=0;id<info.cars && id<livewire::CARS;id++) {
        SwoftyLiveCar raw{};
        if(live->car(id,&raw)!=SWOFTY_ASSET_OK || !(raw.flags&SWOFTY_LIVE_CAR_RACER)) continue;
        auto& c=next.cars[next.count++]; c.vehicle=id; c.flags=raw.flags;
        if(live->driver_name && live->driver_name(id,c.name,sizeof c.name)==SWOFTY_ASSET_OK && c.name[0]) c.known|=livewire::NAME;
        c.name[sizeof c.name-1]=0;
        if((raw.have&SWOFTY_LIVE_HAVE_CLASS) && raw.position>0) {c.position=raw.position;c.known|=livewire::POSITION;}
        if(raw.have&SWOFTY_LIVE_HAVE_TYRE) {c.compound=raw.tyre_actual;c.visual=raw.tyre_visual;c.known|=livewire::TYRES;}
        uint64_t car=direct?readers::pointer(read,table+8*id):0;
        // The pointer identifies a slot replacement; no history is retained for a reused slot.
        c.identity=car?car:(uint64_t(raw.participant)+1);
        if(car_identity[id]!=c.identity) {timing_cache[id]={};car_identity[id]=c.identity;}
        if(car) { readers::wear(read,car,c); readers::setup(read,car,c); }
        if(direct) readers::timing(read,lap_system.load(),id,c,&timing_cache[id]);
    }
    if(next.count) next.state|=livewire::ACTIVE;
    if(!next.count && (session.frame.state&livewire::ACTIVE)) {publish();return;}
    if(!direct) std::snprintf(next.diagnostic,sizeof next.diagnostic,"Wear, setup and lap timing require the supported Steam game build.");
    session.frame=next; publish();
}
void on_load() {
    live=core_export<SwoftyLive>("SwoftyCoreLive");
    memory=core_export<SwoftyMem>("SwoftyCoreMem");
    if(!memory || memory->version!=1) memory=nullptr;
    direct=memory && verified_layout();
    table=direct?api->resolve("g_Sess_VehicleTable"):0;
    direct=direct && table;
    wchar_t name[96]; swprintf_s(name,L"Local\\USTelemetryLive-1-%lu",GetCurrentProcessId());
    mapping=CreateFileMappingW(INVALID_HANDLE_VALUE,nullptr,PAGE_READWRITE,0,sizeof(livewire::Frame),name);
    shared=mapping?static_cast<livewire::Frame*>(MapViewOfFile(mapping,FILE_MAP_WRITE,0,0,sizeof(livewire::Frame))):nullptr;
    if(!shared) {if(mapping) CloseHandle(mapping);mapping=nullptr;api->log("[telemetry] shared snapshot could not be created");return;}
    if(!live || live->version!=2) std::snprintf(session.frame.diagnostic,sizeof session.frame.diagnostic,"The launcher core does not expose the supported live API v2.");
    // Event tokens belong to this DLL; Swofty removes them when unloading the module.
    uint32_t token=0;
    if(!api->events || !api->events->on_frame || !api->events->subscribe) {
        std::snprintf(session.frame.diagnostic,sizeof session.frame.diagnostic,"The launcher core does not expose session events.");publish();return;
    }
    bool events_ok=true;
    for(const char* event:{"session.start","session.end","session.unload","pause","resume"})
        events_ok &= api->events->subscribe(event,on_event,nullptr,&token)==SWOFTY_ASSET_OK;
    events_ok &= api->events->on_frame(on_frame,nullptr,&token)==SWOFTY_ASSET_OK;
    if(!events_ok) std::snprintf(session.frame.diagnostic,sizeof session.frame.diagnostic,"The launcher core could not activate live session callbacks.");
    publish();
    api->log("[telemetry] live display ready; API %s; direct readers %s; no recording or network receiver",live&&live->version==2?"v2":"unavailable",direct?"on":"off");
}
}

extern "C" __declspec(dllexport) const uint32_t SwoftyAddonApi=SWOFTY_API_VERSION;
extern "C" __declspec(dllexport) int SwoftyAddonInit(const SwoftyApi* value) {
    if(!value || value->version<SWOFTY_API_VERSION || !value->resolve || !value->register_mixins || !value->module_base) return 0;
    api=value;
    memory=core_export<SwoftyMem>("SwoftyCoreMem");
    if(!memory || memory->version!=1) memory=nullptr;
    // Named wrappers never alter arguments/results and are offered only when resolved.
    // Registration happens before core commit. On the known layout, declare the
    // hooks even while their code is encrypted: the core waits for decryption.
    SwoftyMixin load{nullptr,SWOFTY_MIXIN_ON_LOAD,0,reinterpret_cast<void*>(on_load),nullptr};
    if(!api->register_mixins(&load,1)) return 0;
    for(const auto& hook:{
        SwoftyMixin{"Sess_LapTimes_GetCarData",SWOFTY_MIXIN_WRAP,0,reinterpret_cast<void*>(get_lap),reinterpret_cast<void**>(&original_get)},
        SwoftyMixin{"Sess_LapTimes_ReportNewLapTime",SWOFTY_MIXIN_WRAP,0,reinterpret_cast<void*>(report_lap),reinterpret_cast<void**>(&original_report)}}) {
        if(memory && verified_layout() && !api->register_mixins(&hook,1)) return 0;
    }
    return 1;
}

BOOL WINAPI DllMain(HINSTANCE,DWORD reason,LPVOID) {
    if(reason==DLL_PROCESS_DETACH) {
        if(shared) {session.clear(true);publish();}
        if(shared) UnmapViewOfFile(shared);
        if(mapping) CloseHandle(mapping);
    }
    return TRUE;
}
