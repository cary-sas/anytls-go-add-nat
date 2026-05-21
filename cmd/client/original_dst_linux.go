//go:build linux

package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"unsafe"

	M "github.com/sagernet/sing/common/metadata"
	"golang.org/x/sys/unix"
)

const soOriginalDst = 80

type sockaddrInet4 struct {
	Family uint16
	Port   [2]byte
	Addr   [4]byte
	Zero   [8]byte
}

type sockaddrInet6 struct {
	Family   uint16
	Port     [2]byte
	Flowinfo uint32
	Addr     [16]byte
	ScopeID  uint32
}

func originalDestination(conn net.Conn) (M.Socksaddr, error) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return M.Socksaddr{}, fmt.Errorf("connection is %T, not *net.TCPConn", conn)
	}

	rawConn, err := tcpConn.SyscallConn()
	if err != nil {
		return M.Socksaddr{}, err
	}

	var destination M.Socksaddr
	var sockErr error
	controlErr := rawConn.Control(func(fd uintptr) {
		localAddr, _ := tcpConn.LocalAddr().(*net.TCPAddr)
		if localAddr != nil && localAddr.IP.To4() == nil {
			destination, sockErr = getOriginalDst6(fd)
			return
		}
		destination, sockErr = getOriginalDst4(fd)
	})
	if controlErr != nil {
		return M.Socksaddr{}, controlErr
	}
	if sockErr != nil {
		return M.Socksaddr{}, sockErr
	}
	return destination, nil
}

func getOriginalDst4(fd uintptr) (M.Socksaddr, error) {
	var addr sockaddrInet4
	size := uint32(unsafe.Sizeof(addr))
	_, _, errno := unix.Syscall6(
		unix.SYS_GETSOCKOPT,
		fd,
		uintptr(unix.SOL_IP),
		uintptr(soOriginalDst),
		uintptr(unsafe.Pointer(&addr)),
		uintptr(unsafe.Pointer(&size)),
		0,
	)
	if errno != 0 {
		return M.Socksaddr{}, errno
	}

	return M.SocksaddrFrom(netip.AddrFrom4(addr.Addr), binary.BigEndian.Uint16(addr.Port[:])), nil
}

func getOriginalDst6(fd uintptr) (M.Socksaddr, error) {
	var addr sockaddrInet6
	size := uint32(unsafe.Sizeof(addr))
	_, _, errno := unix.Syscall6(
		unix.SYS_GETSOCKOPT,
		fd,
		uintptr(unix.SOL_IPV6),
		uintptr(soOriginalDst),
		uintptr(unsafe.Pointer(&addr)),
		uintptr(unsafe.Pointer(&size)),
		0,
	)
	if errno != 0 {
		return M.Socksaddr{}, errno
	}

	return M.SocksaddrFrom(netip.AddrFrom16(addr.Addr), binary.BigEndian.Uint16(addr.Port[:])), nil
}
