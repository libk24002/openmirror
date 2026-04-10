package mirror

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"
)

type pypiHandler struct {
	index http.Handler
	files http.Handler
}

func NewPyPIHandler(index, files http.Handler) http.Handler {
	return &pypiHandler{
		index: nonNilHandler(index),
		files: nonNilHandler(files),
	}
}

func (h *pypiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/packages/") {
		h.files.ServeHTTP(w, r)
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/simple/") {
		h.index.ServeHTTP(w, r)
		return
	}

	captured := newCapturedResponseWriter()
	h.index.ServeHTTP(captured, r)

	headers := captured.Header().Clone()
	body := captured.Body()
	if isSuccessStatus(captured.StatusCode()) {
		rewrittenBody := rewritePyPIFilesLinks(body)
		if !bytes.Equal(rewrittenBody, body) {
			body = rewrittenBody
			headers.Del("Content-Length")
			headers.Set("Content-Length", strconv.Itoa(len(body)))
		}
	}

	for key, values := range headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(captured.StatusCode())
	_, _ = w.Write(body)
}

func rewritePyPIFilesLinks(body []byte) []byte {
	rewritten := bytes.ReplaceAll(body, []byte("https://files.pythonhosted.org/"), []byte("/pypi/"))
	rewritten = bytes.ReplaceAll(rewritten, []byte("//files.pythonhosted.org/"), []byte("/pypi/"))

	return rewritten
}

func isSuccessStatus(statusCode int) bool {
	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices
}

type capturedResponseWriter struct {
	header      http.Header
	statusCode  int
	wroteHeader bool
	body        bytes.Buffer
}

func newCapturedResponseWriter() *capturedResponseWriter {
	return &capturedResponseWriter{header: make(http.Header)}
}

func (w *capturedResponseWriter) Header() http.Header {
	return w.header
}

func (w *capturedResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}

	w.statusCode = statusCode
	w.wroteHeader = true
}

func (w *capturedResponseWriter) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	return w.body.Write(body)
}

func (w *capturedResponseWriter) StatusCode() int {
	if w.wroteHeader {
		return w.statusCode
	}

	return http.StatusOK
}

func (w *capturedResponseWriter) Body() []byte {
	return append([]byte(nil), w.body.Bytes()...)
}

func nonNilHandler(handler http.Handler) http.Handler {
	if handler != nil {
		return handler
	}

	return http.NotFoundHandler()
}
