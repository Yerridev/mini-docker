package main

import (
	"flag"
	"fmt"
	"minidocker/internal/config"
	"minidocker/internal/container"
	"os"
	"path/filepath"
)

const (
	initEnvKey = "MINIDOCKER_INIT"
)

func main() {
	rootfs := flag.String("rootfs", "./rootfs", "path to the root filesystem")
	flag.Parse()

	if isInit() {
		initContainer(*rootfs)
		return
	}

	runContainer(*rootfs)
}

func isInit() bool {
	return os.Getenv(initEnvKey) == "1"
}

func initContainer(rootfs string) {
	// argv: [minidocker-init --rootfs <path>] init <command> [args...]
	// flag.Parse ya consumió --rootfs; flag.Args() = ["init", command, ...]
	args := flag.Args()
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: minidocker run <command> [args...]")
		os.Exit(1)
	}

	cfg := &config.Config{
		Rootfs:  rootfs,
		Command: args[1],
		Args:    args[2:],
	}

	if err := container.ExecInit(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runContainer(rootfs string) {
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: minidocker [--rootfs <path>] run <command> [args...]")
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
			fmt.Fprintf(os.Stderr, "error: rootfs %q: %v\n", rootfs, err)
			os.Exit(1)
		}
		absRootfs = abs
	}

	cfg := &config.Config{
		Rootfs:  absRootfs,
		Command: args[1],
		Args:    args[2:],
	}

	c := container.New(cfg)
	if err := c.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
