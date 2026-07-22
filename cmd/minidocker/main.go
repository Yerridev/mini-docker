package main

import (
	"flag"
	"fmt"
	"minidocker/internal/config"
	"minidocker/internal/container"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	initEnvKey = "MINIDOCKER_INIT"
	// Período por defecto de cpu.max (100 ms) en microsegundos.
	cpuPeriod = 100000
)

// stringSlice acumula el valor de un flag repetible (--env, --volume).
type stringSlice []string

func (s *stringSlice) String() string     { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error { *s = append(*s, v); return nil }

// Flags repetibles: son globales porque tanto el padre como el proceso init
// (hijo) los parsean con el mismo flag.FlagSet.
var (
	envFlags    stringSlice
	volumeFlags stringSlice
)

func main() {
	rootfs := flag.String("rootfs", "./rootfs", "path to the root filesystem")
	memory := flag.String("memory", "", "memory limit, e.g. 100m, 512k, 1g or raw bytes (0 = unlimited)")
	cpu := flag.Float64("cpu", 0, "cpu limit in cores, e.g. 0.5 (0 = unlimited)")
	flag.Var(&envFlags, "env", "environment variable KEY=VALUE (repeatable)")
	flag.Var(&volumeFlags, "volume", "bind mount /host:/container (repeatable)")
	flag.Parse()

	if isInit() {
		initContainer(*rootfs)
		return
	}

	memBytes, err := parseMemory(*memory)
	if err != nil {
		fatal(err)
	}
	env, err := parseEnv(envFlags)
	if err != nil {
		fatal(err)
	}
	volumes, err := parseVolumes(volumeFlags)
	if err != nil {
		fatal(err)
	}

	var cpuQuota int64
	if *cpu > 0 {
		cpuQuota = int64(*cpu * float64(cpuPeriod))
	}

	runContainer(*rootfs, memBytes, cpuQuota, env, volumes)
}

func isInit() bool {
	return os.Getenv(initEnvKey) == "1"
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func initContainer(rootfs string) {
	// argv del hijo: [minidocker-init --rootfs X --volume a:b ...] init cmd args
	// flag.Parse ya consumió los flags; flag.Args() = ["init", command, ...].
	args := flag.Args()
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: minidocker run <command> [args...]")
		os.Exit(1)
	}

	// El hijo necesita los volúmenes para hacer los bind mounts en setup.
	volumes, err := parseVolumes(volumeFlags)
	if err != nil {
		fatal(err)
	}

	cfg := &config.Config{
		Rootfs:  rootfs,
		Command: args[1],
		Args:    args[2:],
		Volumes: volumes,
	}

	if err := container.ExecInit(cfg); err != nil {
		fatal(err)
	}
}

func runContainer(rootfs string, memBytes, cpuQuota int64, env []string, volumes []config.Volume) {
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: minidocker [--rootfs <path>] [--memory <n>] [--cpu <n>] [--env K=V] [--volume /h:/c] run <command> [args...]")
		os.Exit(1)
	}

	if args[0] != "run" {
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", args[0])
		os.Exit(1)
	}

	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: minidocker run <command> [args...]")
		os.Exit(1)
	}

	// Ruta absoluta: el proceso init debe resolver el mismo rootfs
	// sin depender del directorio de trabajo.
	absRootfs := rootfs
	if rootfs != "" {
		abs, err := filepath.Abs(rootfs)
		if err != nil {
			fatal(fmt.Errorf("rootfs %q: %w", rootfs, err))
		}
		absRootfs = abs
	}

	cfg := &config.Config{
		Rootfs:      absRootfs,
		Command:     args[1],
		Args:        args[2:],
		MemoryBytes: memBytes,
		CPUQuota:    cpuQuota,
		CPUPeriod:   cpuPeriod,
		Env:         env,
		Volumes:     volumes,
	}

	if err := container.New(cfg).Run(); err != nil {
		fatal(err)
	}
}

// parseEnv valida que cada --env tenga forma KEY=VALUE.
func parseEnv(items []string) ([]string, error) {
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

// parseVolumes convierte cada "/host:/contenedor" en config.Volume, con la
// ruta de host normalizada a absoluta. El destino debe ser absoluto.
func parseVolumes(items []string) ([]config.Volume, error) {
	out := make([]config.Volume, 0, len(items))
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
		out = append(out, config.Volume{Source: absSrc, Target: dst})
	}
	return out, nil
}

// parseMemory convierte "100m", "512k", "1g" o un número de bytes a int64.
// Cadena vacía => 0 (sin límite). Sufijos binarios (1k = 1024).
func parseMemory(s string) (int64, error) {
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
