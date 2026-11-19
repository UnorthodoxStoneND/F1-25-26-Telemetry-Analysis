// SPDX-License-Identifier: MIT
#pragma once
#include "badge_wire.h"
#include "live_wire.h"
#include "swofty_api.h"
#include "live_images.h"
#include <cstring>
#include <cstdio>

// Optional image access only through SwoftyCoreLive; no texture or archive reader.
namespace badges {
struct Publisher {
    using Logger=void(*)(const char*,...);
    struct Receipt {bool seen=false;int reference_result=0,image_result=0;char reference[256]{};};
    Receipt receipts[badgewire::CARS]{},tyre_receipts[badgewire::CARS]{};
    uint64_t reported_session=0;
    badgewire::Frame frame{};
    const SwoftyLiveImages* started=nullptr;
    // Queue catalogue decoding while the session is loading, without publishing
    // driver rows or retaining pixels outside the matched telemetry snapshot.
    void prefetch(const SwoftyLive* api) {
        const auto* images=prepare(api);
        if(!images||!api->available||!api->available()||!api->session||!api->car||!api->team_badge_texture) return;
        SwoftyLiveSession info{};info.version=1;info.size=sizeof info;
        if(api->session(&info)!=SWOFTY_ASSET_OK||info.version<1||info.size<sizeof info||
           !(info.have&SWOFTY_LIVE_SHAVE_CARS)||!info.cars||info.cars>badgewire::CARS) return;
        char requested[badgewire::CARS*2][256]{};unsigned count=0;
        uint8_t rgba[badgewire::PIXELS];
        for(unsigned id=0;id<info.cars;id++) {
            SwoftyLiveCar car{};car.version=1;car.size=sizeof car;
            if(api->car(id,&car)!=SWOFTY_ASSET_OK||car.version<1||car.size<sizeof car||car.vehicle!=id||
               !(car.have&SWOFTY_LIVE_HAVE_IDENT)||!(car.flags&SWOFTY_LIVE_CAR_RACER)) continue;
            for(unsigned kind=0;kind<2;kind++) {
            if(kind&& (api->version<5||!api->tyre_icon_texture)) continue;
            char reference[256]{};
            auto reference_fn=kind?api->tyre_icon_texture:api->team_badge_texture;
            if(reference_fn(id,reference,sizeof reference)!=SWOFTY_ASSET_OK||!reference[0]||
               !std::memchr(reference,0,sizeof reference)) continue;
            bool duplicate=false;
            for(unsigned n=0;n<count;n++) if(!std::strcmp(requested[n],reference)) duplicate=true;
            if(duplicate) continue;
            std::memcpy(requested[count++],reference,sizeof reference);
            uint32_t width=0,height=0;uint64_t revision=0;
            images->image(reference,rgba,sizeof rgba,&width,&height,&revision);
            }
        }
    }
    const SwoftyLiveImages* prepare(const SwoftyLive* api) {
        // Live State is unavailable in menus, but runtime badges are drawn there.
        // Arm capture before those textures are named/rendered; retry until D3D is ready.
        if(!api||api->version<4||!api->images) return nullptr;
        const auto* images=api->images();
        if(!images||images->version<1||!images->start||!images->image) return nullptr;
        if(started!=images) {
            if(images->start()!=SWOFTY_ASSET_OK) return nullptr;
            started=images;
        }
        return images;
    }
    void report(Logger log,unsigned id,const char* reference,int ref_result,int image_result,bool tyre=false) {
        if(!log) return;
        auto& receipt=tyre?tyre_receipts[id]:receipts[id];
        if(receipt.seen&&receipt.reference_result==ref_result&&receipt.image_result==image_result&&
           !std::strcmp(receipt.reference,reference)) return;
        receipt.seen=true;receipt.reference_result=ref_result;receipt.image_result=image_result;
        std::snprintf(receipt.reference,sizeof receipt.reference,"%s",reference);
        log("[telemetry] %s car %u: reference_result=%d image_result=%d reference='%s'",tyre?"tyre image":"badge",id,ref_result,image_result,reference);
    }
    void sample(const SwoftyLive* api,const livewire::Frame& telemetry,Logger log=nullptr) {
        if(reported_session!=telemetry.session) {
            for(auto& receipt:receipts) receipt={};
            for(auto& receipt:tyre_receipts) receipt={};
            reported_session=telemetry.session;
        }
        std::memset(&frame,0,sizeof frame);
        std::memcpy(frame.magic,"USTBADG2",8);
        frame.version=badgewire::VERSION;frame.size=sizeof frame;frame.session=telemetry.session;
        if(!(telemetry.state&livewire::ACTIVE)||(telemetry.state&64)||!api||api->version<4||
           !api->available||!api->available()||!api->team_badge_texture||!api->images) return;
        const auto* images=prepare(api);
        if(!images) return;
        for(unsigned n=0;n<telemetry.count&&n<livewire::CARS;n++) {
            const auto& car=telemetry.cars[n];
            if(car.vehicle>=badgewire::CARS||!car.identity) continue;
            for(unsigned kind=0;kind<2;kind++) {
            if(kind&&(api->version<5||!api->tyre_icon_texture)) continue;
            auto& out=kind?frame.tyres[car.vehicle]:frame.cars[car.vehicle];
            auto reference_fn=kind?api->tyre_icon_texture:api->team_badge_texture;
            int reference_result=reference_fn(car.vehicle,out.reference,sizeof out.reference);
            if(reference_result!=SWOFTY_ASSET_OK||!out.reference[0]||!std::memchr(out.reference,0,sizeof out.reference)) {
                report(log,car.vehicle,"",reference_result,-1,kind!=0);out={};continue;
            }
            uint32_t width=0,height=0;uint64_t revision=0;
            int image_result=images->image(out.reference,out.rgba,sizeof out.rgba,&width,&height,&revision);
            report(log,car.vehicle,out.reference,reference_result,image_result,kind!=0);
            if(image_result!=SWOFTY_ASSET_OK||
               !width||!height||width>64||height>64) {out={};continue;}
            out.identity=car.identity;out.width=width;out.height=height;out.revision=revision;
            }
        }
    }
};
}
