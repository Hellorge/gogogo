// File: modules/server/listener.go

package server

import (
	"log"
	"net"
	"syscall"
	"time"
)

const (
	tcpFastOpen     = 23  // TCP_FASTOPEN value for Linux
	tcpFastOpenQlen = 256 // Queue length for TFO
)

type tcpKeepAliveListener struct {
	*net.TCPListener
	keepAlivePeriod time.Duration
}

func enableTCPFastOpen(fd int) error {
	// Enable TCP Fast Open on listener
	return syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, tcpFastOpen, tcpFastOpenQlen)
}

func (ln *tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}

	// Get underlying file descriptor
	file, err := tc.File()
	if err != nil {
		tc.Close()
		return nil, err
	}
	fd := int(file.Fd())
	file.Close()

	// Enable TCP Fast Open
	if err := enableTCPFastOpen(fd); err != nil {
		// Log error but don't fail - TFO is an optimization
		log.Printf("TCP Fast Open enable failed: %v", err)
	}

	if err := tc.SetKeepAlive(true); err != nil {
		tc.Close()
		return nil, err
	}

	if err := tc.SetKeepAlivePeriod(ln.keepAlivePeriod); err != nil {
		tc.Close()
		return nil, err
	}

	// Optimize buffer sizes
	if err := tc.SetReadBuffer(64 * 1024); err != nil {
		tc.Close()
		return nil, err
	}

	if err := tc.SetWriteBuffer(64 * 1024); err != nil {
		tc.Close()
		return nil, err
	}

	return tc, nil
}
