package container

import (
	"minidocker/internal/config"
	"minidocker/internal/namespace"
	"os"
	"os/exec"
)

const initEnvKey = "MINIDOCKER_INIT"

type Container struct {
	Config *config.Config
}

func New(cfg *config.Config) *Container {
	return &Container{Config: cfg}
}

func (c *Container) Run() error {
	// Reenviar --rootfs al proceso init: sin esto el hijo usaría
	// siempre el valor por defecto (Hito 2).
	initArgs := append(
		[]string{"--rootfs", c.Config.Rootfs, "init", c.Config.Command},
		c.Config.Args...,
	)
	cmd := exec.Command("/proc/self/exe", initArgs...)
	cmd.Args[0] = "minidocker-init"

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = namespace.SysProcAttr()
	cmd.Env = append(os.Environ(), initEnvKey+"=1")

	return cmd.Run()
}
