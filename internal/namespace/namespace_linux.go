package namespace

import "syscall"

const (
	flagNEWUTS = 0x04000000
	flagNEWPID = 0x20000000
	flagNEWMNT = 0x00020000
)

func SysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Cloneflags: flagNEWUTS | flagNEWPID | flagNEWMNT,
	}
}
