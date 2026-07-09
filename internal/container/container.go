package container

import (
	"fmt"
	"minidocker/internal/cgroup"
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
	initArgs := append([]string{"init", c.Config.Command}, c.Config.Args...)
	cmd := exec.Command("/proc/self/exe", initArgs...)
	cmd.Args[0] = "minidocker-init"

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = namespace.SysProcAttr()
	cmd.Env = append(os.Environ(), initEnvKey+"=1")

	// Hito 3 — cgroups: crear el cgroup solo si se pidió algún límite, para no
	// requerir root ni tocar /sys cuando no hace falta. El cgroup se administra
	// desde el proceso padre; el contenedor hereda el límite al ser movido.
	var cg *cgroup.Manager
	if c.Config.MemoryBytes > 0 || c.Config.CPUQuota > 0 {
		m, err := cgroup.New(containerID())
		if err != nil {
			return fmt.Errorf("cgroup: %w", err)
		}
		cg = m
		// Cleanup pase lo que pase: mata procesos restantes y borra el cgroup.
		defer cg.Cleanup()

		if err := cg.SetMemoryLimit(c.Config.MemoryBytes); err != nil {
			return err
		}
		if err := cg.SetCPULimit(c.Config.CPUQuota, c.Config.CPUPeriod); err != nil {
			return err
		}
	}

	// Start (no Run) para obtener el PID y moverlo al cgroup mientras corre.
	if err := cmd.Start(); err != nil {
		return err
	}

	if cg != nil {
		if err := cg.AddProcess(cmd.Process.Pid); err != nil {
			return fmt.Errorf("cgroup: agregar proceso %d: %w", cmd.Process.Pid, err)
		}
	}

	return cmd.Wait()
}

// containerID genera un identificador de cgroup por ejecución. El PID del padre
// basta: el directorio se elimina en Cleanup, así que no hay colisiones.
func containerID() string {
	return fmt.Sprintf("c-%d", os.Getpid())
}
