package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type Server struct {
	httpServer  *http.Server
	listener    net.Listener
	Config      *Config
	Handler     http.Handler
	middlewares []func(http.Handler) http.Handler
}

type Config struct {
	Host              string
	Port              int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
	KeepAliveDuration time.Duration
	TLSConfig         *tls.Config
	EnableHTTP2       bool
	GracefulTimeout   time.Duration
	MaxConnsPerIP     int
	TCPKeepAlive      time.Duration
}

func NewServer(config *Config) *Server {
	return &Server{
		Config:      config,
		middlewares: []func(http.Handler) http.Handler{},
	}
}

func (s *Server) Use(middleware func(http.Handler) http.Handler) {
	s.middlewares = append(s.middlewares, middleware)
}

func (s *Server) SetHandler(handler http.Handler) {
	s.Handler = handler
}

func (s *Server) chainMiddleware(h http.Handler) http.Handler {
	for i := len(s.middlewares) - 1; i >= 0; i-- {
		h = s.middlewares[i](h)
	}
	return h
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.Config.Host, s.Config.Port)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	ln = tcpKeepAliveListener{
		TCPListener:     ln.(*net.TCPListener),
		keepAlivePeriod: s.Config.TCPKeepAlive,
	}

	s.listener = ln

	handler := s.chainMiddleware(s.Handler)

	if s.Config.EnableHTTP2 && s.Config.TLSConfig == nil {
		handler = h2c.NewHandler(handler, &http2.Server{})
	}

	s.httpServer = &http.Server{
		Handler:        handler,
		ReadTimeout:    s.Config.ReadTimeout,
		WriteTimeout:   s.Config.WriteTimeout,
		IdleTimeout:    s.Config.IdleTimeout,
		MaxHeaderBytes: s.Config.MaxHeaderBytes,
	}

	if s.Config.TLSConfig != nil {
		s.httpServer.TLSConfig = s.Config.TLSConfig
		if s.Config.EnableHTTP2 {
			http2.ConfigureServer(s.httpServer, &http2.Server{})
		}
		return s.httpServer.ServeTLS(s.listener, "", "")
	}

	return s.httpServer.Serve(s.listener)
}

func (s *Server) Shutdown() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), s.Config.GracefulTimeout)
	go func() {
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			fmt.Printf("Server Shutdown error: %v\n", err)
		}
	}()
	return ctx
}

type tcpKeepAliveListener struct {
	*net.TCPListener
	keepAlivePeriod time.Duration
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(ln.keepAlivePeriod)
	return tc, nil
}
