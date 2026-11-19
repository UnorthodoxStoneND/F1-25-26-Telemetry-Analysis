// SPDX-License-Identifier: MIT
#pragma once
#include "live_wire.h"
#include <cmath>

namespace stint {
// One bounded observation per car, not a lap log or recorder. The DLL samples
// throughout the session, even when the launcher page/column is hidden.
struct Observation {
    uint64_t identity=0;
    uint32_t base=0,last_lap=0;
    int32_t compound=0;
    float wear[4]{};
    bool base_known=true,lap_known=false,compound_known=false,wear_known=false;
    bool pit_open=false,pit_changed=false,pending_change=false;
};
struct Counter {
    Observation cars[livewire::CARS]{};
    void reset(){for(auto& car:cars) car={};}
    void apply(livewire::Car& car) {
        car.known&=~livewire::STINT;car.stint_laps=0;
        if(car.vehicle>=livewire::CARS||!car.identity) return;
        auto& old=cars[car.vehicle];
        if(old.identity!=car.identity) {
            bool replacement=old.identity!=0;
            old={};old.identity=car.identity;old.base_known=!replacement;
        }
        const bool lap=(car.known&livewire::LAPS)&&car.laps<1000;
        const bool compound=(car.known&livewire::TYRES)!=0;
        const bool pit_known=(car.known&livewire::PIT)!=0;
        const bool in_pit=pit_known&&(car.flags&livewire::IN_PIT_LANE);
        const bool pit_window=pit_known&&(in_pit||old.pit_open);
        if(in_pit&&!old.pit_open) old.pit_changed=false;
        bool wear=(car.wear_known&15)==15;
        for(float value:car.wear) if(!std::isfinite(value)||value<0||value>100) wear=false;

        const bool rewind=lap&&old.lap_known&&car.laps<old.last_lap;
        if(rewind) {
            // A flashback across a stop cannot be reconstructed without history.
            // Lap zero is unambiguous; otherwise wait for a confirmed new stint.
            old.base=0;old.base_known=car.laps==0;old.pending_change=false;
        }
        bool wear_reset=pit_window&&!old.pit_changed&&wear&&old.wear_known;
        // All four tyres must fall by more than floating-point jitter. This
        // catches a new set of the same compound without resetting drive-throughs.
        for(unsigned wheel=0;wheel<4;wheel++)
            if(old.wear[wheel]-car.wear[wheel]<.01f) wear_reset=false;
        const bool changed_compound=compound&&old.compound_known&&car.compound!=old.compound;
        const bool changed=!rewind&&(changed_compound||wear_reset)&&!(pit_window&&old.pit_changed);
        if(changed) {
            old.base_known=false;old.pending_change=true;
            if(pit_window) old.pit_changed=true;
        }
        if(lap&&old.pending_change) {
            old.base=car.laps;old.base_known=true;old.pending_change=false;
        }
        if(lap&&old.base_known&&car.laps>=old.base) {
            car.stint_laps=car.laps-old.base;car.known|=livewire::STINT;
        }
        if(lap){old.last_lap=car.laps;old.lap_known=true;}
        if(compound){old.compound=car.compound;old.compound_known=true;}
        if(wear){for(unsigned wheel=0;wheel<4;wheel++) old.wear[wheel]=car.wear[wheel];old.wear_known=true;}
        if(pit_known){old.pit_open=in_pit;if(!in_pit)old.pit_changed=false;}
    }
};
}
