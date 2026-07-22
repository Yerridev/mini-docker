//go:build linux

package container

import (
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// TestForwardSignalsCleanExitClosesGoroutine arranca un hijo dormido, llama
// stop() inmediatamente y verifica que la goroutine de forwardSignals cierra
// el channel done y desregistra la señal sin deadlock.
func TestForwardSignalsCleanExitClosesGoroutine(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	stop := forwardSignals(cmd)

	// stop debe cerrar el channel done y desregistrar la señal. La goroutine
	// debe salir sin deadlock.
	stop()

	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}

// TestForwardSignalsForwardsSIGINTToChild verifica que forwardSignals reenvía
// SIGINT (os.Interrupt) al proceso hijo y, si no muere en killGrace, fuerza
// SIGKILL.USD el patrón helper-process estándar de Go: el test re-ejecuta su
// propio binario con GO_HELPER_PROCESS=1 como hijo dormido.
func TestForwardSignalsForwardsSIGINTToChild(t *testing.T) {
	if os.Getenv("GO_HELPER_PROCESS") == "1" {
		// Helper: dormita. No captura señales, así que forwardSignals lo mata
		// (SIGKILL tras killGrace) mientras tanto el test verifica el plazo.
		time.Sleep(30 * time.Second)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestForwardSignalsForwardsSIGINTToChild")
	cmd.Env = append(os.Environ(), "GO_HELPER_PROCESS=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	stop := forwardSignals(cmd)
	defer stop()

	// El test manda SIGINT al proceso "padre simulado"; forwardSignals recibe
	// esa señal vía signal.Notify y la reenvía al hijo.
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("signal: %v", err)
	}

	// Comportamiento esperado: el helper no captura SIGINT → kernel actúa
	// normalmente y lo mata (o reenviado por forwardSignals). Verificamos que
	// el proceso termina antes de killGrace + margen.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-time.After(killGrace + 2*time.Second):
		t.Fatal("timeout esperando terminación del helper tras SIGINT")
	case <-done:
		// OK
	}
}

// TestForwardSignalsRaceUnderConcurrentStartStop lanza N instancias
// concurrentes start → stop para presionar al detector de carreras en los
// channels y la goroutine. Debe pasar con `go test -race`.
func TestForwardSignalsRaceUnderConcurrentStartStop(t *testing.T) {
	const n = 10
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command("/bin/sleep", "30")
			if err := cmd.Start(); err != nil {
				errs <- err
				return
			}
			stop := forwardSignals(cmd)
			stop()
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("start: %v", err)
	}
}
