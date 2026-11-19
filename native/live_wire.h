// SPDX-License-Identifier: MIT
#pragma once
#include <cstdint>
#include <cstddef>

namespace livewire {
constexpr unsigned VERSION = 2, CARS = 32, SETTINGS = 21;
// Preserve wire bit values; former readiness bits are unused.
enum State : uint32_t { ACTIVE=1, PAUSED=2, ENDED=32, SESSION_TIMER=128, SESSION_LAPS=256 };
enum Known : uint32_t { NAME=1, POSITION=2, GRID=4, TYRES=8, LAST=16, BEST=32, PIT=64, RESULT=128, LAPS=256, GAP_AHEAD=512, BEST_SECTORS=1024, STINT=2048 };
enum CarFlag : uint32_t { IN_PIT_LANE=2 };
enum SessionKind : uint32_t { SESSION_UNKNOWN=0, PRACTICE=1, QUALIFYING=2, RACE=3, SPRINT=4, TIME_TRIAL=5 };
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
    uint32_t flags;
    uint64_t identity;
    uint32_t laps;
    int32_t gap_ahead_ms;
    uint32_t best_sectors[3];
    uint32_t stint_laps; // former reserved word; valid only with STINT
};
struct Frame {
    char magic[8];
    uint32_t version, size;
    int64_t sequence; // even = stable; aligned for Interlocked operations
    uint64_t session, tick_ms;
    uint32_t state;
    int32_t player;
    // progress: seconds remaining with SESSION_TIMER, or current lap (low 16)
    // and total laps (high 16) with SESSION_LAPS. Clear bits mean unknown.
    uint32_t count, session_type, session_kind, progress;
    char diagnostic[192];
    Car cars[CARS];
};
static_assert(sizeof(Car)==304 && offsetof(Car,identity)==272 && offsetof(Car,laps)==280 && offsetof(Car,best_sectors)==288 && offsetof(Car,stint_laps)==300);
static_assert(offsetof(Frame,sequence)==16 && offsetof(Frame,cars)==256 && sizeof(Frame)==9984);
}
