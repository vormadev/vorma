const SCROLL_Y_KEY = "__wave_scroll_y";
const SYMBOLS = {
	data_revalidate_fn: Symbol.for("wave-data-revalidate-fn"),
};
const EL_IDS = {
	critical_css: "wave-critical-css",
	non_critical_css: "wave-non-critical-css",
	rebuilding_overlay: "wave-rebuilding-overlay",
};
const REFRESH_PAYLOAD_KEY = {
	change_type: "change_type",
	critical_css: "critical_css",
	non_critical_css_url: "non_critical_css_url",
	build_error: "build_error",
};
const CHANGE_TYPE = {
	show_rebuilding_overlay: "show_rebuilding_overlay",
	hide_rebuilding_overlay: "hide_rebuilding_overlay",
	build_error: "build_error",
	hard_reload: "hard_reload",
	critical_css: "critical_css",
	non_critical_css: "non_critical_css",
	data_revalidate: "data_revalidate",
};
const DEV_REFRESH_EVENTS_PATH_PREFIX = "/wave-dev-refresh-";

function base64_to_utf8(base64) {
	if (!base64) return "";
	const bytes = Uint8Array.from(atob(base64), (m) => m.codePointAt(0) || 0);
	return new TextDecoder().decode(bytes);
}

const wave_msg_prefix = "[wave]:";
const log_info = (...args) => console.info(wave_msg_prefix, ...args);
const log_err = (...args) => console.error(wave_msg_prefix, ...args);

function missing_el_err(el_id, tmpl_method) {
	return new Error(
		`${wave_msg_prefix} expected element with id '#${el_id}' — did you forget to include ${tmpl_method} in your template?`,
	);
}

const scroll_y_val = sessionStorage.getItem(SCROLL_Y_KEY);
if (scroll_y_val) {
	setTimeout(() => {
		sessionStorage.removeItem(SCROLL_Y_KEY);
		log_info("Restoring scroll position");
		window.scrollTo({ top: scroll_y_val, behavior: "smooth" });
	}, 150);
}

const refresh_ws_url = new URL(window.location.href);
refresh_ws_url.protocol =
	window.location.protocol === "https:" ? "wss:" : "ws:";
refresh_ws_url.port = "__REPLACE_ME_WITH_REFRESH_PORT__";
refresh_ws_url.pathname =
	DEV_REFRESH_EVENTS_PATH_PREFIX + "__REPLACE_ME_WITH_REFRESH_TOKEN__";
refresh_ws_url.search = "";
refresh_ws_url.hash = "";

const ws = new WebSocket(refresh_ws_url.toString());
let is_data_revalidating = false;

function upsert_rebuilding_overlay(inner_html, background_color) {
	let el = document.getElementById(EL_IDS.rebuilding_overlay);
	if (!el) {
		el = document.createElement("div");
		el.id = EL_IDS.rebuilding_overlay;
		el.style.display = "flex";
		el.style.position = "fixed";
		el.style.inset = "0";
		el.style.width = "100%";
		el.style.color = "white";
		el.style.textAlign = "center";
		el.style.padding = "10px";
		el.style.zIndex = "1000";
		el.style.fontFamily = "monospace";
		el.style.fontSize = "7vw";
		el.style.fontWeight = "bold";
		el.style.textShadow = "2px 2px 2px #000";
		el.style.justifyContent = "center";
		el.style.alignItems = "center";
		el.style.opacity = "0";
		el.style.transition = "opacity 0.05s";
		document.body.appendChild(el);
		setTimeout(() => {
			el.style.opacity = "1";
		}, 12);
	}
	el.innerHTML = inner_html;
	el.style.backgroundColor = background_color;
	return el;
}

function hide_rebuilding_overlay() {
	if (is_data_revalidating) {
		return;
	}
	document.getElementById(EL_IDS.rebuilding_overlay)?.remove();
}

ws.onmessage = (e) => {
	const parsed = JSON.parse(e.data);
	const change_type = parsed[REFRESH_PAYLOAD_KEY.change_type];
	const non_critical_css_url =
		parsed[REFRESH_PAYLOAD_KEY.non_critical_css_url];
	const critical_css = parsed[REFRESH_PAYLOAD_KEY.critical_css];
	const build_error = parsed[REFRESH_PAYLOAD_KEY.build_error];

	if (change_type === CHANGE_TYPE.show_rebuilding_overlay) {
		log_info("Rebuilding");
		upsert_rebuilding_overlay("Rebuilding...", "#333a");
	}

	if (change_type === CHANGE_TYPE.hide_rebuilding_overlay) {
		hide_rebuilding_overlay();
	}

	if (change_type === CHANGE_TYPE.build_error) {
		log_err("Build error", build_error);
		upsert_rebuilding_overlay("Build error", "#700e");
	}

	if (change_type === CHANGE_TYPE.hard_reload) {
		const y = window.scrollY;
		if (y > 0) {
			sessionStorage.setItem(SCROLL_Y_KEY, y);
		}
		window.location.reload();
	}

	if (change_type === CHANGE_TYPE.non_critical_css) {
		const _old = document.getElementById(EL_IDS.non_critical_css);
		if (!_old) {
			throw missing_el_err(EL_IDS.non_critical_css, "CSSEls");
		}
		const _new = document.createElement("link");
		_new.id = EL_IDS.non_critical_css;
		_new.rel = "stylesheet";
		_new.href = non_critical_css_url;
		_new.onload = () => _old.remove();
		_old.parentNode.insertBefore(_new, _old.nextSibling);
	}

	if (change_type === CHANGE_TYPE.critical_css) {
		const _old = document.getElementById(EL_IDS.critical_css);
		if (!_old) {
			throw missing_el_err(EL_IDS.critical_css, "CSSEls");
		}
		const _new = document.createElement("style");
		_new.id = EL_IDS.critical_css;
		_new.innerHTML = base64_to_utf8(critical_css);
		document.head.replaceChild(_new, _old);
	}

	if (change_type === CHANGE_TYPE.data_revalidate) {
		is_data_revalidating = true;
		if (window[SYMBOLS.data_revalidate_fn]) {
			log_info("Revalidating data");
			Promise.resolve()
				.then(() => window[SYMBOLS.data_revalidate_fn]())
				.then(() => {
					log_info("Data revalidation complete");
				})
				.catch((err) => {
					log_err("Data revalidation failed", err);
				})
				.finally(() => {
					is_data_revalidating = false;
					hide_rebuilding_overlay();
				});
		} else {
			is_data_revalidating = false;
			log_err("Data revalidation function not found");
			hide_rebuilding_overlay();
		}
	}
};

ws.onclose = () => {
	log_info("WebSocket closed (will reload page)");
	window.location.reload();
};

ws.onerror = (e) => {
	log_err("WebSocket error (will reload page)", e);
	ws.onclose = () => {};
	ws.close();
	window.location.reload();
};

window.addEventListener("beforeunload", () => {
	ws.onclose = () => {};
	ws.close();
});
