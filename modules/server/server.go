package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"gogogo/modules/config"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type Server struct {
	httpServer *http.Server
	listener   net.Listener
	config     *Config
}

type Config struct {
	Host           string
	Port           int
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	MaxHeaderBytes int
	TLSConfig      *tls.Config
	EnableHTTP2    bool
	TCPKeepAlive   time.Duration
}

type Handlers struct {
	Web    http.Handler
	SPA    http.Handler
	Static http.Handler
	API    http.Handler
}

func New(handlers Handlers, cfg config.Config) *Server {
	opts := &Config{
		Host:           cfg.Server.Host,
		Port:           cfg.Server.Port,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    cfg.Server.IdleTimeout,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
		EnableHTTP2:    cfg.Server.EnableHTTP2,
		TLSConfig:      cfg.Server.TLSConfig,
		TCPKeepAlive:   30 * time.Second,
	}

	mux := http.NewServeMux()

	// API routes
	mux.Handle("/api/", handlers.API)

	// SPA routes
	if handlers.SPA != nil {
		mux.Handle(cfg.URLPrefixes.SPA, http.StripPrefix(cfg.URLPrefixes.SPA, handlers.SPA))
	}

	// Static files
	mux.Handle("/static/", handlers.Static)

	// All other paths go to web handler
	mux.Handle("/", handlers.Web)

	var handler http.Handler = mux
	if opts.EnableHTTP2 && opts.TLSConfig == nil {
		handler = h2c.NewHandler(mux, &http2.Server{})
	}

	return &Server{
		config: opts,
		httpServer: &http.Server{
			Handler:           handler,
			ReadTimeout:       opts.ReadTimeout,
			WriteTimeout:      opts.WriteTimeout,
			IdleTimeout:       opts.IdleTimeout,
			MaxHeaderBytes:    opts.MaxHeaderBytes,
			ReadHeaderTimeout: opts.ReadTimeout,
		},
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	ln = &tcpKeepAliveListener{
		TCPListener:     ln.(*net.TCPListener),
		keepAlivePeriod: s.config.TCPKeepAlive,
	}

	s.listener = ln

	if s.config.TLSConfig != nil {
		s.httpServer.TLSConfig = s.config.TLSConfig
		if s.config.EnableHTTP2 {
			http2.ConfigureServer(s.httpServer, &http2.Server{})
		}
		return s.httpServer.ServeTLS(ln, "", "")
	}

	return s.httpServer.Serve(ln)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

type tcpKeepAliveListener struct {
	*net.TCPListener
	keepAlivePeriod time.Duration
}

func (ln *tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(ln.keepAlivePeriod)
	tc.SetNoDelay(true)
	return tc, nil
}

// func (ln *tcpKeepAliveListener) Accept() (net.Conn, error) {
//     tc, err := ln.AcceptTCP()
//     if err != nil {
//         return nil, err
//     }

//     if err = tc.SetKeepAlive(true); err != nil {
//         tc.Close()
//         return nil, err
//     }

//     if err = tc.SetKeepAlivePeriod(ln.keepAlivePeriod); err != nil {
//         tc.Close()
//         return nil, err
//     }

//     if err = tc.SetNoDelay(true); err != nil {
//         tc.Close()
//         return nil, err
//     }

//     // Set buffer sizes optimally
//     if err = tc.SetReadBuffer(64 * 1024); err != nil {
//         tc.Close()
//         return nil, err
//     }

//     if err = tc.SetWriteBuffer(64 * 1024); err != nil {
//         tc.Close()
//         return nil, err
//     }

//     return tc, nil
// }
