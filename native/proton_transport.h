// SPDX-License-Identifier: MIT
#pragma once
#include <windows.h>
#include <cstdio>
#include <cwchar>
#include "live_wire.h"
#include "badge_wire.h"

// A native Linux helper cannot open Wine's named Win32 section objects. Under
// Wine only, use one temporary, shared file mapping beside this DLL instead.
// It contains the same two bounded frames; the normal joint sequence commits
// both. The kernel removes the IPC file when the game's last handle closes.
namespace protontransport {
constexpr size_t SIZE=sizeof(livewire::Frame)+sizeof(badgewire::Frame);
struct Mapping {
    HANDLE file=INVALID_HANDLE_VALUE,section=nullptr;
    void* view=nullptr;
    Mapping()=default;
    Mapping(const Mapping&)=delete;
    Mapping& operator=(const Mapping&)=delete;
    void close() {
        if(view) UnmapViewOfFile(view);
        if(section) CloseHandle(section);
        if(file!=INVALID_HANDLE_VALUE) CloseHandle(file);
        view=nullptr;section=nullptr;file=INVALID_HANDLE_VALUE;
    }
    ~Mapping(){close();}
    bool create(const wchar_t* path) {
        close();
        file=CreateFileW(path,GENERIC_READ|GENERIC_WRITE|DELETE,
            FILE_SHARE_READ|FILE_SHARE_WRITE|FILE_SHARE_DELETE,nullptr,CREATE_NEW,
            FILE_ATTRIBUTE_TEMPORARY|FILE_FLAG_DELETE_ON_CLOSE,nullptr);
        if(file==INVALID_HANDLE_VALUE) return false;
        section=CreateFileMappingW(file,nullptr,PAGE_READWRITE,0,DWORD(SIZE),nullptr);
        view=section?MapViewOfFile(section,FILE_MAP_WRITE,0,0,SIZE):nullptr;
        if(!view){close();return false;}
        return true;
    }
};
inline bool under_wine() {
    auto module=GetModuleHandleW(L"ntdll.dll");
    return module&&GetProcAddress(module,"wine_get_version");
}
inline bool create_for_module(Mapping& mapping,const void* address) {
    HMODULE module=nullptr;wchar_t path[MAX_PATH]{};
    if(!GetModuleHandleExW(GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS|GET_MODULE_HANDLE_EX_FLAG_UNCHANGED_REFCOUNT,
        reinterpret_cast<LPCWSTR>(address),&module)) return false;
    auto length=GetModuleFileNameW(module,path,MAX_PATH);
    if(!length||length>=MAX_PATH-1) return false;
    auto slash=wcsrchr(path,L'\\');if(!slash)return false;
    auto remaining=MAX_PATH-size_t(slash+1-path);
    if(swprintf_s(slash+1,remaining,L".telemetry-live-%lu-%llu.ipc",GetCurrentProcessId(),GetTickCount64())<0) return false;
    return mapping.create(path);
}
}
