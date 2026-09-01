(() => {
	if (window.chrome && window.chrome.webview) return;
	const native = window.webkit.messageHandlers.spectacle;
	const listeners = new Set();
	const webview = {
		postMessage: (obj) => native.postMessage(JSON.stringify(obj)),
		addEventListener: (type, fn) => { if (type === "message") listeners.add(fn); },
		removeEventListener: (type, fn) => { listeners.delete(fn); },
		__deliver: (data) => {
			const e = { data };
			for (const fn of [...listeners]) { try { fn(e); } catch (err) {} }
		},
	};
	window.chrome = Object.assign(window.chrome || {}, { webview });

	// WebKitGTK never populates the paste event's clipboardData for
	// images — types arrive empty even for a clean image/png offer — so
	// pages that attach images from their paste handler get nothing. When
	// a trusted paste carries no data at all, ask the Go side to read the
	// clipboard as PNG and replay the paste with a real File. The
	// synthetic event is untrusted, which the major web apps accept; if
	// the page ignores it, fall back to the editing insert WebKit's
	// default action would have done. Main frame only: the reply
	// evaluates there.
	const clip = window.webkit.messageHandlers.spectacleClip;
	if (clip && window === window.top) {
		let seq = 0;
		const pending = new Map();
		window.__spectaclePaste = (id, b64) => {
			const target = pending.get(id);
			pending.delete(id);
			if (!target || !b64) return;
			const bin = atob(b64);
			const buf = new Uint8Array(bin.length);
			for (let i = 0; i < bin.length; i++) buf[i] = bin.charCodeAt(i);
			const file = new File([buf], "image.png", { type: "image/png" });
			const dt = new DataTransfer();
			dt.items.add(file);
			const e = new ClipboardEvent("paste", { clipboardData: dt, bubbles: true, cancelable: true });
			target.dispatchEvent(e);
			if (!e.defaultPrevented && document.activeElement && document.activeElement.isContentEditable) {
				document.execCommand("insertImage", false, URL.createObjectURL(file));
			}
		};
		window.addEventListener("paste", (e) => {
			if (!e.isTrusted || !e.clipboardData || e.clipboardData.types.length) return;
			e.stopImmediatePropagation();
			e.preventDefault();
			pending.set(++seq, e.target);
			clip.postMessage(String(seq));
		}, true);
	}
})();
