package ui

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// RunHarness drives the UI headlessly for automated testing, controlled over
// HTTP on addr (loopback only). Endpoints (GET, query params):
//
//	/nav?path=/album/<id>          navigate
//	/shot?out=/tmp/x.png           wait until loads settle, save a screenshot
//	/wait                          wait until loads settle
//	/click?x=&y=[&button=right][&double=1]
//	/clicktext?text=Show all[&n=0][&button=right]   click the n-th element whose label contains text
//	/move?x=&y=                    hover
//	/drag?x1=&y1=&x2=&y2=       press at 1, drag to 2, release
//	/scroll?x=&y=&dy=              wheel scroll (px, positive = down)
//	/key?name=Space[&mods=ctrl,shift,alt]
//	/type?text=hello               type into the focused field
//	/tree[?q=text]                 semantic tree (labels and bounds) as JSON
//	/calls                         account/playback calls the sandbox skipped
//	/size?w=1280&h=800[&scale=1]   resize the virtual window
//
// Coordinates are in pixels of the virtual window (scale = px per dp).
func RunHarness(addr string, w, h int, scale float32) error {
	if !strings.HasPrefix(addr, "127.0.0.1:") && !strings.HasPrefix(addr, "localhost:") {
		return fmt.Errorf("harness must listen on loopback, got %q", addr)
	}
	hr := &harness{size: image.Pt(w, h), scale: scale, cmds: make(chan func(), 1), wake: make(chan struct{}, 1)}
	inv := func() {
		select {
		case hr.wake <- struct{}{}:
		default:
		}
	}
	invalidate.Store(&inv)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go http.Serve(ln, hr.mux())
	Log.Info("harness listening", "addr", addr)
	hr.loop()
	return nil
}

type harness struct {
	size  image.Point
	scale float32
	win   *headless.Window
	r     input.Router
	ops   op.Ops
	cmds  chan func()
	wake  chan struct{}
	now   time.Time
}

func (hr *harness) loop() {
	runtime.LockOSThread()
	var err error
	hr.win, err = headless.NewWindow(hr.size.X, hr.size.Y)
	if err != nil {
		Log.Error("headless window", "err", err)
		os.Exit(1)
	}
	hr.frame()
	for {
		var timer <-chan time.Time
		if t, ok := hr.r.WakeupTime(); ok {
			d := time.Until(t)
			if t.IsZero() {
				d = 0
			}
			timer = time.After(max(d, 16*time.Millisecond))
		}
		select {
		case f := <-hr.cmds:
			f()
		case <-hr.wake:
			hr.frame()
		case <-timer:
			hr.frame()
		}
	}
}

func (hr *harness) frame() {
	hr.now = time.Now()
	hr.ops.Reset()
	gtx := layout.Context{
		Ops:         &hr.ops,
		Now:         hr.now,
		Metric:      unit.Metric{PxPerDp: hr.scale, PxPerSp: hr.scale},
		Constraints: layout.Exact(hr.size),
		Source:      hr.r.Source(),
	}
	Frame(gtx)
	hr.r.Frame(&hr.ops)
	if err := hr.win.Frame(&hr.ops); err != nil {
		Log.Error("headless frame", "err", err)
	}
}

// do runs f on the UI goroutine and waits for it.
func (hr *harness) do(f func()) {
	done := make(chan struct{})
	hr.cmds <- func() {
		f()
		close(done)
	}
	<-done
}

// settle runs frames until async work and images are done and nothing
// changed for a few frames, or the timeout passes.
func (hr *harness) settle(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	quiet := 0
	for time.Now().Before(deadline) && quiet < 3 {
		busy := false
		hr.do(func() {
			hr.frame()
			postMu.Lock()
			busy = len(posted) > 0
			postMu.Unlock()
			busy = busy || inflight.Load() > 0 || imagesPending() > 0
		})
		if busy {
			quiet = 0
		} else {
			quiet++
		}
		time.Sleep(30 * time.Millisecond)
	}
}

func (hr *harness) pointer(kind pointer.Kind, pos f32.Point, btn pointer.Buttons) {
	hr.r.Queue(pointer.Event{Kind: kind, Source: pointer.Mouse, Position: pos, Buttons: btn, Time: time.Duration(time.Now().UnixNano())})
	hr.frame()
}

func (hr *harness) click(pos f32.Point, right, double bool) {
	btn := pointer.ButtonPrimary
	if right {
		btn = pointer.ButtonSecondary
	}
	n := 1
	if double {
		n = 2
	}
	hr.do(func() { hr.pointer(pointer.Move, pos, 0) })
	for i := 0; i < n; i++ {
		hr.do(func() { hr.pointer(pointer.Press, pos, btn) })
		time.Sleep(20 * time.Millisecond)
		hr.do(func() { hr.pointer(pointer.Release, pos, 0) })
		time.Sleep(20 * time.Millisecond)
	}
}

type node struct {
	Label  string `json:"label,omitempty"`
	Desc   string `json:"desc,omitempty"`
	Class  string `json:"class,omitempty"`
	Bounds [4]int `json:"bounds"`
	Off    bool   `json:"disabled,omitempty"`
}

func (hr *harness) nodes(q string) []node {
	var out []node
	hr.do(func() {
		// AppendSemantics returns every node in a flat list (children included).
		walk := func(ns []input.SemanticNode) {
			for _, n := range ns {
				d := n.Desc
				if (d.Label != "" || d.Description != "") && d.Bounds.Dx() > 0 && d.Bounds.Dy() > 0 && d.Bounds.Overlaps(image.Rectangle{Max: hr.size}) {
					text := strings.ToLower(d.Label + " " + d.Description)
					if q == "" || strings.Contains(text, strings.ToLower(q)) {
						b := d.Bounds
						cls := ""
						if d.Class != 0 {
							cls = d.Class.String()
						}
						out = append(out, node{d.Label, d.Description, cls, [4]int{b.Min.X, b.Min.Y, b.Max.X, b.Max.Y}, d.Disabled})
					}
				}
			}
		}
		walk(hr.r.AppendSemantics(nil))
	})
	return out
}

func (hr *harness) mux() *http.ServeMux {
	m := http.NewServeMux()
	num := func(r *http.Request, k string) float32 {
		v, _ := strconv.ParseFloat(r.URL.Query().Get(k), 32)
		return float32(v)
	}
	ok := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}
	m.HandleFunc("/nav", func(w http.ResponseWriter, r *http.Request) {
		hr.do(func() { Navigate(r.URL.Query().Get("path")) })
		hr.settle(15 * time.Second)
		ok(w, "ok")
	})
	m.HandleFunc("/wait", func(w http.ResponseWriter, r *http.Request) {
		hr.settle(15 * time.Second)
		ok(w, "ok")
	})
	m.HandleFunc("/shot", func(w http.ResponseWriter, r *http.Request) {
		hr.settle(15 * time.Second)
		out := r.URL.Query().Get("out")
		var err error
		hr.do(func() {
			hr.frame()
			img := image.NewRGBA(image.Rectangle{Max: hr.size})
			if err = hr.win.Screenshot(img); err != nil {
				return
			}
			var f *os.File
			if f, err = os.Create(out); err != nil {
				return
			}
			defer f.Close()
			err = png.Encode(f, img)
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		ok(w, out)
	})
	m.HandleFunc("/click", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		hr.click(f32.Pt(num(r, "x"), num(r, "y")), q.Get("button") == "right", q.Get("double") == "1")
		hr.settle(10 * time.Second)
		ok(w, "ok")
	})
	m.HandleFunc("/clicktext", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		ns := hr.nodes(q.Get("text"))
		n, _ := strconv.Atoi(q.Get("n"))
		if n >= len(ns) {
			http.Error(w, fmt.Sprintf("%d matches for %q", len(ns), q.Get("text")), 404)
			return
		}
		b := ns[n].Bounds
		hr.click(f32.Pt(float32(b[0]+b[2])/2, float32(b[1]+b[3])/2), q.Get("button") == "right", q.Get("double") == "1")
		hr.settle(10 * time.Second)
		ok(w, ns[n])
	})
	m.HandleFunc("/move", func(w http.ResponseWriter, r *http.Request) {
		hr.do(func() { hr.pointer(pointer.Move, f32.Pt(num(r, "x"), num(r, "y")), 0) })
		time.Sleep(700 * time.Millisecond) // let hover animations and tooltips finish
		hr.settle(5 * time.Second)
		ok(w, "ok")
	})
	m.HandleFunc("/drag", func(w http.ResponseWriter, r *http.Request) {
		p1, p2 := f32.Pt(num(r, "x1"), num(r, "y1")), f32.Pt(num(r, "x2"), num(r, "y2"))
		btn := pointer.ButtonPrimary
		hr.do(func() { hr.pointer(pointer.Move, p1, 0) })
		hr.do(func() { hr.pointer(pointer.Press, p1, btn) })
		for i := 1; i <= 8; i++ {
			// The router turns moves with a button down into drags.
			p := p1.Add(p2.Sub(p1).Mul(float32(i) / 8))
			hr.do(func() { hr.pointer(pointer.Move, p, btn) })
			time.Sleep(20 * time.Millisecond)
		}
		hr.do(func() { hr.pointer(pointer.Release, p2, 0) })
		hr.settle(10 * time.Second)
		ok(w, "ok")
	})
	m.HandleFunc("/scroll", func(w http.ResponseWriter, r *http.Request) {
		pos := f32.Pt(num(r, "x"), num(r, "y"))
		hr.do(func() {
			hr.pointer(pointer.Move, pos, 0)
			hr.r.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: pos, Scroll: f32.Pt(0, num(r, "dy")), Time: time.Duration(time.Now().UnixNano())})
			hr.frame()
		})
		hr.settle(5 * time.Second)
		ok(w, "ok")
	})
	m.HandleFunc("/key", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var mods key.Modifiers
		for _, s := range strings.Split(q.Get("mods"), ",") {
			switch s {
			case "ctrl":
				mods |= key.ModCtrl
			case "shift":
				mods |= key.ModShift
			case "alt":
				mods |= key.ModAlt
			}
		}
		name := key.Name(q.Get("name"))
		hr.do(func() {
			hr.r.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
			hr.frame()
			hr.r.Queue(key.Event{Name: name, Modifiers: mods, State: key.Release})
			hr.frame()
		})
		hr.settle(5 * time.Second)
		ok(w, "ok")
	})
	m.HandleFunc("/type", func(w http.ResponseWriter, r *http.Request) {
		text := r.URL.Query().Get("text")
		hr.do(func() {
			st := hr.r.EditorState()
			hr.r.Queue(key.EditEvent{Range: st.Selection.Range, Text: text})
			hr.frame()
		})
		hr.settle(10 * time.Second)
		ok(w, "ok")
	})
	m.HandleFunc("/tree", func(w http.ResponseWriter, r *http.Request) {
		ok(w, hr.nodes(r.URL.Query().Get("q")))
	})
	m.HandleFunc("/calls", func(w http.ResponseWriter, r *http.Request) {
		ok(w, B.SandboxCalls())
	})
	m.HandleFunc("/size", func(w http.ResponseWriter, r *http.Request) {
		nw, nh := int(num(r, "w")), int(num(r, "h"))
		var err error
		hr.do(func() {
			if s := num(r, "scale"); s > 0 {
				hr.scale = s
			}
			if nw > 0 && nh > 0 {
				hr.win.Release()
				hr.size = image.Pt(nw, nh)
				hr.win, err = headless.NewWindow(nw, nh)
			}
			if err == nil {
				hr.frame()
			}
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		hr.settle(10 * time.Second)
		ok(w, "ok")
	})
	return m
}
