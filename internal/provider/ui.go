package provider

import (
	"io/fs"
	"net/http"

	"github.com/Nebuless/nebprx/dashboard"
)

func (s *Server) managementPage(w http.ResponseWriter, r *http.Request, _ Identity) {
	sub, err := fs.Sub(dashboard.DistFS, "dist")
	if err != nil {
		http.Error(w, "dashboard not built", http.StatusServiceUnavailable)
		return
	}
	cloned := r.Clone(r.Context())
	u := *r.URL
	u.Path = "/"
	cloned.URL = &u
	http.FileServer(http.FS(sub)).ServeHTTP(w, cloned)
}
func (s *Server) asset(w http.ResponseWriter, r *http.Request) {
	sub, err := fs.Sub(dashboard.DistFS, "dist")
	if err != nil {
		http.Error(w, "dashboard not built", http.StatusServiceUnavailable)
		return
	}
	http.FileServer(http.FS(sub)).ServeHTTP(w, r)
}
