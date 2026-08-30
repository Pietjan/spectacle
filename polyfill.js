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
})();
