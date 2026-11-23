// SPDX-License-Identifier: MIT
#define WIN32_LEAN_AND_MEAN
#define NOMINMAX
#include <windows.h>
#include <cassert>
#include <cstring>
#include <iostream>
#include "proton_transport.h"
int main() {
    wchar_t folder[MAX_PATH]{},path[MAX_PATH]{};
    assert(GetTempPathW(MAX_PATH,folder));
    assert(swprintf_s(path,L"%stelemetry-ipc-test-%lu.ipc",folder,GetCurrentProcessId())>0);
    {
        protontransport::Mapping mapping;assert(mapping.create(path));
        auto* live=static_cast<livewire::Frame*>(mapping.view);
        auto* images=reinterpret_cast<badgewire::Frame*>(static_cast<char*>(mapping.view)+sizeof(*live));
        std::memcpy(live->magic,"USTLIVE6",8);live->size=sizeof(*live);live->session=42;
        std::memcpy(images->magic,"USTBADG2",8);images->size=sizeof(*images);images->session=42;
        // A second read-only mapping sees the same backing bytes and both frames.
        HANDLE file=CreateFileW(path,GENERIC_READ,FILE_SHARE_READ|FILE_SHARE_WRITE|FILE_SHARE_DELETE,
            nullptr,OPEN_EXISTING,FILE_ATTRIBUTE_NORMAL,nullptr);
        assert(file!=INVALID_HANDLE_VALUE);
        HANDLE section=CreateFileMappingW(file,nullptr,PAGE_READONLY,0,0,nullptr);assert(section);
        void* view=MapViewOfFile(section,FILE_MAP_READ,0,0,protontransport::SIZE);assert(view);
        assert(static_cast<livewire::Frame*>(view)->session==42);
        assert(reinterpret_cast<badgewire::Frame*>(static_cast<char*>(view)+sizeof(*live))->session==42);
        UnmapViewOfFile(view);CloseHandle(section);CloseHandle(file);
    }
    assert(GetFileAttributesW(path)==INVALID_FILE_ATTRIBUTES);
    std::cout<<"PASS: Proton backing mapping shares both frames and deletes IPC on last close\n";
}
