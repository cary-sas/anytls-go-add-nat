//go:build !linux

package main

import (
	"fmt"
	"net"

	M "github.com/sagernet/sing/common/metadata"
)

func originalDestination(conn net.Conn) (M.Socksaddr, error) {
	return M.Socksaddr{}, fmt.Errorf("NAT transparent proxy mode requires Linux SO_ORIGINAL_DST support")
}
