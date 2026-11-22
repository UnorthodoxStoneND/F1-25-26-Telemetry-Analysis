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
bool verified=false,missing_screen=false;
uint32_t session_status=0,hub_status=0,load_status=0,unload_status=0;
unsigned char pe[4096]{};
std::unique_ptr<BindingFixture> fixture;
uint64_t(*open_screen)(void*,void*,void*,void*);
unsigned screen_calls=0;
uint64_t screen_original(void* scene,void*,void* a3,void* a4) {
 assert(scene==reinterpret_cast<void*>(0x1234)&&a3==reinterpret_cast<void*>(0x2345)&&a4==reinterpret_cast<void*>(0x3456));
 screen_calls++;return 0xabcdef1234567890ull;
}
void log(const char*,...){}
uintptr_t base(){return verified?reinterpret_cast<uintptr_t>(pe):reinterpret_cast<uintptr_t>(GetModuleHandleW(nullptr));}
uintptr_t resolve(const char* name){if(!std::strcmp(name,"Vm_Open_Screen"))return missing_screen?0:reinterpret_cast<uintptr_t>(screen_original);return fixture?fixture->resolve(name):0;}
int mixins(const SwoftyMixin* list,size_t count){for(size_t i=0;i<count;i++) {
 if(list[i].kind==SWOFTY_MIXIN_ON_LOAD)load=reinterpret_cast<void(*)()>(list[i].handler);
 else if(!std::strcmp(list[i].target,"Vm_Open_Screen")){assert(!missing_screen);open_screen=reinterpret_cast<decltype(open_screen)>(list[i].handler);*list[i].orig=reinterpret_cast<void*>(screen_original);}
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
void emit_flow(const char* name,uint32_t status) {
 SwoftyFlowState data{};data.hash=hash(name);data.to=status;
 SwoftyEvent event{};event.name="flow.state";event.data=&data;
 subscribers.at("flow.state")(&event,nullptr);
}
void emit_context(const char* point) {
 assert(context_subscribers.count(point));SwoftyHookCall call{};call.context=&current_kind;
 context_subscribers[point](nullptr,&call);
}
void emit_screen(const char* name) {
 std::vector<unsigned char> text(9+std::strlen(name)+1),screen(0x70);
 std::memcpy(text.data()+9,name,std::strlen(name)+1);
 auto pointer=reinterpret_cast<uint64_t>(text.data());std::memcpy(screen.data()+0x68,&pointer,sizeof pointer);
 auto before=screen_calls;
 assert(open_screen(reinterpret_cast<void*>(0x1234),screen.data(),reinterpret_cast<void*>(0x2345),reinterpret_cast<void*>(0x3456))==0xabcdef1234567890ull);
 assert(screen_calls==before+1);
}
void emit(const char* name){SwoftyEvent e{};e.name=name;assert(subscribers.count(name));subscribers[name](&e,nullptr);
 if(context_subscribers.count(name))emit_context(name);
}
}
int main(int argc,char** argv){
    assert(argc==3||argc==4||argc==5);verified=argc>=4;missing_screen=argc==5;
    auto dos=reinterpret_cast<IMAGE_DOS_HEADER*>(pe);dos->e_magic=IMAGE_DOS_SIGNATURE;dos->e_lfanew=0x80;
    auto nt=reinterpret_cast<IMAGE_NT_HEADERS64*>(pe+0x80);nt->Signature=IMAGE_NT_SIGNATURE;nt->FileHeader.Machine=IMAGE_FILE_MACHINE_AMD64;nt->FileHeader.TimeDateStamp=argc>=4&&!std::strcmp(argv[3],"current")?builds::CURRENT_STEAM:builds::ORIGINAL_STEAM;
    auto core=LoadLibraryA(argv[1]);assert(core);
    assert(GetProcAddress(core,"SwoftyCoreLive")&&GetProcAddress(core,"SwoftyCoreCarOps"));
    if(verified) {
        fixture=std::make_unique<BindingFixture>(24);
        fixture->live.version=6;fixture->live.tyre_icon_texture=badge_reference;fixture->live.team_badge_texture=badge_reference;fixture->live.images=image_service;
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
    wchar_t name[96];swprintf_s(name,L"Local\\USTelemetryLive-2-%lu",GetCurrentProcessId());
    auto handle=OpenFileMappingW(FILE_MAP_READ,FALSE,name);assert(handle);
    auto view=static_cast<const livewire::Frame*>(MapViewOfFile(handle,FILE_MAP_READ,0,0,sizeof(livewire::Frame)));assert(view);
    swprintf_s(name,L"Local\\USTelemetryBadges-2-%lu",GetCurrentProcessId());
    auto badge_handle=OpenFileMappingW(FILE_MAP_READ,FALSE,name);assert(badge_handle);
    auto badge_view=static_cast<const badgewire::Frame*>(MapViewOfFile(badge_handle,FILE_MAP_READ,0,0,sizeof(badgewire::Frame)));assert(badge_view);
    assert(badge_view->size==sizeof(badgewire::Frame)&&!badge_view->cars[0].width);
    frame(.2,nullptr);assert(view->size==10752 && !(view->sequence&1));
    if(!verified) {assert(!view->count && view->diagnostic[0]);UnmapViewOfFile(badge_view);CloseHandle(badge_handle);UnmapViewOfFile(view);CloseHandle(handle);FreeLibrary(dll);FreeLibrary(core);std::cout<<"PASS: unsupported build is reported instead of an indefinite waiting message\n";return 0;}
    assert(!view->count); // Readable cars before a start are not a session.
    session_status=2;hub_status=2;emit("session.start");emit("pause");
    Sleep(210);frame(.2,nullptr);assert(!view->count && !(view->state&livewire::PAUSED));
    // The career workstation is inside StateSession, but must stay empty.
    fixture->live_enabled=true;hub_status=0;Sleep(210);frame(.2,nullptr);
    assert(view->count==24 && view->player==12 && (view->state&livewire::ACTIVE));
    if(missing_screen) {
        assert(!open_screen);
        emit_flow("ZoomToMonitorCutscene",1);assert(view->state&livewire::WINDOW_READY);
        assert(view->count==24&&view->cars[0].position==1&&view->cars[0].name[0]);
        emit("pause");assert(view->state&livewire::PAUSED);
        emit("resume");assert(!(view->state&livewire::PAUSED));
        emit("session.end");assert(!view->count&&(view->state&livewire::ENDED));
        UnmapViewOfFile(badge_view);CloseHandle(badge_handle);UnmapViewOfFile(view);CloseHandle(handle);FreeLibrary(dll);FreeLibrary(core);
        std::cout<<"PASS: missing optional screen binding never registered; publisher, live field, retro readiness and lifecycle remain enabled for build "<<nt->FileHeader.TimeDateStamp<<"\n";
        return 0;
    }
    fixture->live_cars[0].have|=SWOFTY_LIVE_HAVE_PENALTIES;
    fixture->live_cars[0].drive_through=2;fixture->live_cars[0].stop_go=1;fixture->live_cars[0].time_penalty_s=5.25f;
    Sleep(210);frame(.2,nullptr);
    assert((view->cars[0].known&livewire::PENALTIES)&&view->cars[0].drive_through==2&&view->cars[0].stop_go==1);
    assert((view->cars[0].known&livewire::TIME_PENALTY)&&view->cars[0].time_penalty_ms==5250);
    fixture->live_cars[0].drive_through=0;fixture->live_cars[0].stop_go=0;fixture->live_cars[0].time_penalty_s=0;
    Sleep(210);frame(.2,nullptr);
    assert((view->cars[0].known&livewire::PENALTIES)&&!view->cars[0].drive_through&&!view->cars[0].stop_go);
    fixture->header.have|=SWOFTY_LIVE_SHAVE_ORDER;fixture->header.classified=24;
    for(unsigned i=0;i<24;i++)fixture->header.order[i]=23-i;
    Sleep(210);frame(.2,nullptr);
    assert(view->state&livewire::CLASSIFICATION_ORDER);
    for(unsigned i=0;i<24;i++)assert(view->cars[i].vehicle==23-i);
    fixture->header.order[0]=32;Sleep(210);frame(.2,nullptr);
    assert(!(view->state&livewire::CLASSIFICATION_ORDER)&&view->cars[0].vehicle==0);
    fixture->header.have&=~SWOFTY_LIVE_SHAVE_ORDER;
    // A live grid behind session selection is insufficient to open the window.
    assert(!(view->state&livewire::WINDOW_READY));
    emit_flow("IntroCameras",1);assert(!(view->state&livewire::WINDOW_READY));
    emit_flow("SessionStateBeginGameplay",1);assert(!(view->state&livewire::WINDOW_READY));
    // Open at the garage transition, before BeginGameplay or InGame.
    emit_flow("ZoomToMonitorCutscene",1);assert(view->state&livewire::WINDOW_READY);
    emit_flow("ZoomToMonitorCutscene",0);Sleep(210);frame(.2,nullptr);
    assert(view->state&livewire::WINDOW_READY); // Transient garage entry is latched.

    assert(view->session_type==1&&view->session_kind==livewire::PRACTICE);
    assert(badge_view->session==view->session&&badge_view->sequence==view->sequence&&!(badge_view->sequence&1));
    assert(badge_view->cars[0].identity==view->cars[0].identity&&badge_view->cars[0].width==2&&badge_view->cars[0].rgba[3]==64);
    fixture->live.version=5;Sleep(210);frame(.2,nullptr);assert(view->count==24&&(view->state&64));
    fixture->live.version=6;Sleep(210);frame(.2,nullptr);assert(view->count==24&&badge_view->cars[0].width==2&&badge_view->tyres[0].width==2);
    // Tyre age is the fitted set's actual total, not laps, compound changes
    // or current-stint length. Core revisions (including Flashback) are copied.
    for(unsigned id=0;id<24;id++)assert((view->cars[id].known&livewire::TYRE_AGE)&&view->cars[id].tyre_age_laps==7);
    fixture->details[0].tyre_age_laps=12;fixture->details[0].tyre_stint_laps=2;
    fixture->live_cars[0].lap=8;fixture->live_cars[0].pit_status=1;
    Sleep(210);frame(.2,nullptr);assert(view->cars[0].tyre_age_laps==12);
    fixture->details[0].tyre_age_laps=11;
    Sleep(210);frame(.2,nullptr);assert(view->cars[0].tyre_age_laps==11);
    fixture->details[0].tyre_age_laps=0;
    Sleep(210);frame(.2,nullptr);assert((view->cars[0].known&livewire::TYRE_AGE)&&view->cars[0].tyre_age_laps==0);
    fixture->header.type=15;current_kind=livewire::SPRINT;
    emit_context("flow.state");Sleep(210);frame(.2,nullptr);
    assert(view->session_type==15&&view->session_kind==livewire::SPRINT);
    fixture->live_cars[0].tyre_actual=10;fixture->live_cars[0].tyre_visual=10;
    Sleep(210);frame(.2,nullptr);
    assert((view->cars[0].known&livewire::TYRES)&&view->cars[0].compound==10&&view->cars[0].visual==10);

    emit("session.end");assert(!view->session_type&&!view->session_kind&&!badge_view->cars[0].width);
    fixture->live_cars[0].lap=0;current_kind=livewire::RACE;emit("session.start");Sleep(210);frame(.2,nullptr);
    assert(view->session_type==15&&view->session_kind==livewire::RACE&&view->cars[0].tyre_age_laps==0);
    assert(!(view->state&livewire::WINDOW_READY));
    // Recorded race order: grid rundown -> pre-race -> lights -> BeginGameplay.
    // Readable data and intro cameras must not open; grid/pre-race must.
    emit_flow("IntroCameras",1);assert(!(view->state&livewire::WINDOW_READY));
    emit_flow("GridRundownType",1);assert(view->state&livewire::WINDOW_READY);
    emit_flow("PreRace",1);assert(view->state&livewire::WINDOW_READY);
    emit_flow("PreLights",1);assert(view->state&livewire::WINDOW_READY);
    emit_flow("BeginGameplay",1);assert(view->state&livewire::WINDOW_READY);
    context_available=false;emit_context("flow.state");Sleep(210);frame(.2,nullptr);
    assert(view->session_type==15&&!view->session_kind&&view->count==24);
    context_available=true;current_kind=livewire::SPRINT;emit_context("flow.state");
    fixture->header.type=16;Sleep(210);frame(.2,nullptr);
    assert(view->session_type==16&&!view->session_kind); // Reject an old sprint context in race_2.
    current_kind=livewire::PRACTICE;fixture->header.type=1;emit_context("flow.state");
    // These clocks run before a lap is completed, including in the garage.
    assert(!(view->cars[12].known&(livewire::LAST|livewire::BEST)));
    auto& detail=fixture->details[12];detail.last_lap_ms=detail.best_lap_ms=92000;
    detail.last_sector_ms[0]=detail.best_sector_ms[0]=20000;
    detail.last_sector_ms[1]=detail.best_sector_ms[1]=30000;
    detail.last_sector_ms[2]=detail.best_sector_ms[2]=42000;
    Sleep(210);frame(.2,nullptr);assert(view->cars[12].last_ms==92000&&view->cars[12].best_ms==92000);
    assert((view->cars[12].known&livewire::LAST_SECTORS)&&view->cars[12].last_sectors[0]==20000&&view->cars[12].last_sectors[1]==30000&&view->cars[12].last_sectors[2]==42000);
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
    assert(view->cars[1].flags&livewire::OUT_LAP);
    fixture->details[1].laps_completed++;
    Sleep(210);frame(.2,nullptr);assert(!(view->cars[1].flags&livewire::OUT_LAP));
    emit("pause");assert(view->state&livewire::PAUSED && view->count==24 && (view->state&livewire::WINDOW_READY));
    emit("resume");assert(!(view->state&livewire::PAUSED));
    // Recorded post-race UI: standings -> career rewards/Finances, before
    // StateSession exits and without the post-practice CareerHubUpdate event.
    emit_screen("session_results_container");assert(view->state&livewire::WINDOW_READY);
    emit_screen("slice_rewards_career_pagination");assert(!(view->state&livewire::WINDOW_READY));
    Sleep(210);frame(.2,nullptr);
    assert(view->count==24&&(view->state&livewire::ACTIVE)&&!(view->state&livewire::WINDOW_READY));
    emit_screen("slice_bg_my_team_full");assert(!(view->state&livewire::WINDOW_READY));
    emit_screen("session_results_container");assert(!(view->state&livewire::WINDOW_READY));
    assert(open_screen(reinterpret_cast<void*>(0x1234),nullptr,reinterpret_cast<void*>(0x2345),reinterpret_cast<void*>(0x3456))==0xabcdef1234567890ull);
    emit_flow("ZoomToMonitorCutscene",1);assert(view->state&livewire::WINDOW_READY);
    // Actual post-practice order: the hub update completes within one
    // millisecond, then rewards open while StateSession and cars remain live.
    // No on_frame poll runs during this transition.
    emit_flow("SessionStateCareerHubUpdate",1);assert(!(view->state&livewire::WINDOW_READY));
    emit_flow("SessionStateCareerHubUpdate",2);
    emit_flow("SessionStateCareerHubUpdate",3);
    emit_flow("SessionStateCareerHubUpdate",0);
    Sleep(210);frame(.2,nullptr);
    assert(view->count==24 && (view->state&livewire::ACTIVE) && !(view->state&livewire::WINDOW_READY));
    // The next genuine garage entry still opens before driving starts.
    emit_flow("ZoomToMonitorCutscene",1);assert(view->state&livewire::WINDOW_READY);
    // Returning to the workstation clears rows even without session.end.
    hub_status=1;Sleep(210);frame(.2,nullptr);assert(!view->count);
    hub_status=2;Sleep(210);frame(.2,nullptr);assert(!view->count);
    hub_status=3;Sleep(210);frame(.2,nullptr);assert(!view->count);
    hub_status=0;Sleep(210);frame(.2,nullptr);assert(view->count==24);
    assert(!(view->state&livewire::WINDOW_READY));
    emit_flow("SessionStatePreRace",1);assert(!(view->state&livewire::WINDOW_READY));
    emit_flow("PreRace",2);assert(view->state&livewire::WINDOW_READY);
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
    assert(!(view->state&livewire::WINDOW_READY));
    emit_flow("InGame",1);assert(view->state&livewire::WINDOW_READY);
    emit("session.unload");assert(!view->count);
    UnmapViewOfFile(badge_view);CloseHandle(badge_handle);
    UnmapViewOfFile(view);CloseHandle(handle);FreeLibrary(dll);FreeLibrary(core);
    std::cout<<"PASS: real DLL, Live State/CarOps, 24-car grid, names, workstation/loading gates, timing, pause and end/start for build "<<nt->FileHeader.TimeDateStamp<<"\n";
}
