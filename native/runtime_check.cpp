// SPDX-License-Identifier: AGPL-3.0-or-later
// Build/test preflight only. No game is launched and no system files are changed.
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <winver.h>
#include <cstdio>
#include <vector>

int main(int argc,char** argv) {
    if(argc!=2 && argc!=3) return 1;
    const char* dependency=argc==3?argv[2]:"vcruntime140.dll";
    auto runtime=LoadLibraryExA(dependency,nullptr,LOAD_LIBRARY_SEARCH_SYSTEM32);
    if(!runtime) {
        std::fprintf(stderr,"Microsoft Visual C++ x64 runtime is unavailable (Windows error %lu). Install or repair the latest Microsoft Visual C++ v14 x64 Redistributable, then restart Swofty and the game. See README.md.\n",GetLastError());
        return 2;
    }
    char path[MAX_PATH]{};
    if(!GetModuleFileNameA(runtime,path,MAX_PATH)) return 3;
    const auto size=GetFileVersionInfoSizeA(path,nullptr);
    std::vector<unsigned char> bytes(size);
    VS_FIXEDFILEINFO* version=nullptr;UINT version_size=0;
    if(!size || !GetFileVersionInfoA(path,0,size,bytes.data()) ||
       !VerQueryValueA(bytes.data(),"\\",reinterpret_cast<void**>(&version),&version_size) ||
       version_size<sizeof(*version) || version->dwSignature!=0xfeef04bd) return 3;
    // Swofty uses the same VC++ v14 x64 runtime family without a declared
    // numeric minimum. Let the Windows loader verify the actual import/export
    // compatibility instead of imposing an add-on-only version cutoff.
    auto addon=LoadLibraryExA(argv[1],nullptr,LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|LOAD_LIBRARY_SEARCH_SYSTEM32);
    if(!addon) {
        std::fprintf(stderr,"Native add-on dependency load failed (Windows error %lu). Install or repair the latest Microsoft Visual C++ v14 x64 Redistributable. See README.md.\n",GetLastError());
        return 4;
    }
    auto api=reinterpret_cast<const unsigned*>(GetProcAddress(addon,"SwoftyAddonApi"));
    if(!api || *api!=9 || !GetProcAddress(addon,"SwoftyAddonInit")) return 5;
    std::printf("PASS: system x64 runtime %u.%u.%u.%u; native DLL dependency closure and API 9 exports\n",
                HIWORD(version->dwFileVersionMS),LOWORD(version->dwFileVersionMS),
                HIWORD(version->dwFileVersionLS),LOWORD(version->dwFileVersionLS));
    FreeLibrary(addon);FreeLibrary(runtime);
    return 0;
}
