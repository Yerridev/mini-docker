package cgroup

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// Punto de montaje de la jerarquía unificada de cgroups v2.
	cgroupRoot = "/sys/fs/cgroup"
	// Nivel intermedio que agrupa todos los contenedores de minidocker.
	parentName = "minidocker"
)

// New crea el cgroup del contenedor en /sys/fs/cgroup/minidocker/<id>.
//
// Requiere cgroups v2 y privilegios de root (o CAP_SYS_ADMIN). Antes de poder
// fijar límites hay que "delegar" los controladores: en cgroups v2 un
// controlador solo puede usarse en un cgroup si está habilitado en el
// cgroup.subtree_control del PADRE. Por eso se habilita memory/cpu en la raíz y
// en el nivel intermedio antes de crear el cgroup del contenedor.
func New(id string) (*Manager, error) {
	if _, err := os.Stat(filepath.Join(cgroupRoot, "cgroup.controllers")); err != nil {
		return nil, fmt.Errorf("cgroups v2 no disponible en %s (¿kernel con cgroup1?): %w", cgroupRoot, err)
	}

	if err := delegateControllers(cgroupRoot); err != nil {
		return nil, err
	}
	parent := filepath.Join(cgroupRoot, parentName)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, fmt.Errorf("crear cgroup %s: %w", parent, err)
	}
	if err := delegateControllers(parent); err != nil {
		return nil, err
	}

	path := filepath.Join(parent, id)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("crear cgroup %s: %w", path, err)
	}
	return &Manager{path: path, parent: parent, id: id}, nil
}

// SetMemoryLimit fija memory.max en bytes. bytes <= 0 no hace nada.
func (m *Manager) SetMemoryLimit(bytes int64) error {
	if bytes <= 0 {
		return nil
	}
	return m.write("memory.max", strconv.FormatInt(bytes, 10))
}

// SetCPULimit fija cpu.max = "quota period" (microsegundos). Con quota=50000 y
// period=100000 el contenedor obtiene 0.5 CPUs. quota <= 0 no hace nada.
func (m *Manager) SetCPULimit(quota, period int64) error {
	if quota <= 0 {
		return nil
	}
	return m.write("cpu.max", FormatCPUMax(quota, period))
}

// AddProcess mueve un proceso (y todos sus hilos/hijos) al cgroup escribiendo
// su PID en cgroup.procs. Debe llamarse DESPUÉS de arrancar el proceso.
func (m *Manager) AddProcess(pid int) error {
	return m.write("cgroup.procs", strconv.Itoa(pid))
}

// Cleanup mata cualquier proceso que quede en el cgroup del contenedor, lo
// elimina y quita el nivel intermedio si queda vacío. Seguro con defer, incluso
// si el contenedor murió por kill -9.
func (m *Manager) Cleanup() error {
	if m.path == "" {
		return nil
	}
	m.killAll()
	err := removeWithRetry(m.path)
	// El nivel intermedio se elimina si no hay otros contenedores (best-effort).
	_ = os.Remove(m.parent)
	return err
}

// killAll fuerza la terminación de todo el árbol de procesos del cgroup.
func (m *Manager) killAll() {
	// cgroup.kill (kernel 5.14+) mata todo el subárbol de una sola escritura.
	if err := m.write("cgroup.kill", "1"); err == nil {
		return
	}
	// Fallback para kernels antiguos: SIGKILL a cada PID listado.
	pids, _ := readTokens(filepath.Join(m.path, "cgroup.procs"))
	for _, p := range pids {
		if pid, err := strconv.Atoi(p); err == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

// removeWithRetry hace rmdir reintentando ante EBUSY, que el kernel devuelve
// unos milisegundos mientras termina de vaciar el cgroup.
func removeWithRetry(path string) error {
	var err error
	for i := 0; i < 100; i++ {
		if err = os.Remove(path); err == nil || os.IsNotExist(err) {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("eliminar cgroup %s: %w", path, err)
}

// delegateControllers habilita memory y cpu en el cgroup.subtree_control de dir,
// para que los cgroups hijos puedan usarlos. Solo habilita los disponibles y
// aún no activos (idempotente).
func delegateControllers(dir string) error {
	available, err := readTokens(filepath.Join(dir, "cgroup.controllers"))
	if err != nil {
		return fmt.Errorf("leer cgroup.controllers de %s: %w", dir, err)
	}
	enabled, _ := readTokens(filepath.Join(dir, "cgroup.subtree_control"))

	var toAdd []string
	for _, c := range []string{"memory", "cpu"} {
		if Contains(available, c) && !Contains(enabled, c) {
			toAdd = append(toAdd, "+"+c)
		}
	}
	if len(toAdd) == 0 {
		return nil
	}
	data := strings.Join(toAdd, " ")
	if err := os.WriteFile(filepath.Join(dir, "cgroup.subtree_control"), []byte(data), 0o644); err != nil {
		return fmt.Errorf("delegar controladores %q en %s: %w", data, dir, err)
	}
	return nil
}

func (m *Manager) write(file, data string) error {
	p := filepath.Join(m.path, file)
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		return fmt.Errorf("escribir %s=%q: %w", file, data, err)
	}
	return nil
}

// readTokens lee un archivo y devuelve sus campos separados por espacios/saltos.
func readTokens(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(data)), nil
}
