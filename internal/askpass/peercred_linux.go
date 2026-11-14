package askpass

import (
	"net"

	"golang.org/x/sys/unix"
)

// peerUID is the user id of the process on the other end of c.
func peerUID(c *net.UnixConn) (uint32, error) {
	raw, err := c.SyscallConn()
	if err != nil {
		return 0, err
	}
	var uid uint32
	var gerr error
	if err := raw.Control(func(fd uintptr) {
		cred, e := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if e != nil {
			gerr = e
			return
		}
		uid = cred.Uid
	}); err != nil {
		return 0, err
	}
	return uid, gerr
}
