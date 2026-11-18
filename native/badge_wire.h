// SPDX-License-Identifier: MIT
#pragma once
#include <cstdint>
#include <cstddef>

namespace badgewire {
constexpr uint32_t VERSION=1,CARS=32,PIXELS=64*64*4;
struct Image {
    uint64_t identity,revision;
    uint32_t width,height;
    char reference[256];
    uint8_t rgba[PIXELS];
};
struct Frame {
    char magic[8];
    uint32_t version,size;
    int64_t sequence;
    uint64_t session;
    Image cars[CARS];
};
static_assert(sizeof(Image)==16664&&offsetof(Image,rgba)==280);
static_assert(offsetof(Frame,sequence)==16&&offsetof(Frame,cars)==32&&sizeof(Frame)==533280);
}
