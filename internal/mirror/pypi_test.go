package mirror

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPyPIHandlerRoutesPackagesToFilesHandler(t *testing.T) {
	var indexHits atomic.Int32
	var filesHits atomic.Int32

	indexHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		indexHits.Add(1)
		_, _ = w.Write([]byte("index"))
	})
	filesHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filesHits.Add(1)
		_, _ = w.Write([]byte("files"))
	})

	h := NewPyPIHandler(indexHandler, filesHandler)

	req := httptest.NewRequest(http.MethodGet, "/packages/aa/bb/pkg.whl", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "files" {
		t.Fatalf("body = %q, want %q", got, "files")
	}
	if got := indexHits.Load(); got != 0 {
		t.Fatalf("index hit count = %d, want %d", got, 0)
	}
	if got := filesHits.Load(); got != 1 {
		t.Fatalf("files hit count = %d, want %d", got, 1)
	}
}

func TestPyPIHandlerRoutesSimpleToIndexAndRewritesFileLinks(t *testing.T) {
	var indexHits atomic.Int32
	var filesHits atomic.Int32

	indexHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		indexHits.Add(1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", "999")
		w.Header().Set("X-Upstream", "index")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<a href="https://files.pythonhosted.org/packages/a/aa/file-1.whl">one</a><a href="//files.pythonhosted.org/packages/b/bb/file-2.whl">two</a>`))
	})
	filesHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filesHits.Add(1)
		_, _ = w.Write([]byte("files"))
	})

	h := NewPyPIHandler(indexHandler, filesHandler)

	req := httptest.NewRequest(http.MethodGet, "/simple/sample/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := indexHits.Load(); got != 1 {
		t.Fatalf("index hit count = %d, want %d", got, 1)
	}
	if got := filesHits.Load(); got != 0 {
		t.Fatalf("files hit count = %d, want %d", got, 0)
	}

	body := rec.Body.String()
	if strings.Contains(body, "files.pythonhosted.org") {
		t.Fatalf("body still contains files.pythonhosted.org: %q", body)
	}
	if !strings.Contains(body, `/pypi/packages/a/aa/file-1.whl`) {
		t.Fatalf("body missing rewritten absolute link: %q", body)
	}
	if !strings.Contains(body, `/pypi/packages/b/bb/file-2.whl`) {
		t.Fatalf("body missing rewritten protocol-relative link: %q", body)
	}

	if got := rec.Header().Get("X-Upstream"); got != "index" {
		t.Fatalf("X-Upstream = %q, want %q", got, "index")
	}
	expectedLength := strconv.Itoa(len(body))
	if got := rec.Header().Get("Content-Length"); got != expectedLength {
		t.Fatalf("Content-Length = %q, want %q", got, expectedLength)
	}
}

func TestPyPIHandlerRewritesSimpleLinksWhenClientRequestsGzip(t *testing.T) {
	indexHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamBody := []byte(`<a href="https://files.pythonhosted.org/packages/c/cc/file-3.whl">three</a>`)

		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			var compressed bytes.Buffer
			gzipWriter := gzip.NewWriter(&compressed)
			_, _ = gzipWriter.Write(upstreamBody)
			_ = gzipWriter.Close()

			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Length", strconv.Itoa(compressed.Len()))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(compressed.Bytes())
			return
		}

		w.Header().Set("Content-Length", strconv.Itoa(len(upstreamBody)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(upstreamBody)
	})

	h := NewPyPIHandler(indexHandler, nil)

	req := httptest.NewRequest(http.MethodGet, "/simple/sample/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if strings.Contains(body, "files.pythonhosted.org") {
		t.Fatalf("body still contains files.pythonhosted.org: %q", body)
	}
	if !strings.Contains(body, `/pypi/packages/c/cc/file-3.whl`) {
		t.Fatalf("body missing rewritten link: %q", body)
	}
}
