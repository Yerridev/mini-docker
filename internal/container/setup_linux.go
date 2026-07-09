package container

import (
	"fmt"
	"minidocker/internal/config"
	"syscall"
)

func sethostname(name string) error {
	if err := syscall.Sethostname([]byte(name)); err != nil {
		return fmt.Errorf("sethostname: %w", err)
	}
	return nil
}

func mountProc() error {
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("mount /proc: %w", err)
	}
	return nil
}

func setupContainer(cfg *config.Config) error {
	// FASE 1 — UTS: cambiar hostname dentro del namespace
	if err := sethostname("minidocker"); err != nil {
		return err
	}

	// FASE 3 — MNT + PID: montar /proc propio
	// Dentro del nuevo MNT namespace, este mount NO afecta al host.
	if err := mountProc(); err != nil {
		return err
	}

	// HITO 2 — MNT: chroot/pivot_root al rootfs
	_ = cfg

	return nil
}
