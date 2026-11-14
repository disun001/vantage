package askpass

import (
	"io/fs"
	"syscall"
)

// owner is the user id that owns the file.
func owner(st fs.FileInfo) (uint32, bool) {
	s, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return s.Uid, true
}
