//go:build desktop && linux

package main

/*
#cgo LDFLAGS: -lX11 -lXrandr
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/extensions/Xrandr.h>
#include <stdint.h>
#include <string.h>

typedef struct {int x,y,w,h;} GeoRect;
static Display *geo_display;
static int (*geo_previous_error)(Display*,XErrorEvent*);
// A launcher window can disappear between the WM's list and a query. Ignore
// errors only on our dedicated geometry connection, forwarding all others.
static int geo_error(Display*d,XErrorEvent*e) {return d==geo_display?0:(geo_previous_error?geo_previous_error(d,e):0);}
static Display* geo_connection(void) {
 if(!geo_display) {geo_display=XOpenDisplay(NULL);if(geo_display)geo_previous_error=XSetErrorHandler(geo_error);}
 return geo_display;
}
static int geo_property(Display*d,Window w,const char*name,Atom type,unsigned long*out,int cap) {
 Atom actual;int format;unsigned long count,after;unsigned char*data=NULL;
 int result=XGetWindowProperty(d,w,XInternAtom(d,name,False),0,cap,False,type,&actual,&format,&count,&after,&data);
 int n=0;if(result==Success&&actual==type&&format==32&&data){n=count<cap?(int)count:cap;memcpy(out,data,n*sizeof(unsigned long));}
 if(data)XFree(data);return n;
}
static int geo_bounds(Display*d,Window w,GeoRect*out,int*maximized,int*visible) {
 XWindowAttributes a;Window child;int x,y;if(!XGetWindowAttributes(d,w,&a)||!XTranslateCoordinates(d,w,DefaultRootWindow(d),0,0,&x,&y,&child))return 0;
 unsigned long ext[4]={0};geo_property(d,w,"_NET_FRAME_EXTENTS",XA_CARDINAL,ext,4);
 *out=(GeoRect){x-(int)ext[0],y-(int)ext[2],a.width+(int)ext[0]+(int)ext[1],a.height+(int)ext[2]+(int)ext[3]};
 unsigned long states[32];int n=geo_property(d,w,"_NET_WM_STATE",XA_ATOM,states,32);Atom h=XInternAtom(d,"_NET_WM_STATE_MAXIMIZED_HORZ",False),v=XInternAtom(d,"_NET_WM_STATE_MAXIMIZED_VERT",False);
 *maximized=0;for(int i=0;i<n;i++)if(states[i]==h||states[i]==v)*maximized=1;
 *visible=a.map_state==IsViewable;return 1;
}
static int geo_environment(int pid,GeoRect*out,int cap,int*preferred) {
 Display*d=geo_connection();if(!d)return 0;Window root=DefaultRootWindow(d);int count=0;
 int ev,err,major=1,minor=5;
 if(XRRQueryExtension(d,&ev,&err)&&XRRQueryVersion(d,&major,&minor)&&(major>1||(major==1&&minor>=5))) {
   int n=0;XRRMonitorInfo*m=XRRGetMonitors(d,root,True,&n);
   if(m){for(int i=0;i<n&&count<cap;i++)if(m[i].width>0&&m[i].height>0)out[count++]=(GeoRect){m[i].x,m[i].y,m[i].width,m[i].height};XRRFreeMonitors(m);}
 }
 if(!count)out[count++]=(GeoRect){0,0,DisplayWidth(d,DefaultScreen(d)),DisplayHeight(d,DefaultScreen(d))};
 unsigned long desk[1]={0},work[256];geo_property(d,root,"_NET_CURRENT_DESKTOP",XA_CARDINAL,desk,1);int wn=geo_property(d,root,"_NET_WORKAREA",XA_CARDINAL,work,256);
 int at=desk[0]<64?(int)desk[0]*4:0;
 if(wn>=at+4){int wx=(int32_t)work[at],wy=(int32_t)work[at+1],wr=wx+(int)work[at+2],wb=wy+(int)work[at+3];
   for(int i=0;i<count;i++){int x=out[i].x>wx?out[i].x:wx,y=out[i].y>wy?out[i].y:wy,r=out[i].x+out[i].w<wr?out[i].x+out[i].w:wr,b=out[i].y+out[i].h<wb?out[i].y+out[i].h:wb;if(r>x&&b>y)out[i]=(GeoRect){x,y,r-x,b-y};}}
 unsigned long windows[512];int n=geo_property(d,root,"_NET_CLIENT_LIST_STACKING",XA_WINDOW,windows,512);if(!n)n=geo_property(d,root,"_NET_CLIENT_LIST",XA_WINDOW,windows,512);
 GeoRect launcher={0};long long area=0;*preferred=0;
 for(int i=0;i<n;i++){unsigned long owner[1];if(geo_property(d,windows[i],"_NET_WM_PID",XA_CARDINAL,owner,1)!=1||(int)owner[0]!=pid)continue;
   GeoRect r;int max,visible;if(geo_bounds(d,windows[i],&r,&max,&visible)&&(long long)r.w*r.h>area){launcher=r;area=(long long)r.w*r.h;}}
 long long best=0;for(int i=0;i<count;i++){int x=launcher.x>out[i].x?launcher.x:out[i].x,y=launcher.y>out[i].y?launcher.y:out[i].y,r=launcher.x+launcher.w<out[i].x+out[i].w?launcher.x+launcher.w:out[i].x+out[i].w,b=launcher.y+launcher.h<out[i].y+out[i].h?launcher.y+launcher.h:out[i].y+out[i].h;long long hit=r>x&&b>y?(long long)(r-x)*(b-y):0;if(hit>best){best=hit;*preferred=i;}}
 return count;
}
static int geo_read(unsigned long window,GeoRect*out,int*maximized) {
 Display*d=geo_connection();int visible=0;return d&&geo_bounds(d,window,out,maximized,&visible)&&visible;
}
static void geo_state(Display*d,Window w,int enabled) {
 XEvent e;memset(&e,0,sizeof(e));e.xclient.type=ClientMessage;e.xclient.window=w;e.xclient.message_type=XInternAtom(d,"_NET_WM_STATE",False);e.xclient.format=32;
 e.xclient.data.l[0]=enabled?1:0;e.xclient.data.l[1]=XInternAtom(d,"_NET_WM_STATE_MAXIMIZED_HORZ",False);e.xclient.data.l[2]=XInternAtom(d,"_NET_WM_STATE_MAXIMIZED_VERT",False);e.xclient.data.l[3]=1;
 XSendEvent(d,DefaultRootWindow(d),False,SubstructureRedirectMask|SubstructureNotifyMask,&e);
}
static int geo_apply(unsigned long window,GeoRect r,int maximized) {
 Display*d=geo_connection();if(!d)return 0;unsigned long ext[4]={0},wm[1];geo_property(d,window,"_NET_FRAME_EXTENTS",XA_CARDINAL,ext,4);
 int x=r.x+(int)ext[0],y=r.y+(int)ext[2],w=r.w-(int)ext[0]-(int)ext[1],h=r.h-(int)ext[2]-(int)ext[3];if(w<100||h<100)return 0;
 if(!geo_property(d,DefaultRootWindow(d),"_NET_SUPPORTING_WM_CHECK",XA_WINDOW,wm,1)){XMoveResizeWindow(d,window,x,y,w,h);XFlush(d);return 1;}
 geo_state(d,window,0);
 XEvent e;memset(&e,0,sizeof(e));e.xclient.type=ClientMessage;e.xclient.window=window;e.xclient.message_type=XInternAtom(d,"_NET_MOVERESIZE_WINDOW",False);e.xclient.format=32;
 e.xclient.data.l[0]=StaticGravity|(15<<8)|(1<<12);e.xclient.data.l[1]=x;e.xclient.data.l[2]=y;e.xclient.data.l[3]=w;e.xclient.data.l[4]=h;
 int ok=XSendEvent(d,DefaultRootWindow(d),False,SubstructureRedirectMask|SubstructureNotifyMask,&e);if(maximized)geo_state(d,window,1);XFlush(d);return ok;
}
*/
import "C"

import "fyne.io/fyne/v2/driver"

func geometryHandle(context any) uintptr {
	if native, ok := context.(driver.X11WindowContext); ok {
		return native.WindowHandle
	}
	return 0
}
func geometryEnvironment(parentPID int) ([]windowRect, int) {
	var monitors [64]C.GeoRect
	var preferred C.int
	n := int(C.geo_environment(C.int(parentPID), &monitors[0], 64, &preferred))
	out := make([]windowRect, 0, n)
	for i := 0; i < n; i++ {
		r := monitors[i]
		out = append(out, windowRect{int(r.x), int(r.y), int(r.w), int(r.h)})
	}
	return out, int(preferred)
}
func geometryRead(handle uintptr) (windowPlacement, bool, bool) {
	var r C.GeoRect
	var maximized C.int
	if C.geo_read(C.ulong(handle), &r, &maximized) == 0 {
		return windowPlacement{}, false, false
	}
	return windowPlacement{Version: 1, Bounds: windowRect{int(r.x), int(r.y), int(r.w), int(r.h)}, Maximized: maximized != 0}, maximized == 0, true
}
func geometryApply(handle uintptr, p windowPlacement) bool {
	r := C.GeoRect{x: C.int(p.Bounds.X), y: C.int(p.Bounds.Y), w: C.int(p.Bounds.W), h: C.int(p.Bounds.H)}
	maximized := C.int(0)
	if p.Maximized {
		maximized = 1
	}
	return C.geo_apply(C.ulong(handle), r, maximized) != 0
}
