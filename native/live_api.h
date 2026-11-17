// SPDX-License-Identifier: AGPL-3.0-or-later
#pragma once
#include "swofty_api.h"
#include "live_wire.h"

// Optional cached core snapshot. Never uses the older UDP-backed telemetry API.
namespace liveapi {
struct Snapshot {
 SwoftyLiveSession session{};
 SwoftyLiveCar cars[32]{};
 bool valid=false;
 void load(const SwoftyLive* api,unsigned count) {
  *this={};
  if(!api||api->version<1||!api->available||!api->session||!api->car||!api->seq||count>32||!api->available()) return;
  uint64_t before=0,after=0;
  if(api->seq(&before)!=SWOFTY_ASSET_OK) return;
  session.version=1;session.size=sizeof session;
  if(api->session(&session)!=SWOFTY_ASSET_OK||session.version<1||session.size<sizeof session) return;
  if((session.have&SWOFTY_LIVE_SHAVE_CARS)&&session.cars!=count) return;
  for(unsigned id=0;id<count;id++) {
   auto& c=cars[id];c.version=1;c.size=sizeof c;
   if(api->car(id,&c)!=SWOFTY_ASSET_OK||c.version<1||c.size<sizeof c||c.vehicle!=id) c={};
  }
  valid=api->seq(&after)==SWOFTY_ASSET_OK&&before==after;
 }
 bool player(int& out,unsigned count)const {
  if(!valid||!(session.have&SWOFTY_LIVE_SHAVE_PLAYER)||session.player< -1||session.player>=int(count)) return false;
  out=session.player;return true;
 }
 void apply(unsigned id,livewire::Car& out,unsigned count)const {
  if(!valid||id>=32) return;
  const auto& c=cars[id];
  if((c.have&SWOFTY_LIVE_HAVE_CLASS)&&c.position>0&&c.position<=int(count)) {
   out.position=c.position;out.known|=livewire::POSITION;
  }
  if((c.have&SWOFTY_LIVE_HAVE_TYRE)&&((c.tyre_actual>=16&&c.tyre_actual<=20)||c.tyre_actual==7||c.tyre_actual==8)&&
     ((c.tyre_visual>=16&&c.tyre_visual<=18)||c.tyre_visual==7||c.tyre_visual==8)) {
   out.compound=c.tyre_actual;out.visual=c.tyre_visual;out.known|=livewire::TYRES;
  }
  if((c.have&SWOFTY_LIVE_HAVE_PIT_LANE)&&c.pit_status<=2) {
   out.known|=livewire::PIT;
   if(c.pit_status) out.flags|=livewire::IN_PIT_LANE;
  }
 }
};
}
