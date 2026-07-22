package container

import (
	"fmt"
	"minidocker/internal/config"
	"os"
	"path/filepath"
	"syscall"
)

func sethostname(name string) error {
	if err := syscall.Sethostname([]byte(name)); err != nil {
		return fmt.Errorf("sethostname: %w", err)
	}
	return nil
}

// makeMountsPrivate marca todo el árbol de montajes como privado.
// En hosts con propagación "shared" (systemd es el caso típico), los
// montajes hechos dentro del container se FUGARÍAN al host sin esto.
// También es requisito para que pivot_root(2) acepte el nuevo root.
func makeMountsPrivate() error {
	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("remount / privado: %w", err)
	}
	return nil
}

// validRootfs reporta si path parece un root filesystem utilizable
// (existe y contiene un directorio bin/).
func validRootfs(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(path, "bin"))
	return err == nil && st.IsDir()
}

// prepareRootfs convierte el rootfs en un mount point (bind sobre sí
// mismo). pivot_root(2) exige que new_root sea un mount point.
func prepareRootfs(path string) error {
	if err := syscall.Mount(path, path, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind-mount rootfs %s: %w", path, err)
	}
	return nil
}

// chrootRootfs cambia la raíz del proceso con chroot(2) (Hito 2, tarea 2).
func chrootRootfs(path string) error {
	// Guardar el directorio actual ANTES de chroot: tras el cambio de
	// raíz ya no existe forma de recuperarlo desde dentro.
	if _, err := os.Getwd(); err != nil {
		return fmt.Errorf("getwd antes de chroot: %w", err)
	}
	if err := syscall.Chroot(path); err != nil {
		return fmt.Errorf("chroot %s: %w", path, err)
	}
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir a la nueva raíz: %w", err)
	}
	return nil
}

// pivotRootfs cambia la raíz con pivot_root(2) (Hito 2, tarea 3 —
// nivel sobresaliente). A diferencia de chroot, el root viejo del host
// se desmonta por completo: no queda NINGUNA vía de escape.
func pivotRootfs(path string) error {
	oldroot := filepath.Join(path, ".oldroot")
	if err := os.MkdirAll(oldroot, 0o700); err != nil {
		return fmt.Errorf("crear %s: %w", oldroot, err)
	}
	if err := syscall.PivotRoot(path, oldroot); err != nil {
		return fmt.Errorf("pivot_root %s: %w", path, err)
	}
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir a la nueva raíz: %w", err)
	}
	// Desmontar y eliminar el root viejo: el host queda invisible.
	if err := syscall.Unmount("/.oldroot", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount /.oldroot: %w", err)
	}
	if err := os.Remove("/.oldroot"); err != nil {
		return fmt.Errorf("eliminar /.oldroot: %w", err)
	}
	return nil
}

// mountProc monta un /proc propio. Debe ocurrir DESPUÉS de
// chroot/pivot_root para que ps y /proc reflejen el PID namespace
// del container dentro de su nuevo rootfs (Hito 2, tarea 4).
func mountProc() error {
	if err := os.MkdirAll("/proc", 0o555); err != nil {
		return fmt.Errorf("crear /proc: %w", err)
	}
	flags := uintptr(syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC)
	if err := syscall.Mount("proc", "/proc", "proc", flags, ""); err != nil {
		return fmt.Errorf("mount /proc: %w", err)
	}
	return nil
}

// mountVolumes monta los bind mounts host->contenedor (Hito 4). Debe ocurrir
// ANTES del pivot_root: la ruta de origen es del host y desaparece cuando el
// root viejo se desmonta. El destino se crea dentro del rootfs (si lo hay).
func mountVolumes(cfg *config.Config) error {
	for _, v := range cfg.Volumes {
		dest := v.Target
		if validRootfs(cfg.Rootfs) {
			dest = filepath.Join(cfg.Rootfs, v.Target)
		}
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return fmt.Errorf("crear destino de volumen %s: %w", dest, err)
		}
		if err := syscall.Mount(v.Source, dest, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
			return fmt.Errorf("bind-mount volumen %s -> %s: %w", v.Source, dest, err)
		}
	}
	return nil
}

func setupContainer(cfg *config.Config) error {
	// FASE 1 — UTS: cambiar hostname dentro del namespace
	if err := sethostname("minidocker"); err != nil {
		return err
	}

	// FASE 2 — MNT: aislar la propagación de montajes (sin fugas al host)
	if err := makeMountsPrivate(); err != nil {
		return err
	}

	// HITO 4 — MNT: volúmenes (bind mounts) ANTES del cambio de raíz.
	if err := mountVolumes(cfg); err != nil {
		return err
	}

	// HITO 2 — MNT: rootfs propio con pivot_root (fallback: chroot)
	if validRootfs(cfg.Rootfs) {
		if err := prepareRootfs(cfg.Rootfs); err != nil {
			return err
		}
		if err := pivotRootfs(cfg.Rootfs); err != nil {
			// pivot_root puede fallar según el filesystem (p. ej. NFS
			// o rootfs iniciales); chroot como plan B.
			fmt.Fprintf(os.Stderr, "[minidocker] pivot_root no disponible (%v); usando chroot\n", err)
			if err := chrootRootfs(cfg.Rootfs); err != nil {
				return err
			}
		}
	} else if cfg.Rootfs != "" {
		fmt.Fprintf(os.Stderr, "[minidocker] aviso: rootfs %q no existe o no contiene bin/; ejecutando sin chroot (modo Hito 1)\n", cfg.Rootfs)
	}

	// FASE 3 — MNT + PID: montar /proc propio, ya dentro del nuevo root.
	// Dentro del nuevo MNT namespace, este mount NO afecta al host.
	if err := mountProc(); err != nil {
		return err
	}

	return nil
}
