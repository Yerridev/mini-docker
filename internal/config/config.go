package config

type Config struct {
	Rootfs  string
	Command string
	Args    []string

	// Hito 3 — límites de recursos (cgroups v2). Valor 0 = sin límite.
	MemoryBytes int64 // memory.max, en bytes
	CPUQuota    int64 // cpu.max, numerador (microsegundos por período)
	CPUPeriod   int64 // cpu.max, denominador (microsegundos); 0 => 100000
}
