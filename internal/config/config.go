package config

import (
	"fmt"
	"minidocker/internal/netns"
	"path/filepath"
	"strconv"
	"strings"
)

// Volume es un bind mount de una ruta del host a una del contenedor (Hito 4).
type Volume struct {
	Source string // ruta en el host
	Target string // ruta absoluta dentro del contenedor
}

type Config struct {
	Rootfs   string
	Command  string
	Args     []string
	Hostname string

	// Hito 3 — límites de recursos (cgroups v2). Valor 0 = sin límite.
	MemoryBytes int64 // memory.max, en bytes
	CPUQuota    int64 // cpu.max, numerador (microsegundos por período)
	CPUPeriod   int64 // cpu.max, denominador (microsegundos); 0 => 100000

	// Hito 4 — entorno y volúmenes.
	Env     []string // variables "KEY=VALUE" para el proceso del contenedor
	Volumes []Volume // bind mounts host -> contenedor

	// Hito 5 — red aislada.
	NetMode netns.Mode // loopback (default), none, veth
}

// ParseEnv valida que cada --env tenga forma KEY=VALUE y lo devuelve tal cual.
// Cadena vacía o clave faltante son error. Las entradas se devuelven sin
// normalización para preservar el valor literal pasado al exec del contenedor.
func ParseEnv(items []string) ([]string, error) {
	out := make([]string, 0, len(items))
	for _, e := range items {
		k, _, ok := strings.Cut(e, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("--env inválido %q (esperado KEY=VALUE)", e)
		}
		out = append(out, e)
	}
	return out, nil
}

// ParseVolumes convierte cada "/host:/contenedor" en Volume, normalizando la
// ruta de host a absoluta (el proceso init puede correr en otro cwd). El
// destino debe ser absoluto (relativo al rootfs del contenedor).
func ParseVolumes(items []string) ([]Volume, error) {
	out := make([]Volume, 0, len(items))
	for _, v := range items {
		src, dst, ok := strings.Cut(v, ":")
		if !ok || src == "" || dst == "" {
			return nil, fmt.Errorf("--volume inválido %q (esperado /host:/contenedor)", v)
		}
		absSrc, err := filepath.Abs(src)
		if err != nil {
			return nil, fmt.Errorf("--volume %q: %w", v, err)
		}
		if !strings.HasPrefix(dst, "/") {
			return nil, fmt.Errorf("--volume %q: el destino debe ser una ruta absoluta", v)
		}
		out = append(out, Volume{Source: absSrc, Target: dst})
	}
	return out, nil
}

// ParseMemory convierte "100m", "512k", "1g" o un número de bytes a int64.
// Cadena vacía => 0 (sin límite). Sufijos binarios (1k = 1024).
func ParseMemory(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}

	mult := int64(1)
	switch s[len(s)-1] {
	case 'k', 'K':
		mult, s = 1024, s[:len(s)-1]
	case 'm', 'M':
		mult, s = 1024*1024, s[:len(s)-1]
	case 'g', 'G':
		mult, s = 1024*1024*1024, s[:len(s)-1]
	}

	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("valor de memoria inválido: %q", s)
	}
	if n < 0 {
		return 0, fmt.Errorf("valor de memoria negativo: %q", s)
	}
	return n * mult, nil
}
