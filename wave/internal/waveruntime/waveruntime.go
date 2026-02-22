package waveruntime

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/vormadev/vorma/kit/htmlutil"
)

// RefreshScriptConfig defines runtime browser integration names used by the
// refresh script.
type RefreshScriptConfig struct {
	BrowserRevalidateFunctionName     string
	RefreshRebuildingOverlayElementID string
	NonCriticalCSSLinkElementID       string
	CriticalCSSStyleElementID         string
}

// BuildRefreshScript returns the raw JavaScript body for Wave's refresh script.
func BuildRefreshScript(
	port int,
	config RefreshScriptConfig,
) string {
	return fmt.Sprintf(
		refreshScriptTemplate,
		config.RefreshRebuildingOverlayElementID,
		port,
		config.NonCriticalCSSLinkElementID,
		config.CriticalCSSStyleElementID,
		config.BrowserRevalidateFunctionName,
	)
}

// BuildCriticalCSSStyleElement renders a critical-css <style> tag and returns
// both the rendered HTML and its CSP SHA-256 hash.
func BuildCriticalCSSStyleElement(
	content string,
	criticalCSSStyleElementID string,
) (
	renderedElement template.HTML,
	sha256Hash string,
	err error,
) {
	element := htmlutil.Element{
		Tag: "style",
		AttributesKnownSafe: map[string]string{
			"id": criticalCSSStyleElementID,
		},
		DangerousInnerHTML: "\n" + content,
	}

	sha256Hash, err = htmlutil.ComputeContentSha256(&element)
	if err != nil {
		return template.HTML(""), "", fmt.Errorf("compute csp hash for critical css: %w", err)
	}

	renderedElement, err = htmlutil.RenderElement(&element)
	if err != nil {
		return template.HTML(""), "", fmt.Errorf("render critical css style element: %w", err)
	}

	return renderedElement, sha256Hash, nil
}

// BuildStylesheetLink renders the non-critical stylesheet <link> element.
func BuildStylesheetLink(
	stylesheetURL string,
	nonCriticalCSSLinkElementID string,
) string {
	if stylesheetURL == "" {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(`<link rel="stylesheet" href="`)
	builder.WriteString(stylesheetURL)
	builder.WriteString(`" id="`)
	builder.WriteString(nonCriticalCSSLinkElementID)
	builder.WriteString(`" />`)
	return builder.String()
}

// BuildPublicFileMapElements renders the preload and module script elements
// that install the public file map into the browser runtime namespace.
func BuildPublicFileMapElements(
	fileMapURL string,
	browserRuntimeNamespace string,
) (
	elements string,
	sha256Hash string,
	err error,
) {
	linkElement := htmlutil.Element{
		Tag:         "link",
		Attributes:  map[string]string{"rel": "modulepreload", "href": fileMapURL},
		SelfClosing: true,
	}

	scriptElement := htmlutil.Element{
		Tag:        "script",
		Attributes: map[string]string{"type": "module"},
		DangerousInnerHTML: BuildPublicFileMapModuleScript(
			fileMapURL,
			browserRuntimeNamespace,
		),
	}

	scriptSHA256Hash, err := htmlutil.ComputeContentSha256(&scriptElement)
	if err != nil {
		return "", "", fmt.Errorf("compute csp hash for file map module script: %w", err)
	}

	var elementsBuilder strings.Builder

	err = htmlutil.RenderElementToBuilder(&linkElement, &elementsBuilder)
	if err != nil {
		return "", "", fmt.Errorf("render file map preload link: %w", err)
	}

	err = htmlutil.RenderElementToBuilder(&scriptElement, &elementsBuilder)
	if err != nil {
		return "", "", fmt.Errorf("render file map module script element: %w", err)
	}

	return elementsBuilder.String(), scriptSHA256Hash, nil
}

const publicFileMapModuleScriptFormat = `
		import { wavePublicFileMap } from %q;
		const browserRuntimeNamespace = %q;
		if (!window[browserRuntimeNamespace]) window[browserRuntimeNamespace] = {};
		window[browserRuntimeNamespace].publicFileMap = wavePublicFileMap;
`

// BuildPublicFileMapModuleScript returns the JS module body that installs the
// generated public file map on the browser runtime namespace.
func BuildPublicFileMapModuleScript(
	fileMapURL string,
	browserRuntimeNamespace string,
) string {
	return fmt.Sprintf(
		publicFileMapModuleScriptFormat,
		fileMapURL,
		browserRuntimeNamespace,
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
