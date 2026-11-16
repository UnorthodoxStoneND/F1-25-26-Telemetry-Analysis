// SPDX-License-Identifier: MIT
// Synthetic host-test objects only; never instantiated by the runtime DLL.
#pragma once
#include "direct_live.h"
#include <cassert>
#include <memory>
#include <string>
#include <vector>
struct TestMemory {
 struct Block {std::unique_ptr<char[]> data;size_t size;};
 std::vector<Block> blocks;
 uint64_t alloc(size_t n) {auto p=std::make_unique<char[]>(n);auto address=reinterpret_cast<uint64_t>(p.get());blocks.push_back({std::move(p),n});return address;}
 bool operator()(uint64_t at,void* dst,size_t n) {
  for(const auto& b:blocks) {auto begin=reinterpret_cast<uint64_t>(b.data.get());if(at>=begin && at-begin<=b.size && n<=b.size-(at-begin)){std::memcpy(dst,b.data.get()+at-begin,n);return true;}}
  return false;
 }
 template<class T> void put(uint64_t at,T value){char previous[sizeof(T)];assert((*this)(at,previous,sizeof(T)));std::memcpy(reinterpret_cast<void*>(at),&value,sizeof(T));}
 uint64_t text(const std::string& value){auto p=alloc(128);assert(value.size()<128);std::memcpy(reinterpret_cast<void*>(p),value.c_str(),value.size()+1);return p;}
};
struct BindingFixture {
 TestMemory memory;
 directlive::Sites sites;
 uint64_t cars[32]{},results[32]{},sets[32]{},participants[32]{};
 unsigned count;
 int player;
 explicit BindingFixture(unsigned n=24):count(n),player(int(n/2)) {
  auto& m=memory;assert(n>=4&&n<=32);
  sites.table=m.alloc(n*8);sites.count=m.alloc(1);m.put(sites.count,uint8_t(n));sites.manager=m.alloc(8);sites.controllers=m.alloc(8);
  auto keys=m.alloc(64);sites.results=keys;sites.tyres=keys+8;sites.mode=keys+16;sites.entrants=keys+24;sites.participants=keys+32;sites.car_vt=keys+40;sites.human_vt=keys+48;sites.controller_vt=keys+56;
  auto manager=m.alloc(0x40),registry=m.alloc(0x30),slots=m.alloc(85);m.put(sites.manager,manager);m.put(manager+0x30,registry);m.put(registry+0x18,slots);m.put(registry+0x20,slots+80);
  auto result_system=m.alloc(0x160),tyre_system=m.alloc(0x120),mode_system=m.alloc(0x90),entrant_system=m.alloc(0x300),helper=m.alloc(0xa0);
  uint64_t ids[5]={sites.results,sites.tyres,sites.mode,sites.entrants,sites.participants},values[5]={result_system,tyre_system,mode_system,entrant_system,helper};
  for(unsigned i=0;i<5;i++){m.put(slots+16*i,ids[i]);m.put(slots+16*i+8,values[i]);m.put(slots+80+i,uint8_t(1));}
  auto result_list=m.alloc(n*8),tyre_list=m.alloc(n*8),mode=m.alloc(0x268),game=m.alloc(0x300),entries=m.alloc(n*0x88),parts=m.alloc(n*8),driver_table=m.alloc(0x20),driver_rows=m.alloc(n*16);
  m.put(result_system+0x148,result_list);m.put(result_system+0x150,result_list+n*8);m.put(tyre_system+0x110,tyre_list);m.put(tyre_system+0x118,tyre_list+n*8);
  m.put(mode_system+0x88,mode);m.put(mode+0x260,game);m.put(game+0x2e0,entries);m.put(game+0x2e8,entries+n*0x88);
  m.put(helper+0x88,parts);m.put(helper+0x90,parts+n*8);m.put(helper+0x60,m.text("Human Driver"));
  m.put(entrant_system+0x268,driver_table);m.put(driver_table+8,driver_rows);m.put(driver_table+16,driver_rows+n*16);
  auto controller=m.alloc(0x370),counts=m.alloc(0x40),controller_sets=m.alloc(n*8),dummy_count=m.alloc(n*8);
  m.put(sites.controllers,controller);m.put(controller,sites.controller_vt);m.put(controller+0x365,uint8_t(1));m.put(controller+0x90,counts);m.put(controller+0x30,controller_sets);m.put(counts+0x28,dummy_count);m.put(counts+0x30,dummy_count+n*8);
  for(unsigned id=0;id<n;id++) {
   auto car=m.alloc(0xa000);cars[id]=car;m.put(sites.table+8*id,car);m.put(car+0x98,car+0xa0);m.put(car+0x380,sites.car_vt);m.put(car+0xe8,uint8_t(id));
   auto ers=m.alloc(0x20);m.put(car+0x5a8,ers);m.put(ers+8,car);
   auto result=m.alloc(0xa0);results[id]=result;m.put(result_list+8*id,result);m.put(result,uint8_t(id));m.put(result+0x48,uint32_t(id^0x12345678u));m.put(result+0x50,uint32_t(0x12345678));m.put(result+0x90,uint32_t(n-id-1));
   auto entry=entries+id*0x88;m.put(entry+0x82,uint8_t(id));m.put(entry+0x50,uint32_t(id<<16));
   auto record=m.alloc(0xd0);m.put(driver_rows+id*16+8,record);m.put(record+0xc8,m.text("Driver "+std::to_string(id+1)));
   auto part=m.alloc(0x40),physics=m.alloc(0x60);participants[id]=part;m.put(parts+8*id,part);m.put(part+0x18,physics);m.put(physics+0x58,car);m.put(part+0x32,uint8_t(int(id)==player?0x80:0));
   auto set=m.alloc(0x40);sets[id]=set;m.put(controller_sets+8*id,set);m.put(set+0x34,uint8_t(id));
   if(int(id)==player){auto human=m.alloc(8);m.put(set+0x20,human);m.put(human,sites.human_vt);}
   auto tyre_object=m.alloc(0x9ca0),fitted=m.alloc(0x78),physical=m.alloc(8),visual=m.alloc(0x58);m.put(tyre_list+id*8,tyre_object);
   m.put(tyre_object+0x9bc0,uint64_t(7));m.put(tyre_object+0x9c50,uint64_t(7));m.put(tyre_object+0x9c58,uint64_t(1));m.put(tyre_object+0x9c48,uint64_t(1));m.put(tyre_object+0x9c90,fitted);m.put(tyre_object+0x9c98,fitted+0x78);m.put(fitted+12,uint32_t(1));
   m.put(tyre_object+0x9bd0,uint32_t(1));m.put(tyre_object+0x9bd8,physical);m.put(tyre_object+0x9be0,visual);m.put(physical,uint8_t(id==n-1?7:id==n-2?8:18));m.put(visual,uint8_t(id==n-1?7:id==n-2?8:16+id%3));
   auto dynamics=m.alloc(0x100),tune=m.alloc(0x300),aero=m.alloc(0x1a00),brake=m.alloc(0x2200),susp=m.alloc(0x100),fuel=m.alloc(0x110);
   m.put(car+0x900,dynamics);m.put(car+0x590,tune);m.put(car+0x570,aero);m.put(car+0x550,brake);m.put(car+0x580,susp);m.put(car+0x540,fuel);m.put(car+0x9f40,uint32_t(8));
   for(auto component:{tune,aero,brake,susp,fuel})m.put(component+8,car);
   for(auto slider:readers::SLIDERS)m.put(tune+slider.offset,.5f);
   m.put(tune+0x248,.5f);m.put(tune+0x24c,.5f);m.put(aero+0x19b4,uint32_t(20));m.put(aero+0x19c0,uint32_t(25));m.put(brake+0x21dc,.98f);m.put(brake+0x21d8,.4f);m.put(fuel+0x100,50.f);
   for(unsigned wheel=0;wheel<4;wheel++){auto tyre=m.alloc(0x100),physical_wheel=m.alloc(0x70);m.put(dynamics+0x60+wheel*8,tyre);m.put(tyre+0x10,car);m.put(tyre+0x88,.05f+.01f*float(wheel));m.put(susp+0x70+wheel*8,physical_wheel);m.put(physical_wheel+0x64,wheel<2?22.f:24.f);}
  }
 }
 uint64_t resolve(const char* name)const {
  if(!std::strcmp(name,"g_Sess_VehicleTable"))return sites.table;
  if(!std::strcmp(name,"g_Sess_VehicleCount"))return sites.count;
  if(!std::strcmp(name,"g_Game_Manager"))return sites.manager;
  if(!std::strcmp(name,"g_Live_ControllerSystem"))return sites.controllers;
  if(!std::strcmp(name,"vt_Car_SystemsContainer"))return sites.car_vt;
  if(!std::strcmp(name,"vt_Sess_ControllerSystem"))return sites.controller_vt;
  if(!std::strcmp(name,"vt_Sess_HumanController"))return sites.human_vt;
  if(!std::strcmp(name,"k_Live_SessionSystemKey"))return sites.results;
  if(!std::strcmp(name,"k_Live_TyreSystemKey"))return sites.tyres;
  if(!std::strcmp(name,"k_Live_GameModeSystemKey"))return sites.mode;
  if(!std::strcmp(name,"k_Live_EntrantsSystemKey"))return sites.entrants;
  if(!std::strcmp(name,"k_Live_VehicleHelperSystemKey"))return sites.participants;
  return 0;
 }
};
