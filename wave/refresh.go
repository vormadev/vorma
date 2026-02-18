package wave

import (
	"fmt"
	"html/template"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/cryptoutil"
)

const defaultRefreshPort = 10000

func (w *Wave) RefreshScript() template.HTML {
	if !w.IsDev() {
		return ""
	}

	port := GetRefreshServerPort()
	if port == 0 {
		port = defaultRefreshPort
	}

	return template.HTML(
		fmt.Sprintf(
			"<script>%s</script>",
			RefreshScriptInnerWithParsedConfig(port, w.cfg),
		),
	)
}

func (w *Wave) RefreshScriptSha256Hash() string {
	if !w.IsDev() {
		return ""
	}

	port := GetRefreshServerPort()
	if port == 0 {
		port = defaultRefreshPort
	}

	hash := cryptoutil.Sha256Hash(
		[]byte(RefreshScriptInnerWithParsedConfig(port, w.cfg)),
	)
	return bytesutil.ToBase64(hash)
}

// RefreshScriptInner returns the raw JavaScript for the refresh script.
// Exported so devserver can serve it via HTTP endpoint.
func RefreshScriptInner(port int) string {
	return refreshScriptInnerWithConfig(
		port,
		refreshScriptConfig{
			browserRevalidateFunctionName:     DefaultBrowserRevalidateFunctionName,
			refreshRebuildingOverlayElementID: DefaultRefreshRebuildingOverlayElementID,
			nonCriticalCSSLinkElementID:       DefaultNonCriticalCSSLinkElementID,
			criticalCSSStyleElementID:         DefaultCriticalCSSStyleElementID,
		},
	)
}

// RefreshScriptInnerWithParsedConfig returns the raw JavaScript for the refresh
// script, resolving runtime browser integration names from ParsedConfig.
func RefreshScriptInnerWithParsedConfig(
	port int,
	parsedConfig *ParsedConfig,
) string {
	return refreshScriptInnerWithConfig(
		port,
		refreshScriptConfig{
			browserRevalidateFunctionName:     parsedConfig.BrowserRevalidateFunctionName(),
			refreshRebuildingOverlayElementID: parsedConfig.RefreshRebuildingOverlayElementID(),
			nonCriticalCSSLinkElementID:       parsedConfig.NonCriticalCSSLinkElementID(),
			criticalCSSStyleElementID:         parsedConfig.CriticalCSSStyleElementID(),
		},
	)
}

type refreshScriptConfig struct {
	browserRevalidateFunctionName     string
	refreshRebuildingOverlayElementID string
	nonCriticalCSSLinkElementID       string
	criticalCSSStyleElementID         string
}

func refreshScriptInnerWithConfig(port int, config refreshScriptConfig) string {
	return fmt.Sprintf(
		refreshScriptTemplate,
		config.refreshRebuildingOverlayElementID,
		port,
		config.nonCriticalCSSLinkElementID,
		config.criticalCSSStyleElementID,
		config.browserRevalidateFunctionName,
	)
}

const refreshScriptTemplate = `
function base64ToUTF8(base64) {
	const bytes = Uint8Array.from(atob(base64), (m) => m.codePointAt(0) || 0);
	return new TextDecoder().decode(bytes);
}
const refreshRebuildingOverlayElementID = %q;
function getCurrentEl() {
	return document.getElementById(refreshRebuildingOverlayElementID);
}
const scrollYKey = "__wave_internal__devScrollY";
const scrollY = sessionStorage.getItem(scrollYKey);
if (scrollY) {
	setTimeout(() => {
		sessionStorage.removeItem(scrollYKey);
		console.info("Wave: Restoring previous scroll position");
		window.scrollTo({ top: scrollY, behavior: "smooth" })
	}, 150);
}
const refreshWebSocketURL = new URL(window.location.href);
refreshWebSocketURL.protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
refreshWebSocketURL.port = String(%d);
refreshWebSocketURL.pathname = "/events";
refreshWebSocketURL.search = "";
refreshWebSocketURL.hash = "";
const ws = new WebSocket(refreshWebSocketURL.toString());
const nonCriticalCSSLinkElementID = %q;
const criticalCSSStyleElementID = %q;
const browserRevalidateFunctionName = %q;
ws.onopen = () => {
	ws.send("ping");
};
ws.onmessage = (e) => {
	const { changeType, criticalCSS, normalCSSURL } = JSON.parse(e.data);
	if (changeType == "rebuilding") {
		console.log("Wave: Rebuilding server...");
		const currentEl = getCurrentEl();
		if (!currentEl) {
			const el = document.createElement("div");
			el.innerHTML = "Rebuilding...";
			el.id = refreshRebuildingOverlayElementID;
			el.style.display = "flex";
			el.style.position = "fixed";
			el.style.inset = "0";
			el.style.width = "100%%";
			el.style.backgroundColor = "#333a";
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
			}, 10);
		}
	}
	if (changeType == "other") {
		const scrollY = window.scrollY;
		if (scrollY > 0) {
			sessionStorage.setItem(scrollYKey, scrollY);
		}
		window.location.reload();
	}
	if (changeType == "normal") {
		const oldLink = document.getElementById(nonCriticalCSSLinkElementID);
		const newLink = document.createElement("link");
		newLink.id = nonCriticalCSSLinkElementID;
		newLink.rel = "stylesheet";
		newLink.href = normalCSSURL;
		if (oldLink && oldLink.parentNode) {
			newLink.onload = () => oldLink.remove();
			oldLink.parentNode.insertBefore(newLink, oldLink.nextSibling);
		} else {
			document.head.appendChild(newLink);
		}
	}
	if (changeType == "critical") {
		const oldStyle = document.getElementById(criticalCSSStyleElementID);
		const newStyle = document.createElement("style");
		newStyle.id = criticalCSSStyleElementID;
		newStyle.innerHTML = base64ToUTF8(criticalCSS);
		if (oldStyle && oldStyle.parentNode) {
			oldStyle.parentNode.replaceChild(newStyle, oldStyle);
		} else {
			document.head.appendChild(newStyle);
		}
	}
	if (changeType == "revalidate") {
		console.log("Wave: Revalidating...");
		const el = getCurrentEl();
		const revalidateFunction = window[browserRevalidateFunctionName];
		if (typeof revalidateFunction === "function") {
			Promise.resolve(revalidateFunction()).then(() => {
				console.log("Wave: Revalidated");
				el?.remove();
			}).catch((error) => {
				console.error("Wave: Revalidate failed", error);
				el?.remove();
			});
		} else {
			console.error("No revalidate function found", browserRevalidateFunctionName);
			el?.remove();
		}
	}
};
ws.onclose = () => {
	console.log("Wave: WebSocket closed");
	window.location.reload();
};
ws.onerror = (e) => {
	console.log("Wave: WebSocket error", e);
	ws.close();
	window.location.reload();
};
window.addEventListener("beforeunload", () => {
	ws.onclose = () => {};
	ws.close();
});
`
