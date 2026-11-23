// SPDX-License-Identifier: MIT
#pragma once
#include "live_wire.h"
#include "readers.h"
#include "swofty_api.h"
#include <cstdio>

namespace analysisteams {
// The published GameModeSystem points at a vector of EntrantRecord rows.
// EntrantRecord is 0x88 bytes. SessionEntrantView exposes the fields it shares.
constexpr size_t entrant_stride=0x88, max_entrants=512;
static_assert(sizeof(swofty::game::SessionEntrantView)<=entrant_stride);

struct Labels {
    struct Entry {uint32_t stat=0; uint64_t retry=0; char name[40]{};};
    Entry entries[livewire::CARS]{};
    size_t used=0;

    void clear() { *this={}; }

    const char* name(const SwoftyDb* db,uint32_t stat,uint64_t now) {
        if(!db || !db->available || !db->open || !db->step || !db->cell || !db->row_name || !db->available())
            return nullptr;
        size_t i=0;
        for(;i<used;i++) if(entries[i].stat==stat) break;
        if(i==used) {
            if(used==livewire::CARS) return nullptr;
            entries[used++].stat=stat;
        }
        auto& entry=entries[i];
        if(entry.name[0] || now<entry.retry) return entry.name;
        entry.retry=now+1000;
        SwoftyDbCursor cursor{};
        int result=db->open("entrants.teams",&cursor);
        for(uint64_t row=0;result==SWOFTY_ASSET_OK&&row<cursor.count&&row<8192;row++,result=db->step(&cursor)) {
            SwoftyDbValue id{};
            if(db->cell(&cursor,"stat_id",&id)!=SWOFTY_ASSET_OK ||
               (id.tag!=SWOFTY_DB_INT && id.tag!=SWOFTY_DB_ID) || id.i!=stat) continue;
            const char* label=db->row_name(&cursor);
            if(label && std::strlen(label)<sizeof entry.name)
                std::memcpy(entry.name,label,std::strlen(label)+1);
            break;
        }
        return entry.name;
    }
};

template<class R> void fill(R& read,uint64_t mode_system,const SwoftyDb* db,Labels& labels,uint64_t now,livewire::Frame& frame) {
    using namespace swofty::game;
    if(!mode_system) return;
    uint32_t wanted=0;
    for(unsigned i=0;i<frame.count;i++) {
        const auto id=frame.cars[i].vehicle;
        if(!frame.analysis_cars[id].team[0]) wanted|=1u<<id;
    }
    if(!wanted) return;

    const auto mode=readers::pointer(read,mode_system+offsetof(GameModeSystem,mode));
    const auto manager=mode?readers::pointer(read,mode+offsetof(GameMode,entrants_manager)):0;
    if(!manager) return;
    const auto begin=readers::pointer(read,manager+offsetof(EntrantsManager,session_begin));
    const auto end=readers::pointer(read,manager+offsetof(EntrantsManager,session_end));
    if(!begin || end<=begin || (end-begin)%entrant_stride || (end-begin)/entrant_stride>max_entrants) return;

    uint32_t stats[livewire::CARS]{},seen=0,duplicate=0;
    for(auto at=begin;at<end;at+=entrant_stride) {
        SessionEntrantView entry{};
        if(!readers::read(read,at,entry)) return;
        const auto id=entry.vehicle_id;
        if(id>=livewire::CARS || !(wanted&(1u<<id))) continue;
        duplicate|=seen&(1u<<id);
        seen|=1u<<id;
        stats[id]=entry.team_stat_id;
    }
    const auto matched=seen&~duplicate;
    for(unsigned id=0;id<livewire::CARS;id++) {
        if(!(matched&(1u<<id))) continue;
        const auto kind=stats[id]&0xff, stat=stats[id]>>16;
        if(stat==0x3fff||stat==0xffff) continue;
        // Team-domain 1 is the user-defined My Team table. A rendered badge
        // does not carry the team's name, so use the known category.
        const char* name=kind==1?"myteam":kind==0?labels.name(db,stat,now):nullptr;
        if(name) std::snprintf(frame.analysis_cars[id].team,sizeof frame.analysis_cars[id].team,"%s",name);
    }
}
}
