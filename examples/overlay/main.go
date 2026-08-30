// Command overlay demonstrates overlay webviews: a transparent
// full-window layer above a content view, with pointer input limited to
// a 56px rail via SetInputRegions. Tooltips overhang the rail into the
// content area; a modal expands the input region to the whole window.
package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pietjan/spectacle"
)

const railDIP = 56

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	b, err := spectacle.New(spectacle.Config{
		ID:      "spectacle-overlay-demo",
		Name:    "Overlay Demo",
		Comment: "spectacle overlay and input-region demo",
		DataDir: filepath.Join(cache, "spectacle-overlay-demo"),
		Debug:   os.Getenv("OVERLAY_DEMO_DEBUG") != "",
	})
	if err != nil {
		return err
	}
	win, err := b.NewWindow("Overlay Demo", spectacle.Rect{W: 900, H: 600})
	if err != nil {
		return err
	}
	content, err := b.NewWebView(win, "")
	if err != nil {
		return err
	}
	content.OnMessage(func(raw string) { log.Printf("content message: %s", raw) })
	content.NavigateHTML(contentHTML)
	overlay, err := b.NewWebView(win, "", spectacle.Overlay())
	if err != nil {
		return err
	}
	overlay.NavigateHTML(overlayHTML)

	var w, h, dpi int
	modal := false
	apply := func() {
		full := spectacle.Rect{W: w, H: h}
		content.SetBounds(full)
		overlay.SetBounds(full)
		if modal {
			overlay.SetInputRegions([]spectacle.Rect{full})
		} else {
			overlay.SetInputRegions([]spectacle.Rect{{W: railDIP * dpi / 96, H: h}})
		}
	}
	win.OnResize(func(cw, ch, cdpi int) {
		log.Printf("resize %dx%d dpi=%d", cw, ch, cdpi)
		w, h, dpi = cw, ch, cdpi
		apply()
	})
	overlay.OnMessage(func(raw string) {
		log.Printf("overlay message: %s", raw)
		var msg struct {
			Modal  *bool `json:"modal"`
			Resize []int `json:"resize"`
		}
		if json.Unmarshal([]byte(raw), &msg) != nil {
			return
		}
		if len(msg.Resize) == 2 {
			win.SetBounds(spectacle.Rect{W: msg.Resize[0], H: msg.Resize[1]})
			return
		}
		if msg.Modal == nil {
			return
		}
		modal = *msg.Modal
		apply()
		if modal {
			overlay.Focus()
		}
	})

	win.Show()
	return b.Run()
}

// The layer underneath: proves clicks and hover land through the
// overlay's transparent area.
const contentHTML = `<!doctype html><html><head><style>
  html,body{margin:0;height:100%;font-family:sans-serif}
  body{background:linear-gradient(135deg,#1e293b,#0f172a);color:#e2e8f0;
       display:flex;flex-direction:column;align-items:center;justify-content:center;gap:1rem}
  button{font-size:1.2rem;padding:.6rem 1.4rem;border-radius:.5rem;border:0;cursor:pointer}
  #pos{color:#64748b}
</style></head><body>
  <h1>content layer</h1>
  <button onclick="this.textContent='clicked '+(++n)+'×'">click me</button>
  <div id="pos">hover shows coordinates here</div>
  <script>
    let n=0;
    addEventListener('mousemove',e=>{
      pos.textContent=e.clientX+', '+e.clientY;
      if (!window.__t || Date.now()-window.__t > 500) {
        window.__t=Date.now(); console.log('content: move '+e.clientX+','+e.clientY+' win '+innerWidth+'x'+innerHeight);
      }
    });
    addEventListener('click',e=>console.log('content: click '+e.clientX+','+e.clientY));
    // Console output is invisible on Windows; mirror events over the bridge.
    const say=m=>window.chrome?.webview?.postMessage({log:m});
    addEventListener('mousedown',e=>say('down '+e.button+' '+e.clientX+','+e.clientY));
    addEventListener('mouseup',e=>say('up '+e.button+' '+e.clientX+','+e.clientY));
    addEventListener('click',e=>say('click '+e.clientX+','+e.clientY));
    addEventListener('keydown',e=>say('key '+e.key));
    addEventListener('wheel',e=>say('wheel '+e.deltaY));
  </script>
</body></html>`

// The overlay: a 56px rail with overhanging tooltips, plus a modal.
// Everything outside the rail is transparent and (rail-only input
// region) click-through.
const overlayHTML = `<!doctype html><html><head><style>
  html,body{margin:0;height:100%;font-family:sans-serif}
  #rail{position:fixed;inset:0 auto 0 0;width:56px;background:#27272ae6;
        display:flex;flex-direction:column;align-items:center;gap:.5rem;padding-top:.5rem}
  .item{position:relative;width:40px;height:40px;border-radius:.5rem;display:flex;
        align-items:center;justify-content:center;font-size:1.4rem;cursor:pointer}
  .item:hover{background:#3f3f46}
  .tip{pointer-events:none;position:absolute;left:calc(100% + 8px);top:50%;
       transform:translateY(-50%);white-space:nowrap;background:#18181b;color:#fff;
       font-size:.75rem;padding:.25rem .5rem;border-radius:.375rem;
       border:1px solid #ffffff1a;visibility:hidden;opacity:0;transition:opacity .1s}
  .item:hover .tip{visibility:visible;opacity:1}
  #backdrop{position:fixed;inset:0;background:#0008;display:none;
            align-items:center;justify-content:center}
  #backdrop.open{display:flex}
  #dialog{background:#27272a;color:#e4e4e7;border-radius:.75rem;padding:1.5rem;width:320px}
  #dialog button{margin-top:1rem;padding:.4rem 1rem;border-radius:.4rem;border:0;cursor:pointer}
</style></head><body>
  <div id="rail">
    <div class="item">💬<span class="tip">A tooltip overhanging the rail</span></div>
    <div class="item" onclick="window.chrome.webview.postMessage({resize:[innerWidth>700?[600,400]:[900,600]][0]})">📧<span class="tip">Toggle window size</span></div>
    <div class="item" onclick="setModal(true)">⚙️<span class="tip">Open the modal</span></div>
  </div>
  <div id="backdrop" onclick="if(event.target===this)setModal(false)">
    <div id="dialog">
      <h2>modal on the overlay</h2>
      <p>The backdrop dims the content view underneath; input everywhere
         belongs to the overlay while this is open.</p>
      <button onclick="setModal(false)">Close</button>
    </div>
  </div>
  <script>
    function setModal(open){
      backdrop.classList.toggle('open', open);
      try {
        window.chrome.webview.postMessage({modal: open});
        console.log('overlay: posted modal='+open);
      } catch (e) {
        console.log('overlay: postMessage FAILED: '+e);
      }
    }
    for (const el of document.querySelectorAll('.item')) {
      el.addEventListener('mouseenter',()=>console.log('overlay: enter '+el.textContent.trim()));
      el.addEventListener('mouseleave',()=>console.log('overlay: leave '+el.textContent.trim()));
    }
    addEventListener('mousemove',e=>{
      if (!window.__t || Date.now()-window.__t > 500) {
        window.__t=Date.now(); console.log('overlay: move '+e.clientX+','+e.clientY);
      }
    });
    addEventListener('click',e=>console.log('overlay: click '+e.clientX+','+e.clientY+' on '+(e.target.id||e.target.className||e.target.tagName)));
    let hovered=-1;
    setInterval(()=>{
      const n=document.querySelectorAll('.item:hover').length;
      if(n!==hovered){hovered=n;console.log('overlay: hoverCount '+n);}
    },250);
  </script>
</body></html>`
