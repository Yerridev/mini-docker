//go:build linux

package netns

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"testing"
)

// TestMain implementa el patrón helper-process: cuando el test se re-ejecuta
// con GO_NETNS_HELPER=loopback, levanta "lo" dentro de un namespace NET nuevo
// y verifica que la interfaz reporta FlagUp.
func TestMain(m *testing.M) {
	if os.Getenv("GO_NETNS_HELPER") == "loopback" {
		if err := setLinkUp("lo"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		ifi, err := net.InterfaceByName("lo")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if ifi.Flags&net.FlagUp == 0 {
			fmt.Fprintln(os.Stderr, "loopback no está UP")
			os.Exit(1)
		}
		fmt.Println("loopback-up")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestSetLinkUpLoopback arranca un helper con CLONE_NEWNET y verifica que
// setLinkUp("lo") deja la interfaz loopback levantada.
func TestSetLinkUpLoopback(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requiere root (CAP_NET_ADMIN)")
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestSetLinkUpLoopback")
	cmd.Env = append(os.Environ(), "GO_NETNS_HELPER=loopback")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: 0x40000000, // CLONE_NEWNET
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper failed: %v\n%s", err, out)
	}
	if string(out) != "loopback-up\n" {
		t.Fatalf("salida inesperada: %q", out)
	}
}
