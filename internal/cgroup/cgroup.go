// Package cgroup aplica límites de recursos (memoria y CPU) a un contenedor
// usando cgroups v2. Toda la manipulación ocurre en el proceso PADRE
// (minidocker run), que vive en el namespace de montaje del host y por tanto
// ve /sys/fs/cgroup; el proceso hijo (init) nunca toca /sys.
package cgroup

import "fmt"

// defaultPeriod es el período por defecto para cpu.max (100 ms), en
// microsegundos. Definido aquí (sin build tag) para que FormatCPUMax sea
// testeable en cualquier plataforma.
const defaultPeriod = 100000

// Manager administra un cgroup v2 dedicado a un contenedor.
// El valor cero no es utilizable: usar New().
type Manager struct {
	// path: cgroup del contenedor, ej: /sys/fs/cgroup/minidocker/<id>.
	path string
	// parent: nivel intermedio /sys/fs/cgroup/minidocker.
	parent string
	id     string
}

// Path devuelve la ruta del cgroup del contenedor (para diagnóstico).
func (m *Manager) Path() string { return m.path }

// FormatCPUMax devuelve el contenido de cpu.max ("quota period", microsegundos).
// Con quota=50000 y period=100000 el contenedor obtiene 0.5 CPUs. Extraída a este
// archivo común (sin build tag) para poder testearla en cualquier plataforma.
func FormatCPUMax(quota, period int64) string {
	if period <= 0 {
		period = defaultPeriod
	}
	return fmt.Sprintf("%d %d", quota, period)
}

// Contains reporta si s está en list. Definida acá (sin build tag) para ser
// testeable desde cualquier plataforma; los archivos *_linux.go la usan.
func Contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
