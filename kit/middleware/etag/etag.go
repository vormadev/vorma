package etag

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"hash"
	"io"
	"maps"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	Strong      bool
	Hash        func() hash.Hash
	MaxBodySize int64
	SkipFunc    func(r *http.Request) bool
}

// Simple, automatic, and conservative ETag middleware that handles (1) setting ETags
// on GET and HEAD requests and (2) returning 304 responses as appropriate based on a
// request's If-None-Match header. Defaults to weak ETags, but can be configered to
// set strong ETags. ETags are determined by buffering and hashing the response body.
func Auto(config ...*Config) func(http.Handler) http.Handler {
	var configToUse *Config
	if len(config) > 0 && config[0] != nil {
		configToUse = config[0]
	} else {
		configToUse = new(Config)
	}
	if configToUse.Hash == nil {
		configToUse.Hash = sha1.New
	}
	if configToUse.MaxBodySize == 0 {
		configToUse.MaxBodySize = 8 * 1024 * 1024 // 8MB
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}
			if configToUse.SkipFunc != nil && configToUse.SkipFunc(r) {
				next.ServeHTTP(w, r)
				return
			}
			ew := newETagWriter(w, configToUse.Hash(), configToUse.MaxBodySize)
			defer ew.Close()
			next.ServeHTTP(ew, r)
			if !canUseETag(ew) {
				ew.WriteOriginalResponse()
				return
			}
			etag := generateETag(ew.hash, configToUse.Strong, ew.headers)
			ifNoneMatch := r.Header.Get("If-None-Match")
			if ifNoneMatch != "" && etagMatches(ifNoneMatch, etag) {
				respondNotModified(w, etag)
				return
			}
			ew.WriteResponseWithETag(etag)
		})
	}
}

type etagWriter struct {
	w           http.ResponseWriter
	status      int
	headersSent bool
	buf         *bytes.Buffer
	hash        hash.Hash
	tee         io.Writer
	headers     http.Header
	maxSize     int64
	size        int64
	tooBig      bool
}

var bufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 4096))
	},
}

func newETagWriter(w http.ResponseWriter, hash hash.Hash, maxSize int64) *etagWriter {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	headers := make(http.Header)
	maps.Copy(headers, w.Header())
	ew := &etagWriter{
		w:       w,
		status:  http.StatusOK,
		buf:     buf,
		hash:    hash,
		headers: headers,
		maxSize: maxSize,
	}
	ew.tee = io.MultiWriter(buf, hash)
	return ew
}

func (ew *etagWriter) Header() http.Header {
	return ew.headers
}

func (ew *etagWriter) WriteHeader(code int) {
	if !ew.headersSent {
		ew.status = code
		ew.headersSent = true
	}
}

func (ew *etagWriter) Write(b []byte) (int, error) {
	if !ew.headersSent {
		ew.headersSent = true
	}
	if ew.tooBig {
		ew.size += int64(len(b))
		return ew.w.Write(b)
	}
	if ew.maxSize > 0 && ew.size+int64(len(b)) > ew.maxSize {
		ew.tooBig = true
		if ew.buf.Len() > 0 {
			maps.Copy(ew.w.Header(), ew.headers)
			ew.w.WriteHeader(ew.status)
			ew.w.Write(ew.buf.Bytes())
			ew.buf.Reset()
		}
		ew.size += int64(len(b))
		return ew.w.Write(b)
	}

	ew.size += int64(len(b))
	return ew.tee.Write(b)
}

func (ew *etagWriter) Close() {
	if ew.buf != nil {
		bufPool.Put(ew.buf)
		ew.buf = nil
	}
}

func (ew *etagWriter) WriteResponseWithETag(etag string) {
	h := ew.w.Header()
	maps.Copy(h, ew.headers)
	h.Set("ETag", etag)
	if !ew.tooBig && ew.buf != nil {
		h.Set("Content-Length", strconv.Itoa(ew.buf.Len()))
	}
	ew.w.WriteHeader(ew.status)
	if ew.buf != nil && ew.buf.Len() > 0 {
		ew.w.Write(ew.buf.Bytes())
	}
}

func (ew *etagWriter) WriteOriginalResponse() {
	if ew.tooBig {
		return
	}
	maps.Copy(ew.w.Header(), ew.headers)
	ew.w.WriteHeader(ew.status)
	if ew.buf != nil && ew.buf.Len() > 0 {
		ew.w.Write(ew.buf.Bytes())
	}
}

func canUseETag(ew *etagWriter) bool {
	if ew.tooBig {
		return false
	}
	if ew.status != http.StatusOK {
		return false
	}
	if ew.buf == nil || ew.buf.Len() == 0 {
		return false
	}
	if strings.Contains(ew.headers.Get("Cache-Control"), "no-store") {
		return false
	}
	if ew.headers.Get("Set-Cookie") != "" {
		return false
	}
	return true
}

func generateETag(h hash.Hash, strong bool, headers http.Header) string {
	if buildID := headers.Get("X-Vorma-Build-Id"); buildID != "" {
		h.Write([]byte(buildID))
	}

	sum := h.Sum(nil)
	tag := hex.EncodeToString(sum)

	if !strong {
		return `W/"` + tag + `"`
	}
	return `"` + tag + `"`
}

func etagMatches(ifNoneMatch, etag string) bool {
	if ifNoneMatch == "*" {
		return true
	}
	etagVal := extractETagValue(etag)
	for cand := range strings.SplitSeq(ifNoneMatch, ",") {
		if extractETagValue(strings.TrimSpace(cand)) == etagVal {
			return true
		}
	}
	return false
}

func extractETagValue(etag string) string {
	etag = strings.TrimPrefix(etag, "W/")
	return strings.Trim(etag, "\"")
}

func respondNotModified(w http.ResponseWriter, etag string) {
	h := w.Header()
	for header := range h {
		if isPayloadHeader(header) {
			h.Del(header)
		}
	}
	h.Set("ETag", etag)
	w.WriteHeader(http.StatusNotModified)
}

func isPayloadHeader(header string) bool {
	switch strings.ToLower(header) {
	case "content-type", "content-length", "content-encoding",
		"content-language", "content-md5", "content-range",
		"content-disposition", "last-modified", "digest":
		return true
	default:
		return false
	}
}
