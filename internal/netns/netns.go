// Package netns configura la red del contenedor dentro de su namespace NET.
// Contiene tipos y funciones portables; la implementación real vive en
// netns_linux.go y los stubs para otros SO en netns_other.go.
package netns

import (
	"fmt"
	"strings"
)

// Mode representa el modo de red solicitado para el contenedor.
type Mode int

const (
	// ModeNone deshabilita el setup de red: el contenedor conserva la
	// interfaz loopback creada por el kernel pero no se levanta.
	ModeNone Mode = iota
	// ModeLoopback levanta la interfaz loopback dentro del namespace NET.
	ModeLoopback
	// ModeVeth configura un par veth host↔contenedor (bonus, no implementado
	// en la versión mínima).
	ModeVeth
)

// ParseMode convierte una cadena como "loopback", "none" o "veth" en Mode.
// Valor por defecto: ModeLoopback.
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "loopback":
		return ModeLoopback, nil
	case "none":
		return ModeNone, nil
	case "veth":
		return ModeVeth, nil
	default:
		return ModeLoopback, fmt.Errorf("modo de red inválido %q (loopback, none, veth)", s)
	}
}

// String devuelve la representación textual del modo.
func (m Mode) String() string {
	switch m {
	case ModeNone:
		return "none"
	case ModeLoopback:
		return "loopback"
	case ModeVeth:
		return "veth"
	default:
		return fmt.Sprintf("Mode(%d)", m)
	}
}
