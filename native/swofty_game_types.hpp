// SPDX-License-Identifier: LicenseRef-PolyForm-Noncommercial-1.0.0
// Upstream generated type declarations, selected without changing their layouts.
// Source: SwoftyLauncher v1.9.76, b5ca25d4e722ba76ef5ad4269c324dd42fb4f74c.
// Required Notice: Copyright Swofty Pty Ltd (ACN 665 787 244)
// License: PolyForm Noncommercial 1.0.0; see SWOFTY-LAYOUTS-LICENSE.txt.
#pragma once
#include <cstddef>
#include <cstdint>
namespace swofty::game {
struct NeRobinHoodMapU64;
struct GameVehicleProgressSystem;
struct GameManagerLive {
    uint8_t _pad_0x0[0x30];
    NeRobinHoodMapU64 *systems_map; // L2 experimental
    uint8_t _pad_0x38[0x298];
    GameVehicleProgressSystem *progress_system; // L2 experimental
};
static_assert(offsetof(GameManagerLive, systems_map) == 0x30);
static_assert(offsetof(GameManagerLive, progress_system) == 0x2d0);

struct NeRobinHoodMapU64 {
    uint8_t _pad_0x0[0x10];
    uint64_t hash_seed; // L2 experimental
    uint8_t *slots; // L2 experimental
    uint8_t *meta; // L2 experimental
    uint64_t count; // L2 experimental
    uint64_t mask; // L2 experimental
    uint64_t max_count; // L2 experimental
    uint32_t step; // L2 experimental
    uint32_t shift; // L2 experimental
};
static_assert(offsetof(NeRobinHoodMapU64, hash_seed) == 0x10);
static_assert(offsetof(NeRobinHoodMapU64, slots) == 0x18);
static_assert(offsetof(NeRobinHoodMapU64, meta) == 0x20);
static_assert(offsetof(NeRobinHoodMapU64, count) == 0x28);
static_assert(offsetof(NeRobinHoodMapU64, mask) == 0x30);
static_assert(offsetof(NeRobinHoodMapU64, max_count) == 0x38);
static_assert(offsetof(NeRobinHoodMapU64, step) == 0x40);
static_assert(offsetof(NeRobinHoodMapU64, shift) == 0x44);

struct CarLive {
    char *teleport_tag; // L2 experimental
    int32_t teleport_priority; // L2 experimental
    uint8_t _pad_0xc[0x4];
    float teleport_prev_mtx[16]; // L2 experimental
    float teleport_new_mtx[16]; // L2 experimental
    float teleport_dt; // L2 experimental
    uint32_t teleport_flags; // L2 experimental
    uint8_t _pad_0x98[0x50];
    uint8_t vehicle_id; // L3 stable
    uint8_t _pad_0xe9[0x2af];
    uint32_t system_count; // L3 stable
    uint8_t _pad_0x39c[0x104];
    void *upgrades_allocator; // L2 experimental
    uint8_t _pad_0x4a8[0x88];
    void *engine; // L3 stable
    void *gearbox; // L3 stable
    void *fuel_tank; // L3 stable
    void *fuel_ecu; // L3 stable
    void *brake_ecu; // L3 stable
    void *chassis; // L3 stable
    void *steering; // L3 stable
    void *core_aero; // L3 stable
    void *f1_aero; // L3 stable
    void *differential; // L3 stable
    void *suspension; // L3 stable
    uint8_t _pad_0x588[0x8];
    void *f1_tuning; // L3 stable
    void *f1_assists; // L3 stable
    void *turbo; // L3 stable
    void *ers; // L3 stable
    uint8_t _pad_0x5b0[0x18];
    void *f1_wing_damage; // L2 experimental
    void *f1_suspension_damage; // L2 experimental
    uint8_t _pad_0x5d8[0x9968];
    uint32_t handling_version; // L3 stable
};
static_assert(offsetof(CarLive, teleport_tag) == 0x0);
static_assert(offsetof(CarLive, teleport_priority) == 0x8);
static_assert(offsetof(CarLive, teleport_prev_mtx) == 0x10);
static_assert(offsetof(CarLive, teleport_new_mtx) == 0x50);
static_assert(offsetof(CarLive, teleport_dt) == 0x90);
static_assert(offsetof(CarLive, teleport_flags) == 0x94);
static_assert(offsetof(CarLive, vehicle_id) == 0xe8);
static_assert(offsetof(CarLive, system_count) == 0x398);
static_assert(offsetof(CarLive, upgrades_allocator) == 0x4a0);
static_assert(offsetof(CarLive, engine) == 0x530);
static_assert(offsetof(CarLive, gearbox) == 0x538);
static_assert(offsetof(CarLive, fuel_tank) == 0x540);
static_assert(offsetof(CarLive, fuel_ecu) == 0x548);
static_assert(offsetof(CarLive, brake_ecu) == 0x550);
static_assert(offsetof(CarLive, chassis) == 0x558);
static_assert(offsetof(CarLive, steering) == 0x560);
static_assert(offsetof(CarLive, core_aero) == 0x568);
static_assert(offsetof(CarLive, f1_aero) == 0x570);
static_assert(offsetof(CarLive, differential) == 0x578);
static_assert(offsetof(CarLive, suspension) == 0x580);
static_assert(offsetof(CarLive, f1_tuning) == 0x590);
static_assert(offsetof(CarLive, f1_assists) == 0x598);
static_assert(offsetof(CarLive, turbo) == 0x5a0);
static_assert(offsetof(CarLive, ers) == 0x5a8);
static_assert(offsetof(CarLive, f1_wing_damage) == 0x5c8);
static_assert(offsetof(CarLive, f1_suspension_damage) == 0x5d0);
static_assert(offsetof(CarLive, handling_version) == 0x9f40);

}

