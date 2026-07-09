package container

import (
	"minidocker/internal/config"
	"os"
	"syscall"
)

func execContainer(cfg *config.Config) error {
	return syscall.Exec(cfg.Command, append([]string{cfg.Command}, cfg.Args...), os.Environ())
}
