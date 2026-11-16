// SPDX-License-Identifier: MIT
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include "swofty_api.h"
#include "live_wire.h"
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
bool verified=false;
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
int subscribe(const char* event,SwoftyEventFn fn,void*,uint32_t* token){subscribers[event]=fn;*token=1;return SWOFTY_ASSET_OK;}
int on_frame(SwoftyFrameFn fn,void*,uint32_t* token){frame=fn;*token=2;return SWOFTY_ASSET_OK;}
void emit(const char* name){SwoftyEvent e{};e.name=name;assert(subscribers.count(name));subscribers[name](&e,nullptr);}
}
int main(int argc,char** argv){
    assert(argc==3||argc==4);verified=argc==4;
    auto dos=reinterpret_cast<IMAGE_DOS_HEADER*>(pe);dos->e_magic=IMAGE_DOS_SIGNATURE;dos->e_lfanew=0x80;
    auto nt=reinterpret_cast<IMAGE_NT_HEADERS64*>(pe+0x80);nt->Signature=IMAGE_NT_SIGNATURE;nt->FileHeader.Machine=IMAGE_FILE_MACHINE_AMD64;nt->FileHeader.TimeDateStamp=argc==4&&!std::strcmp(argv[3],"current")?builds::CURRENT_STEAM:builds::ORIGINAL_STEAM;
    auto core=LoadLibraryA(argv[1]);assert(core);
    assert(!GetProcAddress(core,"SwoftyCoreLive")); // The separate snapshot service is completely absent.
    if(verified) fixture=std::make_unique<BindingFixture>(24);
    auto dll=LoadLibraryA(argv[2]);assert(dll);
    auto init=reinterpret_cast<SwoftyAddonInitFn>(GetProcAddress(dll,"SwoftyAddonInit"));assert(init);
    SwoftyEvents events={available,subscribe,on_frame,nullptr};SwoftyApi api{};
    api.version=9;api.module_base=base;api.resolve=resolve;api.register_mixins=mixins;api.events=&events;api.log=log;
    assert(!init(nullptr));assert(init(&api));assert(load);load();assert(frame);
    std::vector<unsigned char> system(0x100);uint64_t entries[32]{};unsigned char records[32][512]{};
    auto write=[](void* to,const auto& value){std::memcpy(to,&value,sizeof value);};
    if(verified) {
        assert(get_lap && report_lap);write(system.data()+0x58,reinterpret_cast<uint64_t>(entries));write(system.data()+0x60,reinterpret_cast<uint64_t>(entries)+32*8);
        for(unsigned i=0;i<32;i++){entries[i]=reinterpret_cast<uint64_t>(records[i]);float time=80.f+float(i);write(records[i]+0xe8,time);}
        assert(get_lap(system.data(),12)==reinterpret_cast<void*>(0x12345678));
        assert(report_lap(system.data(),reinterpret_cast<void*>(0x1234))==0xdeadbeef12345678ull);
    } else assert(!get_lap&&!report_lap);
    wchar_t name[96];swprintf_s(name,L"Local\\USTelemetryLive-1-%lu",GetCurrentProcessId());
    auto handle=OpenFileMappingW(FILE_MAP_READ,FALSE,name);assert(handle);
    auto view=static_cast<const livewire::Frame*>(MapViewOfFile(handle,FILE_MAP_READ,0,0,sizeof(livewire::Frame)));assert(view);
    frame(.2,nullptr);assert(view->size==9216 && !(view->sequence&1));
    if(!verified) {assert(!view->count && view->diagnostic[0]);UnmapViewOfFile(view);CloseHandle(handle);FreeLibrary(dll);FreeLibrary(core);std::cout<<"PASS: unsupported build is reported instead of an indefinite waiting message\n";return 0;}
    assert(view->count==24 && view->player==12 && (view->state&livewire::ACTIVE) && (view->state&livewire::DIRECT_READY));
    // These clocks run before a lap is completed, including in the garage.
    assert(!(view->cars[12].known&(livewire::LAST|livewire::BEST)));
    for(unsigned i=0;i<32;i++){float time=80.f+float(i);write(records[i]+0x48,20.f);write(records[i]+0x58,30.f);write(records[i]+0x68,time-50.f);}
    Sleep(210);frame(.2,nullptr);assert(view->cars[12].last_ms==92000&&view->cars[12].best_ms==92000);
    assert(view->cars[0].position==1 && !std::strcmp(view->cars[0].name,"Driver 1"));
    assert(view->cars[0].wear_known==15 && view->cars[0].setup_known==(1u<<21)-1);
    emit("pause");assert(view->state&livewire::PAUSED && view->count==24);
    emit("resume");assert(!(view->state&livewire::PAUSED));
    auto session=view->session;emit("session.end");assert(view->count==0 && view->session>session && (view->state&livewire::ENDED));
    Sleep(210);frame(.2,nullptr);assert(!view->count); // Stale core snapshot cannot resurrect ended session.
    emit("session.start");Sleep(210);frame(.2,nullptr);assert(view->count==24);
    emit("session.unload");assert(!view->count);
    UnmapViewOfFile(view);CloseHandle(handle);FreeLibrary(dll);FreeLibrary(core);
    std::cout<<"PASS: real DLL, bindings-only core without snapshot export, 24-car grid, timing hooks, pause and end/start for build "<<nt->FileHeader.TimeDateStamp<<"\n";
}
