package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fgrehm/clotilde/internal/server"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/util"
)

var _ = Describe("Server", func() {
	var (
		srv     *server.Server
		handler http.Handler
		repoDir string
	)

	BeforeEach(func() {
		repoDir = GinkgoT().TempDir()

		// Create a .tours directory with a tour file
		toursDir := filepath.Join(repoDir, ".tours")
		Expect(os.MkdirAll(toursDir, 0o755)).To(Succeed())

		tourJSON := `{
			"title": "Overview",
			"steps": [
				{"file": "src/main.go", "line": 10, "description": "Entry point"},
				{"file": "src/config.go", "line": 5, "description": "Config"}
			]
		}`
		Expect(os.WriteFile(filepath.Join(toursDir, "overview.tour"), []byte(tourJSON), 0o644)).To(Succeed())

		// Create some source files
		srcDir := filepath.Join(repoDir, "src")
		Expect(os.MkdirAll(srcDir, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(srcDir, "config.go"), []byte("package main\n\ntype Config struct{}\n"), 0o644)).To(Succeed())

		// Create dirs that should be excluded from tree
		Expect(os.MkdirAll(filepath.Join(repoDir, ".git", "objects"), 0o755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(repoDir, "node_modules", "foo"), 0o755)).To(Succeed())

		sess := session.NewSession("test-tour", util.GenerateUUID())
		srv = server.New(0, repoDir, "haiku", sess, "")
		handler = srv.Handler()
	})

	Describe("GET /api/tours", func() {
		It("returns list of tours with name, title, and step count", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/tours", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))

			var tours []map[string]any
			Expect(json.Unmarshal(w.Body.Bytes(), &tours)).To(Succeed())
			Expect(tours).To(HaveLen(1))
			Expect(tours[0]["name"]).To(Equal("overview"))
			Expect(tours[0]["title"]).To(Equal("Overview"))
			Expect(tours[0]["steps"]).To(BeNumerically("==", 2))
		})
	})

	Describe("GET /api/tours/:name", func() {
		It("returns full tour data", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/tours/overview", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var tourData map[string]any
			Expect(json.Unmarshal(w.Body.Bytes(), &tourData)).To(Succeed())
			Expect(tourData["title"]).To(Equal("Overview"))

			steps, ok := tourData["steps"].([]any)
			Expect(ok).To(BeTrue())
			Expect(steps).To(HaveLen(2))
		})

		It("returns 404 for nonexistent tour", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/tours/nonexistent", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("GET /api/files/", func() {
		It("returns file content", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/files/src/main.go", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(ContainSubstring("package main"))
		})

		It("returns 403 for path traversal attempt", func() {
			// Use a path with .. that doesn't get cleaned by the mux
			req := httptest.NewRequest(http.MethodGet, "/api/files/src/..%2F..%2Fetc%2Fpasswd", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// Should be either 403 (caught by our check) or 404 (file doesn't exist)
			// but never 200 with sensitive content
			Expect(w.Code).To(SatisfyAny(
				Equal(http.StatusForbidden),
				Equal(http.StatusNotFound),
			))
			Expect(w.Body.String()).NotTo(ContainSubstring("root:"))
		})

		It("returns 404 for nonexistent file", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/files/nonexistent.go", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("GET /api/tree", func() {
		It("returns file listing", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/tree", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var files []string
			Expect(json.Unmarshal(w.Body.Bytes(), &files)).To(Succeed())
			Expect(files).To(ContainElement("src/main.go"))
			Expect(files).To(ContainElement("src/config.go"))
		})

		It("excludes .git, node_modules directories", func() {
			req := httptest.NewRequest(http.MethodGet, "/api/tree", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var files []string
			Expect(json.Unmarshal(w.Body.Bytes(), &files)).To(Succeed())
			for _, f := range files {
				Expect(f).NotTo(HavePrefix(".git/"))
				Expect(f).NotTo(HavePrefix("node_modules/"))
			}
		})
	})
})
