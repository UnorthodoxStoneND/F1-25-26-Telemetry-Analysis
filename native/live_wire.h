// SPDX-License-Identifier: MIT
#pragma once
#include <cstdint>
#include <cstddef>

namespace livewire {
constexpr unsigned VERSION = 1, CARS = 32, SETTINGS = 21;
enum State : uint32_t { ACTIVE=1, PAUSED=2, API_READY=4, DIRECT_READY=8, TIMING_READY=16, ENDED=32 };
enum Known : uint32_t { NAME=1, POSITION=2, GRID=4, TYRES=8, LAST=16, BEST=32, PIT=64, RESULT=128 };
enum CarFlag : uint32_t { PLAYER=1, IN_PIT_LANE=2 };
// Result status occupies bits 8..10 without changing the shared-memory layout.
constexpr unsigned RESULT_SHIFT=8;
constexpr uint32_t RESULT_MASK=7u<<RESULT_SHIFT;
enum Setup : unsigned { FW,RW,DIFF_ON,DIFF_OFF,CAMBER_F,CAMBER_R,TOE_F,TOE_R,SUSP_F,SUSP_R,ARB_F,ARB_R,RIDE_F,RIDE_R,BRAKE_PRESSURE,BRAKE_BIAS,PRESSURE_FL,PRESSURE_FR,PRESSURE_RL,PRESSURE_RR,FUEL };
struct Car {
    uint32_t vehicle, known;
    int32_t position, grid, compound, visual;
    uint32_t last_ms, best_ms, wear_known, setup_known;
    float wear[4]; // RL, RR, FL, FR, percent
    float setup[SETTINGS];
    char name[128];
    uint32_t flags; // bits 16..23: surname UTF-8 byte length when name includes given name
    uint64_t identity;
};
struct Frame {
    char magic[8];
    uint32_t version, size;
    int64_t sequence; // even = stable; aligned for Interlocked operations
    uint64_t session, tick_ms;
    uint32_t state;
    int32_t player;
    uint32_t count, session_type, reserved[2];
    char diagnostic[192];
    Car cars[CARS];
};
static_assert(sizeof(Car)==280 && offsetof(Car,identity)==272);
static_assert(offsetof(Frame,sequence)==16 && offsetof(Frame,cars)==256 && sizeof(Frame)==9216);
}
