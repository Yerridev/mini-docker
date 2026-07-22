// Package cgroup aplica límites de recursos (memoria y CPU) a un contenedor
// usando cgroups v2. Toda la manipulación ocurre en el proceso PADRE
// (minidocker run), que vive en el namespace de montaje del host y por tanto
// ve /sys/fs/cgroup; el proceso hijo (init) nunca toca /sys.
package cgroup

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
