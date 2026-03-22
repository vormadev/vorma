package xyz

import (
	"net/url"
	"strings"
	"testing"
)

func TestMakeEmojiDataURL_BuildsExpectedSVGDataURL(t *testing.T) {
	dataURL := MakeEmojiDataURL("🚀")

	if !strings.HasPrefix(dataURL, "data:image/svg+xml,") {
		t.Fatalf("expected data URL prefix, got %q", dataURL)
	}

	encodedPayload := strings.TrimPrefix(dataURL, "data:image/svg+xml,")
	svgPayload, unescapeErr := url.PathUnescape(encodedPayload)
	if unescapeErr != nil {
		t.Fatalf("url.PathUnescape() error = %v", unescapeErr)
	}
	if !strings.Contains(svgPayload, "<text x='0' y='14'>🚀</text>") {
		t.Fatalf("expected emoji text node in decoded SVG payload, got %q", svgPayload)
	}
	if !strings.HasSuffix(svgPayload, "</svg>") {
		t.Fatalf("expected closing svg tag in decoded payload, got %q", svgPayload)
	}
}

func TestMakeEmojiDataURL_EscapesReservedCharacters(t *testing.T) {
	dataURL := MakeEmojiDataURL("#")

	parsedURL, parseErr := url.Parse(dataURL)
	if parseErr != nil {
		t.Fatalf("url.Parse() error = %v", parseErr)
	}
	if parsedURL.Fragment != "" {
		t.Fatalf("expected empty fragment for encoded data URL, got %q", parsedURL.Fragment)
	}
}
