// SPDX-License-Identifier: MIT
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include "swofty_api.h"
#include "live_wire.h"
#include "badge_wire.h"
#include "live_images.h"
#include <cassert>
#include <cstdarg>
#include <cstring>
#include <iostream>
#include <map>
#include <string>
#include <vector>
#include "binding_fixture.h"
#include "builds.h"
namespace {
void(*load)();SwoftyFrameFn frame;
std::map<std::string,SwoftyEventFn> subscribers;
std::map<std::string,SwoftyHookFn> context_subscribers;
uint32_t current_kind=livewire::PRACTICE;
bool context_available=true;
bool verified=false;
uint32_t session_status=0,hub_status=0,load_status=0,unload_status=0;
unsigned char pe[4096]{};
std::unique_ptr<BindingFixture> fixture;
void*(*get_lap)(void*,uint8_t);
uint64_t(*report_lap)(void*,void*);
void* get_original(void* self,uint8_t id){assert(self && id==12);return reinterpret_cast<void*>(0x12345678);}
uint64_t report_original(void* self,void* message){assert(self && message==reinterpret_cast<void*>(0x1234));return 0xdeadbeef12345678ull;}
void log(const char*,...){}
uintptr_t base(){return verified?reinterpret_cast<uintptr_t>(pe):reinterpret_cast<uintptr_t>(GetModuleHandleW(nullptr));}
uintptr_t resolve(const char* name){return fixture?fixture->resolve(name):0;}
int mixins(const SwoftyMixin* list,size_t count){for(size_t i=0;i<count;i++) {
 if(list[i].kind==SWOFTY_MIXIN_ON_LOAD)load=reinterpret_cast<void(*)()>(list[i].handler);
 else if(!std::strcmp(list[i].target,"Sess_LapTimes_GetCarData")){get_lap=reinterpret_cast<decltype(get_lap)>(list[i].handler);*list[i].orig=reinterpret_cast<void*>(get_original);}
 else if(!std::strcmp(list[i].target,"Sess_LapTimes_ReportNewLapTime")){report_lap=reinterpret_cast<decltype(report_lap)>(list[i].handler);*list[i].orig=reinterpret_cast<void*>(report_original);}
 else assert(false);
 }return 1;}
int available(){return 1;}
int badge_reference(uint32_t id,char* out,uint32_t cap){
 return std::snprintf(out,cap,"eaid://ui_package/art/textures/render_badge_eUserDefined %u",id/2)>0?SWOFTY_ASSET_OK:SWOFTY_ASSET_INVALID;
}
int badge_image(const char*,uint8_t* out,uint32_t cap,uint32_t* w,uint32_t* h,uint64_t* revision){
 assert(cap==badgewire::PIXELS);const uint8_t pixels[]={255,0,0,64,0,255,0,255};
 std::memcpy(out,pixels,sizeof pixels);*w=2;*h=1;*revision=7;return SWOFTY_ASSET_OK;
}
unsigned badge_starts=0;
int start_images(){badge_starts++;return SWOFTY_ASSET_OK;}
SwoftyLiveImages badge_images{1,start_images,nullptr,badge_image,nullptr};
const SwoftyLiveImages* image_service(){return &badge_images;}
uint32_t hash(const char* p){uint32_t h=5381;for(;*p;p++)h=h*33+uint8_t(*p);return h;}
int flow_status(uint32_t key,uint32_t* status,uint64_t* age){
 *age=0;
 if(key==hash("StateSession"))*status=session_status;
 else if(key==hash("SessionStateCareerHubUpdate"))*status=hub_status;
 else if(key==hash("StateLoadSession"))*status=load_status;
 else if(key==hash("StateUnloadSession"))*status=unload_status;
 else return SWOFTY_ASSET_INVALID;
 return SWOFTY_ASSET_OK;
}
int subscribe(const char* event,SwoftyEventFn fn,void*,uint32_t* token){subscribers[event]=fn;*token=1;return SWOFTY_ASSET_OK;}
int on_frame(SwoftyFrameFn fn,void*,uint32_t* token){frame=fn;*token=2;return SWOFTY_ASSET_OK;}
int attach(const char* point,uint32_t phase,int32_t,SwoftyHookFn fn,void*,uint32_t* token) {
 assert(phase==SWOFTY_PHASE_AFTER);context_subscribers[point]=fn;*token=3;return SWOFTY_ASSET_OK;
}
int context_in(const void* context,SwoftyText selector,int* matches) {
 assert(context==&current_kind);
 if(!context_available)return SWOFTY_ASSET_UNAVAILABLE;
 const char* families[]={"", "practice", "qualifying", "race", "sprint", "time_trial"};
 *matches=current_kind>0&&current_kind<=5&&std::string(selector.data,selector.size)==
  std::string("[{\"session_type\":[\"")+families[current_kind]+"\"]}]";
 return SWOFTY_ASSET_OK;
}
void emit_context(const char* point) {
 assert(context_subscribers.count(point));SwoftyHookCall call{};call.context=&current_kind;
 context_subscribers[point](nullptr,&call);
}
void emit(const char* name){SwoftyEvent e{};e.name=name;assert(subscribers.count(name));subscribers[name](&e,nullptr);
 if(context_subscribers.count(name))emit_context(name);
}
}
int main(int argc,char** argv){
    assert(argc==3||argc==4);verified=argc==4;
    auto dos=reinterpret_cast<IMAGE_DOS_HEADER*>(pe);dos->e_magic=IMAGE_DOS_SIGNATURE;dos->e_lfanew=0x80;
    auto nt=reinterpret_cast<IMAGE_NT_HEADERS64*>(pe+0x80);nt->Signature=IMAGE_NT_SIGNATURE;nt->FileHeader.Machine=IMAGE_FILE_MACHINE_AMD64;nt->FileHeader.TimeDateStamp=argc==4&&!std::strcmp(argv[3],"current")?builds::CURRENT_STEAM:builds::ORIGINAL_STEAM;
    auto core=LoadLibraryA(argv[1]);assert(core);
    assert(GetProcAddress(core,"SwoftyCoreLive")&&GetProcAddress(core,"SwoftyCoreCarOps"));
    if(verified) {
        fixture=std::make_unique<BindingFixture>(24);
        fixture->live.version=4;fixture->live.team_badge_texture=badge_reference;fixture->live.images=image_service;
        fixture->live_enabled=false; // Runtime capture must start before Live State becomes available.
        reinterpret_cast<void(*)(const SwoftyLive*)>(GetProcAddress(core,"TelemetryTestSetLive"))(&fixture->live);
        reinterpret_cast<void(*)(const SwoftyCarOps*)>(GetProcAddress(core,"TelemetryTestSetCarOps"))(&fixture->ops);
    }
    auto dll=LoadLibraryA(argv[2]);assert(dll);
    auto init=reinterpret_cast<SwoftyAddonInitFn>(GetProcAddress(dll,"SwoftyAddonInit"));assert(init);
    SwoftyEvents events={available,subscribe,on_frame,nullptr};SwoftyApi api{};
    api.version=9;api.module_base=base;api.resolve=resolve;api.register_mixins=mixins;api.events=&events;api.log=log;
    SwoftyHooks hooks{};hooks.version=1;hooks.available=available;hooks.attach=attach;hooks.context_in=context_in;api.hooks=&hooks;
    SwoftyCareer career{};career.available=available;career.flow_status=flow_status;api.career=&career;
    assert(!init(nullptr));assert(init(&api));assert(load);
    if(verified) assert(badge_starts==1); // Capture precedes hook settlement/on_load.
    load();assert(frame);
    if(verified) assert(badge_starts==1);
    std::vector<unsigned char> system(0x100);uint64_t entries[32]{};unsigned char records[32][512]{};
    auto write=[](void* to,const auto& value){std::memcpy(to,&value,sizeof value);};
    if(verified) {
        assert(get_lap && report_lap);write(system.data()+0x58,reinterpret_cast<uint64_t>(entries));write(system.data()+0x60,reinterpret_cast<uint64_t>(entries)+32*8);
        for(unsigned i=0;i<32;i++){entries[i]=reinterpret_cast<uint64_t>(records[i]);float time=80.f+float(i);write(records[i]+0xe8,time);}
        assert(get_lap(system.data(),12)==reinterpret_cast<void*>(0x12345678));
        assert(report_lap(system.data(),reinterpret_cast<void*>(0x1234))==0xdeadbeef12345678ull);
    } else assert(!get_lap&&!report_lap);
    wchar_t name[96];swprintf_s(name,L"Local\\USTelemetryLive-2-%lu",GetCurrentProcessId());
    auto handle=OpenFileMappingW(FILE_MAP_READ,FALSE,name);assert(handle);
    auto view=static_cast<const livewire::Frame*>(MapViewOfFile(handle,FILE_MAP_READ,0,0,sizeof(livewire::Frame)));assert(view);
    swprintf_s(name,L"Local\\USTelemetryBadges-1-%lu",GetCurrentProcessId());
    auto badge_handle=OpenFileMappingW(FILE_MAP_READ,FALSE,name);assert(badge_handle);
    auto badge_view=static_cast<const badgewire::Frame*>(MapViewOfFile(badge_handle,FILE_MAP_READ,0,0,sizeof(badgewire::Frame)));assert(badge_view);
    assert(badge_view->size==sizeof(badgewire::Frame)&&!badge_view->cars[0].width);
    frame(.2,nullptr);assert(view->size==9984 && !(view->sequence&1));
    if(!verified) {assert(!view->count && view->diagnostic[0]);UnmapViewOfFile(badge_view);CloseHandle(badge_handle);UnmapViewOfFile(view);CloseHandle(handle);FreeLibrary(dll);FreeLibrary(core);std::cout<<"PASS: unsupported build is reported instead of an indefinite waiting message\n";return 0;}
    assert(!view->count); // Readable cars before a start are not a session.
    session_status=2;hub_status=2;emit("session.start");emit("pause");
    Sleep(210);frame(.2,nullptr);assert(!view->count && !(view->state&livewire::PAUSED));
    // The career workstation is inside StateSession, but must stay empty.
    fixture->live_enabled=true;hub_status=0;Sleep(210);frame(.2,nullptr);
    assert(view->count==24 && view->player==12 && (view->state&livewire::ACTIVE));
    assert(view->session_type==1&&view->session_kind==livewire::PRACTICE);
    assert(badge_view->session==view->session&&badge_view->sequence==view->sequence&&!(badge_view->sequence&1));
    assert(badge_view->cars[0].identity==view->cars[0].identity&&badge_view->cars[0].width==2&&badge_view->cars[0].rgba[3]==64);
    fixture->live.version=3;Sleep(210);frame(.2,nullptr);assert(view->count==24&&!badge_view->cars[0].width);
    fixture->live.version=4;Sleep(210);frame(.2,nullptr);assert(view->count==24&&badge_view->cars[0].width==2);
    // Same numeric ID, distinct weekend-aware Sprint/Race context.
    fixture->header.type=15;current_kind=livewire::SPRINT;
    emit_context("flow.state");Sleep(210);frame(.2,nullptr);
    assert(view->session_type==15&&view->session_kind==livewire::SPRINT);
    emit("session.end");assert(!view->session_type&&!view->session_kind&&!badge_view->cars[0].width);
    current_kind=livewire::RACE;emit("session.start");Sleep(210);frame(.2,nullptr);
    assert(view->session_type==15&&view->session_kind==livewire::RACE);
    context_available=false;emit_context("flow.state");Sleep(210);frame(.2,nullptr);
    assert(view->session_type==15&&!view->session_kind&&view->count==24);
    context_available=true;current_kind=livewire::SPRINT;emit_context("flow.state");
    fixture->header.type=16;Sleep(210);frame(.2,nullptr);
    assert(view->session_type==16&&!view->session_kind); // Reject an old sprint context in race_2.
    current_kind=livewire::PRACTICE;fixture->header.type=1;emit_context("flow.state");
    // These clocks run before a lap is completed, including in the garage.
    assert(!(view->cars[12].known&(livewire::LAST|livewire::BEST)));
    for(unsigned i=0;i<32;i++){float time=80.f+float(i);write(records[i]+0x48,20.f);write(records[i]+0x58,30.f);write(records[i]+0x68,time-50.f);}
    assert(get_lap(system.data(),12)==reinterpret_cast<void*>(0x12345678));
    Sleep(210);frame(.2,nullptr);assert(view->cars[12].last_ms==92000&&view->cars[12].best_ms==92000);
    assert(view->cars[0].position==1 && !std::strcmp(view->cars[0].name,"Driver 1"));
    assert(view->cars[0].wear_known==15 && view->cars[0].setup_known==(1u<<21)-1);
    for(unsigned id=0;id<view->count;id++) {
        const unsigned keys[]={livewire::PRESSURE_RL,livewire::PRESSURE_RR,livewire::PRESSURE_FL,livewire::PRESSURE_FR};
        for(unsigned wheel=0;wheel<4;wheel++) {
            assert(view->cars[id].setup_known&(1u<<keys[wheel]));
            assert(std::fabs(view->cars[id].setup[keys[wheel]]-((wheel<2?22.f:24.f)+.1f*float(wheel+1)+.1f*float(id%4)))<.001);
        }
    }
    assert((view->cars[1].known&livewire::PIT) && (view->cars[1].flags&livewire::IN_PIT_LANE));
    fixture->live_cars[1].pit_status=0;Sleep(210);frame(.2,nullptr);
    assert((view->cars[1].known&livewire::PIT) && !(view->cars[1].flags&livewire::IN_PIT_LANE));
    emit("pause");assert(view->state&livewire::PAUSED && view->count==24);
    emit("resume");assert(!(view->state&livewire::PAUSED));
    // Returning to the workstation clears rows even without session.end.
    hub_status=1;Sleep(210);frame(.2,nullptr);assert(!view->count);
    hub_status=2;Sleep(210);frame(.2,nullptr);assert(!view->count);
    hub_status=3;Sleep(210);frame(.2,nullptr);assert(!view->count);
    hub_status=0;Sleep(210);frame(.2,nullptr);assert(view->count==24);
    load_status=1;Sleep(210);frame(.2,nullptr);assert(!view->count);
    load_status=0;Sleep(210);frame(.2,nullptr);assert(view->count==24);
    unload_status=1;Sleep(210);frame(.2,nullptr);assert(!view->count);
    unload_status=0;Sleep(210);frame(.2,nullptr);assert(view->count==24);
    api.career=nullptr;Sleep(210);frame(.2,nullptr);
    assert(!view->count && std::strstr(view->diagnostic,"session-state API"));
    api.career=&career;Sleep(210);frame(.2,nullptr);assert(view->count==24 && !view->diagnostic[0]);
    fixture->live_enabled=false;Sleep(210);frame(.2,nullptr);
    assert(view->count==24&&(view->state&64)&&view->diagnostic[0]);
    emit("session.end");assert(!view->count&&(view->state&livewire::ENDED));
    emit("session.start");Sleep(210);frame(.2,nullptr);assert(!view->count&&view->diagnostic[0]);
    fixture->live_enabled=true;Sleep(210);frame(.2,nullptr);
    assert(view->count==24&&!(view->state&64)&&!view->diagnostic[0]);
    fixture->ops_enabled=false;Sleep(210);frame(.2,nullptr);
    assert(view->count==24&&(view->state&64)&&view->diagnostic[0]);
    fixture->ops_enabled=true;Sleep(210);frame(.2,nullptr);
    assert(view->count==24&&!(view->state&64)&&!view->diagnostic[0]);
    auto session=view->session;emit("session.end");assert(view->count==0 && view->session>session && (view->state&livewire::ENDED));
    Sleep(210);frame(.2,nullptr);assert(!view->count); // Stale core snapshot cannot resurrect ended session.
    emit("session.start");Sleep(210);frame(.2,nullptr);assert(view->count==24);
    emit("session.unload");assert(!view->count);
    UnmapViewOfFile(badge_view);CloseHandle(badge_handle);
    UnmapViewOfFile(view);CloseHandle(handle);FreeLibrary(dll);FreeLibrary(core);
    std::cout<<"PASS: real DLL, Live State/CarOps, 24-car grid, names, workstation/loading gates, timing, pause and end/start for build "<<nt->FileHeader.TimeDateStamp<<"\n";
}
