//go:build !linux

package netns

// Setup es un no-op en sistemas que no son Linux. El namespace NET no existe
// fuera de Linux, por lo que no hay nada que configurar.
func Setup(mode Mode) error {
	return nil
}
