package config

// Volume es un bind mount de una ruta del host a una del contenedor (Hito 4).
type Volume struct {
	Source string // ruta en el host
	Target string // ruta absoluta dentro del contenedor
}

type Config struct {
	Rootfs  string
	Command string
	Args    []string

	// Hito 3 — límites de recursos (cgroups v2). Valor 0 = sin límite.
	MemoryBytes int64 // memory.max, en bytes
	CPUQuota    int64 // cpu.max, numerador (microsegundos por período)
	CPUPeriod   int64 // cpu.max, denominador (microsegundos); 0 => 100000

	// Hito 4 — entorno y volúmenes.
	Env     []string // variables "KEY=VALUE" para el proceso del contenedor
	Volumes []Volume // bind mounts host -> contenedor
}
