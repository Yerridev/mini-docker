//go:build !linux

// Stub para Windows/macOS: permite compilar el proyecto fuera de Linux.
// Los cgroups son una característica del kernel de Linux, así que aquí todas
// las operaciones son no-ops.
package cgroup

// New no crea nada fuera de Linux; devuelve un Manager inerte.
func New(id string) (*Manager, error) {
	_ = id
	return &Manager{}, nil
}

func (m *Manager) SetMemoryLimit(bytes int64) error { _ = bytes; return nil }

func (m *Manager) SetCPULimit(quota, period int64) error { _ = quota; _ = period; return nil }

func (m *Manager) AddProcess(pid int) error { _ = pid; return nil }

func (m *Manager) Cleanup() error { return nil }
