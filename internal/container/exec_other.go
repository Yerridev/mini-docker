//go:build !linux

package container

import (
	"minidocker/internal/config"
	"os"
	"os/exec"
	"syscall"
)

func execContainer(cfg *config.Config) error {
	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.Env = os.Environ()
	return cmd.Run()
}
