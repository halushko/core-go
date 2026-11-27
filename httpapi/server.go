package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const DefaultMaxBodyBytes int64 = 1 << 20

type Config struct {
	// MaxBodyBytes defaults to 1 MiB when zero or negative.
	MaxBodyBytes int64
	// OpenAPI is disabled when nil.
	OpenAPI *OpenAPIConfig
}

// Server owns the HTTP router. Its public API does not expose Gin.
type Server struct {
	engine       *gin.Engine
	maxBodyBytes int64
	docs         *openAPIDocument
}

func New() *Server {
	return NewWithConfig(Config{})
}

func NewWithConfig(config Config) *Server {
	if config.MaxBodyBytes <= 0 {
		config.MaxBodyBytes = DefaultMaxBodyBytes
	}
	engine := gin.New()
	_ = engine.SetTrustedProxies(nil)
	engine.HandleMethodNotAllowed = true
	engine.RedirectTrailingSlash = false
	engine.NoRoute(func(c *gin.Context) {
		writeFailure(c, http.StatusNotFound, "NOT_FOUND", "Route not found")
	})
	engine.NoMethod(func(c *gin.Context) {
		writeFailure(c, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
	})
	s := &Server{engine: engine, maxBodyBytes: config.MaxBodyBytes}
	if config.OpenAPI != nil {
		s.enableOpenAPI(*config.OpenAPI)
	}
	return s
}

// ServeHTTP also allows integration with net/http.Server and httptest.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.engine.ServeHTTP(w, r)
}

// Run starts a server. For custom timeouts, TLS and graceful shutdown,
// use Server directly as the Handler of a standard http.Server.
func (s *Server) Run(addr string) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           s,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return server.ListenAndServe()
}
