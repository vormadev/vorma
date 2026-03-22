package etag

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash"
	"net"
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
			etag := generateETag(ew.hash, configToUse.Strong, ew.w.Header())
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
	maxSize     int64
	size        int64
	tooBig      bool
}

var bufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 4096))
	},
}

func newETagWriter(
	w http.ResponseWriter,
	hash hash.Hash,
	maxSize int64,
) *etagWriter {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	return &etagWriter{
		w:       w,
		status:  http.StatusOK,
		buf:     buf,
		hash:    hash,
		maxSize: maxSize,
	}
}

func (ew *etagWriter) Header() http.Header {
	return ew.w.Header()
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
		ew.beginPassthrough()
		ew.size += int64(len(b))
		return ew.w.Write(b)
	}

	ew.size += int64(len(b))
	_, err := ew.hash.Write(b)
	if err != nil {
		return 0, err
	}
	return ew.buf.Write(b)
}

func (ew *etagWriter) Close() {
	if ew.buf != nil {
		bufPool.Put(ew.buf)
		ew.buf = nil
	}
}

func (ew *etagWriter) beginPassthrough() {
	if ew.tooBig {
		return
	}
	ew.tooBig = true
	ew.w.WriteHeader(ew.status)
	if ew.buf != nil && ew.buf.Len() > 0 {
		_, _ = ew.w.Write(ew.buf.Bytes())
		ew.buf.Reset()
	}
}

func (ew *etagWriter) Flush() {
	flusher, ok := ew.w.(http.Flusher)
	if !ok {
		return
	}
	if ew.buf == nil {
		flusher.Flush()
		return
	}

	if !ew.tooBig {
		ew.beginPassthrough()
	}

	flusher.Flush()
}

func (ew *etagWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := ew.w.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf(
			"response writer does not support hijacking",
		)
	}
	ew.tooBig = true
	return hijacker.Hijack()
}

func (ew *etagWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := ew.w.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (ew *etagWriter) WriteResponseWithETag(etag string) {
	h := ew.w.Header()
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
	headers := ew.w.Header()
	if hasNoStoreDirective(headers.Get("Cache-Control")) {
		return false
	}
	if headers.Get("Set-Cookie") != "" {
		return false
	}
	return true
}

func hasNoStoreDirective(cacheControl string) bool {
	if cacheControl == "" {
		return false
	}
	for directive := range strings.SplitSeq(cacheControl, ",") {
		if strings.EqualFold(strings.TrimSpace(directive), "no-store") {
			return true
		}
	}
	return false
}

func generateETag(h hash.Hash, strong bool, headers http.Header) string {
	if buildID := headers.Get("X-Vorma-Client-Build-Id"); buildID != "" {
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
