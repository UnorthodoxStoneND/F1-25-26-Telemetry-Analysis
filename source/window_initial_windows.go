//go:build desktop && windows

package main

/*
#cgo LDFLAGS: -luser32
#include <windows.h>
#include <stdint.h>
#include <stdlib.h>

// A thread-local creation hook affects only our explicitly named window.
// Fyne creates its HWND lazily inside Show; waiting until Show returns is too
// late to prevent the initial appearance on Windows' default monitor.
static HHOOK retro_initial_hook;
static HWND retro_initial_window;
static WCHAR retro_initial_title[256];
static RECT retro_initial_bounds;
static LRESULT CALLBACK retro_initial_proc(int code,WPARAM wp,LPARAM lp) {
 if(code==HCBT_CREATEWND) {
   CBT_CREATEWNDW *created=(CBT_CREATEWNDW*)lp;
   CREATESTRUCTW *cs=created->lpcs;
   if(cs->lpszName&&(uintptr_t)cs->lpszName>0xffff&&!lstrcmpW(cs->lpszName,retro_initial_title)) {
     retro_initial_window=(HWND)wp;
     cs->x=retro_initial_bounds.left;cs->y=retro_initial_bounds.top;
     cs->cx=retro_initial_bounds.right-retro_initial_bounds.left;
     cs->cy=retro_initial_bounds.bottom-retro_initial_bounds.top;
   }
 } else if(code==HCBT_MOVESIZE&&(HWND)wp==retro_initial_window) {
   // Fyne/GLFW can issue an initialization resize before Show completes.
   // Keep it on the chosen bounds until creation/first show has finished.
   *(RECT*)lp=retro_initial_bounds;
 }
 return CallNextHookEx(retro_initial_hook,code,wp,lp);
}
static int retro_initial_begin(const char*title,int x,int y,int width,int height) {
 if(retro_initial_hook)return 0;
 if(!MultiByteToWideChar(CP_UTF8,0,title,-1,retro_initial_title,256))return 0;
 retro_initial_window=NULL;retro_initial_bounds=(RECT){x,y,x+width,y+height};
 retro_initial_hook=SetWindowsHookExW(WH_CBT,retro_initial_proc,NULL,GetCurrentThreadId());
 return retro_initial_hook!=NULL;
}
static void retro_initial_end(void) {
 if(retro_initial_hook)UnhookWindowsHookEx(retro_initial_hook);
 retro_initial_hook=NULL;retro_initial_window=NULL;
}
*/
import "C"

import "unsafe"

func geometryBeforeShow(title string, p windowPlacement) func() {
	text := C.CString(title)
	defer C.free(unsafe.Pointer(text))
	b := p.Bounds
	if C.retro_initial_begin(text, C.int(b.X), C.int(b.Y), C.int(b.W), C.int(b.H)) == 0 {
		return func() {}
	}
	return func() { C.retro_initial_end() }
}
