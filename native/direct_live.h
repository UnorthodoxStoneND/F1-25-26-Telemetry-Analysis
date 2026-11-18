// SPDX-License-Identifier: MIT
#pragma once
#include "readers.h"
#include "live_api.h"
#include <cstring>
#include <cstdio>

// Independent, read-only readers using Swofty's named game bindings.
// Factual layouts are checked again at runtime:
// vehicle id/lock/vtable, subsystem ownership, bounded vectors and unique ranks.
namespace directlive {
struct Sites {
 uint64_t table=0,count=0,manager=0,results=0,tyres=0,mode=0,entrants=0,participants=0,pit=0;
 uint64_t controllers=0,controller_vt=0,human_vt=0,car_vt=0;
};
struct Entry {uint64_t key,value;};
struct Registry {
 Entry entries[4096]{};uint8_t occupied[4096]{};size_t count=0;
 template<class R> bool load(R& r,uint64_t slot) {
  count=0;auto manager=readers::pointer(r,slot);auto map=readers::pointer(r,manager+0x30);
  auto data=readers::pointer(r,map+0x18),meta=readers::pointer(r,map+0x20);
  if(!data||meta<=data||(meta-data)%16||(meta-data)/16>4096) return false;
  size_t n=size_t((meta-data)/16);
  if(!r(data,entries,n*sizeof(Entry))||!r(meta,occupied,n)) return false;
  count=n;return true;
 }
 uint64_t find(uint64_t key)const {
  if(!key) return 0;
  for(size_t i=0;i<count;i++) if(occupied[i] && entries[i].key==key && entries[i].value>=0x10000) return entries[i].value;
  return 0;
 }
};
template<class R> size_t vector_size(R& r,uint64_t owner,unsigned offset,size_t stride,size_t limit,uint64_t& begin) {
 uint64_t end=0;begin=readers::pointer(r,owner+offset);
 if(!begin||!readers::read(r,owner+offset+8,end)||end<begin||(end-begin)%stride||(end-begin)/stride>limit) return 0;
 return size_t((end-begin)/stride);
}
template<class R> bool name_at(R& r,uint64_t at,char (&out)[128]) {
 char text[128]{};size_t n=0;
 for(;n<sizeof(text);n++) {if(!readers::read(r,at+n,text[n])) return false;if(!text[n]) break;}
 if(!n||n==sizeof(text)) return false;
 // Reject control bytes and malformed UTF-8 rather than showing random memory.
 for(size_t i=0;i<n;) {
  auto first=uint8_t(text[i]);size_t length=first<0x80?1:first>=0xc2&&first<0xe0?2:first<0xf0&&first>=0xe0?3:first>=0xf0&&first<=0xf4?4:0;
  if(!length||first<0x20||first==0x7f||i+length>n) return false;
  for(size_t k=1;k<length;k++) if((uint8_t(text[i+k])&0xc0)!=0x80) return false;
  if(length==3&&((first==0xe0&&uint8_t(text[i+1])<0xa0)||(first==0xed&&uint8_t(text[i+1])>=0xa0))) return false;
  if(length==4&&((first==0xf0&&uint8_t(text[i+1])<0x90)||(first==0xf4&&uint8_t(text[i+1])>=0x90))) return false;
  i+=length;
 }
 std::memcpy(out,text,n+1);return true;
}
template<class R> int player(R& r,const Sites& s) {
 auto system=readers::pointer(r,s.controllers);uint8_t built=0;
 if(!s.controller_vt||!s.human_vt||readers::pointer(r,system)!=s.controller_vt||!readers::read(r,system+0x365,built)||!built) return -1;
 auto counts=readers::pointer(r,system+0x90),sets=readers::pointer(r,system+0x30);uint64_t begin=0,end=0;
 if(!counts||!sets||!readers::read(r,counts+0x28,begin)||!readers::read(r,counts+0x30,end)||end<begin||(end-begin)%8||(end-begin)/8>32) return -1;
 int found=-1;
 for(unsigned id=0;id<(end-begin)/8;id++) {
  auto set=readers::pointer(r,sets+id*8),human=readers::pointer(r,set+0x20);uint8_t vehicle=255;
  if(human && readers::pointer(r,human)==s.human_vt && readers::read(r,set+0x34,vehicle)&&vehicle==id) {
   if(found>=0) return -1;found=int(id);
  }
 }
 return found;
}
struct Context {
 Registry registry;
 uint64_t results=0,helper=0,mode=0,game=0,entrants=0,tyres=0,pit=0,result_list=0,tyre_list=0,entry_list=0,participant_list=0;
 size_t result_count=0,tyre_count=0,entry_count=0,participant_count=0;
 template<class R> void load(R& r,const Sites& s,bool need_tyres=true,bool need_pit=true) {
  registry.load(r,s.manager);
  results=registry.find(s.results);helper=registry.find(s.participants);mode=registry.find(s.mode);entrants=registry.find(s.entrants);tyres=registry.find(s.tyres);
  if(!need_tyres) tyres=0;
  pit=need_pit?registry.find(s.pit):0;
  auto current_mode=readers::pointer(r,mode+0x88);game=readers::pointer(r,current_mode+0x260);
  result_count=vector_size(r,results,0x148,8,64,result_list);
  tyre_list=0;tyre_count=need_tyres?vector_size(r,tyres,0x110,8,64,tyre_list):0;
  entry_count=vector_size(r,game,0x2e0,0x88,512,entry_list);participant_count=vector_size(r,helper,0x88,8,64,participant_list);
 }
 template<class R> uint64_t result(R& r,unsigned id)const {
  if(id>=result_count) return 0;auto record=readers::pointer(r,result_list+id*8);uint8_t vehicle=255;
  return readers::read(r,record,vehicle)&&vehicle==id?record:0;
 }
 template<class R> void ranks(R& r,unsigned id,livewire::Car& c)const {
  if((c.known&(livewire::POSITION|livewire::RESULT))==(livewire::POSITION|livewire::RESULT)) return;
  auto record=result(r,id);if(!record) return;
  uint8_t status=255;
  if(!(c.known&livewire::RESULT)&&readers::read(r,record+0x18,status)&&status<=7) {
   c.known|=livewire::RESULT;
   c.flags=(c.flags&~livewire::RESULT_MASK)|(uint32_t(status)<<livewire::RESULT_SHIFT);
  }
  if(c.known&livewire::POSITION) return; // Grid is only a fallback ordering key.
  uint32_t a=0,b=0;
  if(readers::read(r,record+0x48,a)&&readers::read(r,record+0x50,b)&&(a^b)<result_count&&(a^b)<32) {c.position=int32_t((a^b)+1);c.known|=livewire::POSITION;}
  if(readers::read(r,record+0x90,a)&&readers::read(r,record+0x98,b)&&(a^b)<result_count&&(a^b)<32) {c.grid=int32_t((a^b)+1);c.known|=livewire::GRID;}
 }
 template<class R> void driver(R& r,unsigned id,livewire::Car& c)const {
  uint64_t part=0;
  for(size_t i=0;i<participant_count;i++) {
   auto p=readers::pointer(r,participant_list+i*8),physics=readers::pointer(r,p+0x18),car=readers::pointer(r,physics+0x58);uint8_t vehicle=255;
   if(car&&readers::read(r,car+0xe8,vehicle)&&vehicle==id) {part=p;break;}
  }
  uint8_t flags=0;if(!part||!readers::read(r,part+0x32,flags)) return;
  uint64_t name=0,first=0;
  for(size_t i=0;i<entry_count;i++) {
   auto entry=entry_list+i*0x88;uint8_t vehicle=255;uint32_t reference=0;
   if(!readers::read(r,entry+0x82,vehicle)||vehicle!=id) continue;
   if(!readers::read(r,entry+0x50,reference)||(reference&255)>2) break;
   auto driver_table=readers::pointer(r,entrants+0x268+(reference&255)*16);uint64_t records=0;
   auto count=vector_size(r,driver_table,8,16,65536,records);auto index=reference>>16;
   if(index<count) {
    auto record=readers::pointer(r,records+index*16+8);
    // Loaded DriverRecord = packed ref + DriverInfo. Its localised first/last
    // string pointers are +0x30/+0xc8 (Steam driver-loader and copy constructor).
    first=readers::pointer(r,record+0x30);name=readers::pointer(r,record+0xc8);
   }break;
  }
  char surname[128]{},given[128]{};
  if(name && name_at(r,name,surname)) {
   size_t last_size=std::strlen(surname);
   if(first && name_at(r,first,given) && last_size+1+std::strlen(given)<sizeof c.name) {
    std::memcpy(c.name,surname,last_size);c.name[last_size]=' ';
    std::memcpy(c.name+last_size+1,given,std::strlen(given)+1);
    c.flags|=uint32_t(last_size)<<16; // UTF-8 byte boundary, not a guessed word split.
   } else std::memcpy(c.name,surname,last_size+1);
   c.known|=livewire::NAME;
  } else if((flags&0x80) && name_at(r,readers::pointer(r,helper+0x60),c.name)) c.known|=livewire::NAME;
 }
 template<class R> void compound(R& r,unsigned id,livewire::Car& c)const {
  if(id>=tyre_count) return;
  auto object=readers::pointer(r,tyre_list+id*8);uint64_t version=0,for_version=0,built=0,count=0,sets=0,end=0;
  if(!object||!readers::read(r,object+0x9bc0,version)||!readers::read(r,object+0x9c50,for_version)||version!=for_version||
     !readers::read(r,object+0x9c58,built)||!built||!readers::read(r,object+0x9c48,count)||!count||count>5||
     !readers::read(r,object+0x9c90,sets)||!readers::read(r,object+0x9c98,end)||end<sets||end-sets<0x78) return;
  uint32_t fitted=0;if(!readers::read(r,end-0x6c,fitted)) return;
  uint64_t chosen=0,fallback=0;
  for(size_t i=0;i<count;i++) {
   auto desc=object+0x9bd0+i*24;uint32_t descriptor=0;
   if(!readers::read(r,desc,descriptor)) return;
   if(descriptor==fitted) chosen=desc;
   if(descriptor==(fitted>=3?4u:1u)) fallback=desc;
  }
  if(!chosen) chosen=fallback;if(!chosen) return;
  auto physical=readers::pointer(r,chosen+8),visual=readers::pointer(r,chosen+16);uint8_t actual=255,colour=255;
  if(!readers::read(r,physical,actual)||!readers::read(r,visual,colour)) return;
  if(!((actual>=16&&actual<=20)||actual==7||actual==8)||!((colour>=16&&colour<=18)||colour==7||colour==8)) return;
  c.compound=actual;c.visual=colour;c.known|=livewire::TYRES;
 }
 template<class R> void pit_lane(R& r,unsigned id,livewire::Car& c)const {
  // The game's lane arrays have 27 slots, independently of the vehicle table.
  if(!pit || id>=27) return;
  auto state=readers::pointer(r,pit+0x68+id*16);uint32_t phase=0;
  auto lane=readers::pointer(r,pit+0x48);
  auto vehicle=readers::pointer(r,lane+0x938+id*16);uint8_t inside=0;
  if(!state||!lane||!vehicle||!readers::read(r,state+4,phase)||phase>15||
     !readers::read(r,vehicle+0x103,inside)||inside>1) return;
  c.known|=livewire::PIT;
  if(inside) c.flags|=livewire::IN_PIT_LANE;
 }
 template<class R> bool fill(R& r,const Sites& s,livewire::Frame& frame,const SwoftyLive* api=nullptr) {
  frame={};frame.player=-1;frame.state=livewire::DIRECT_READY;
  uint8_t count=0;
  if(!s.table||!s.count||!s.car_vt) {std::snprintf(frame.diagnostic,sizeof frame.diagnostic,"Required vehicle bindings are unavailable.");return false;}
  if(!readers::read(r,s.count,count)||count>32) {std::snprintf(frame.diagnostic,sizeof frame.diagnostic,"The vehicle count could not be read within the supported range.");return false;}
  if(!count) return false;
  liveapi::Snapshot snapshot;snapshot.load(api,count);
  if(!snapshot.player(frame.player,count)) frame.player=player(r,s);
  bool need_tyres=false,need_pit=false;
  for(unsigned id=0;id<count;id++) {
   livewire::Car c{};snapshot.apply(id,c,count);
   need_tyres|=!(c.known&livewire::TYRES);need_pit|=!(c.known&livewire::PIT);
  }
  load(r,s,need_tyres,need_pit);
  uint64_t positions=0,grids=0;bool duplicate_positions=false,duplicate_grids=false;
  for(unsigned id=0;id<count;id++) {
   auto car=readers::pointer(r,s.table+id*8);uint8_t vehicle=255;
   if(!car||readers::pointer(r,car+0x98)!=car+0xa0||!readers::read(r,car+0xe8,vehicle)||vehicle!=id||
      !s.car_vt||readers::pointer(r,car+0x380)!=s.car_vt) continue;
   auto ers=readers::pointer(r,car+0x5a8);
   if((!ers||readers::pointer(r,ers+8)!=car)&&!result(r,id)) continue;
   auto& c=frame.cars[frame.count++];c.vehicle=id;c.identity=car;c.flags=8|(frame.player==int(id)?1:0);
   snapshot.apply(id,c,count);
   ranks(r,id,c);driver(r,id,c);
   if(!(c.known&livewire::TYRES)) compound(r,id,c);
   readers::wear(r,car,c);readers::setup(r,car,c);
   if(!(c.known&livewire::PIT)) pit_lane(r,id,c);
   if(c.known&livewire::POSITION) {uint64_t bit=1ull<<c.position;duplicate_positions|=(positions&bit)!=0;positions|=bit;}
   if(c.known&livewire::GRID) {uint64_t bit=1ull<<c.grid;duplicate_grids|=(grids&bit)!=0;grids|=bit;}
  }
  for(unsigned i=0;i<frame.count;i++) {
   if(duplicate_positions) frame.cars[i].known&=~livewire::POSITION;
   if(duplicate_grids) frame.cars[i].known&=~livewire::GRID;
  }
  if(frame.count) frame.state|=livewire::ACTIVE;
  else std::snprintf(frame.diagnostic,sizeof frame.diagnostic,"Vehicles exist, but their object layout checks failed. Live values are unavailable.");
  return frame.count>0;
 }
};
}
