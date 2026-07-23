//go:build linux

package netns

import (
	"encoding/binary"
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

// Setup configura la red del contenedor según el modo solicitado.
// Se ejecuta dentro del proceso init, ya dentro del namespace NET.
func Setup(mode Mode) error {
	switch mode {
	case ModeNone:
		return nil
	case ModeLoopback, ModeVeth:
		if err := setLinkUp("lo"); err != nil {
			return fmt.Errorf("levantar loopback: %w", err)
		}
	}
	// El modo veth requiere configuración adicional del lado del host
	// (crear par, mover un extremo al namespace, asignar IPs, NAT).
	// Se deja como extensión bonus del Hito 5.
	return nil
}

// setLinkUp levanta una interfaz de red usando ioctl SIOCSIFFLAGS.
// Fallback a `ip link set <name> up` si el socket/ioctl no está disponible.
func setLinkUp(name string) error {
	if len(name) >= 16 {
		return fmt.Errorf("nombre de interfaz %q demasiado largo", name)
	}

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return fallbackLinkUp(name, fmt.Errorf("socket: %w", err))
	}
	defer syscall.Close(fd)

	const ifreqSize = 40
	var req [ifreqSize]byte
	copy(req[:], name)

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.SIOCGIFFLAGS, uintptr(unsafe.Pointer(&req[0])))
	if errno != 0 {
		return fallbackLinkUp(name, fmt.Errorf("SIOCGIFFLAGS %s: %w", name, errno))
	}

	flags := binary.LittleEndian.Uint16(req[16:18])
	flags |= uint16(syscall.IFF_UP | syscall.IFF_RUNNING)
	binary.LittleEndian.PutUint16(req[16:18], flags)

	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.SIOCSIFFLAGS, uintptr(unsafe.Pointer(&req[0])))
	if errno != 0 {
		return fallbackLinkUp(name, fmt.Errorf("SIOCSIFFLAGS %s: %w", name, errno))
	}
	return nil
}

func fallbackLinkUp(name string, reason error) error {
	if out, err := exec.Command("ip", "link", "set", name, "up").CombinedOutput(); err != nil {
		return fmt.Errorf("%v; ip fallback: %w (%s)", reason, err, out)
	}
	return nil
}
