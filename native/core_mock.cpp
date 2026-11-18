// SPDX-License-Identifier: MIT
// Host-test core only. Never loaded into the game or included as a runtime DLL.
#include "swofty_api.h"
namespace {
int readable(const void* p,size_t n){return p && n ? 1:0;}
SwoftyMem memory={1,readable,nullptr};
}
extern "C" __declspec(dllexport) const SwoftyMem* SwoftyCoreMem(){return &memory;}
