package main

import (
	"flag"
	"fmt"
	"minidocker/internal/config"
	"minidocker/internal/container"
	"os"
	"strconv"
	"strings"
)

const (
	initEnvKey = "MINIDOCKER_INIT"
	// Período por defecto de cpu.max (100 ms) en microsegundos.
	cpuPeriod = 100000
)

func main() {
	rootfs := flag.String("rootfs", "./rootfs", "path to the root filesystem")
	memory := flag.String("memory", "", "memory limit, e.g. 100m, 512k, 1g or raw bytes (0 = unlimited)")
	cpu := flag.Float64("cpu", 0, "cpu limit in cores, e.g. 0.5 (0 = unlimited)")
	flag.Parse()

	if isInit() {
		initContainer(*rootfs)
		return
	}

	memBytes, err := parseMemory(*memory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var cpuQuota int64
	if *cpu > 0 {
		cpuQuota = int64(*cpu * float64(cpuPeriod))
	}

	runContainer(*rootfs, memBytes, cpuQuota)
}

func isInit() bool {
	return os.Getenv(initEnvKey) == "1"
}

func initContainer(rootfs string) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: minidocker run <command> [args...]")
		os.Exit(1)
	}

	cfg := &config.Config{
		Rootfs:  rootfs,
		Command: os.Args[2],
		Args:    os.Args[3:],
	}

	if err := container.ExecInit(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runContainer(rootfs string, memBytes, cpuQuota int64) {
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: minidocker [--rootfs <path>] [--memory <n>] [--cpu <n>] run <command> [args...]")
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

	cfg := &config.Config{
		Rootfs:      rootfs,
		Command:     args[1],
		Args:        args[2:],
		MemoryBytes: memBytes,
		CPUQuota:    cpuQuota,
		CPUPeriod:   cpuPeriod,
	}

	c := container.New(cfg)
	if err := c.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
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
