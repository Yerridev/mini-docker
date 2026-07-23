package container

import (
	"fmt"
	"minidocker/internal/cgroup"
	"minidocker/internal/config"
	"minidocker/internal/namespace"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const initEnvKey = "MINIDOCKER_INIT"

// killGrace es el tiempo que se espera tras reenviar SIGTERM/SIGINT al
// contenedor antes de forzar SIGKILL. El proceso init del contenedor (PID 1 en
// su namespace) no recibe la acción por defecto de SIGTERM, así que si no la
// maneja hay que matarlo; un período corto mantiene la respuesta ágil.
const killGrace = 3 * time.Second

type Container struct {
	Config *config.Config
}

func New(cfg *config.Config) *Container {
	return &Container{Config: cfg}
}

func (c *Container) Run() error {
	// Reenviar --rootfs y --volume al proceso init (el hijo los necesita para
	// el chroot y los bind mounts). Los límites de cgroup y las variables de
	// entorno los aplica el padre, no se reenvían como flags.
	initArgs := []string{"--rootfs", c.Config.Rootfs, "--hostname", c.Config.Hostname, "--net", c.Config.NetMode.String()}
	for _, v := range c.Config.Volumes {
		initArgs = append(initArgs, "--volume", v.Source+":"+v.Target)
	}
	initArgs = append(initArgs, "init", c.Config.Command)
	initArgs = append(initArgs, c.Config.Args...)

	cmd := exec.Command("/proc/self/exe", initArgs...)
	cmd.Args[0] = "minidocker-init"

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = namespace.SysProcAttr()
	// Hito 4 — variables de entorno: se inyectan en el entorno del hijo, que lo
	// hereda hasta el syscall.Exec final (execContainer usa os.Environ()).
	cmd.Env = mergeEnv(append(os.Environ(), initEnvKey+"=1"), c.Config.Env)

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

	// Hito 4 — señales: reenviar SIGINT/SIGTERM al contenedor y forzar SIGKILL
	// si no termina dentro del período de gracia.
	stop := forwardSignals(cmd)
	defer stop()

	return cmd.Wait()
}

// forwardSignals reenvía la primera SIGINT/SIGTERM recibida al contenedor y, si
// no muere en killGrace, lo mata con SIGKILL. Devuelve una función de parada.
func forwardSignals(cmd *exec.Cmd) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})

	go func() {
		select {
		case sig := <-ch:
			_ = cmd.Process.Signal(sig)
			select {
			case <-done: // el contenedor salió limpio
			case <-time.After(killGrace):
				_ = cmd.Process.Kill()
			}
		case <-done:
		}
	}()

	return func() {
		close(done)
		signal.Stop(ch)
	}
}

// mergeEnv combina base con extra dando prioridad a extra para claves repetidas,
// para que --env pueda sobrescribir variables heredadas.
func mergeEnv(base, extra []string) []string {
	if len(extra) == 0 {
		return base
	}
	override := make(map[string]struct{}, len(extra))
	for _, e := range extra {
		if k, _, ok := strings.Cut(e, "="); ok {
			override[k] = struct{}{}
		}
	}
	out := make([]string, 0, len(base)+len(extra))
	for _, e := range base {
		k, _, _ := strings.Cut(e, "=")
		if _, replaced := override[k]; !replaced {
			out = append(out, e)
		}
	}
	return append(out, extra...)
}

// containerID genera un identificador de cgroup por ejecución. El PID del padre
// basta: el directorio se elimina en Cleanup, así que no hay colisiones.
func containerID() string {
	return fmt.Sprintf("c-%d", os.Getpid())
}
