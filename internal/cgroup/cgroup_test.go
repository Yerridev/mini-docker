package cgroup

import "testing"

func TestFormatCPUMax(t *testing.T) {
	cases := []struct {
		name          string
		quota, period int64
		want          string
	}{
		{"half_cpu", 50000, 100000, "50000 100000"},
		{"quarter_cpu", 25000, 100000, "25000 100000"},
		{"full_cpu", 100000, 100000, "100000 100000"},
		{"period_zero_uses_default", 50000, 0, "50000 100000"},
		{"period_negative_uses_default", 50000, -1, "50000 100000"},
		{"two_cpus", 200000, 100000, "200000 100000"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FormatCPUMax(c.quota, c.period)
			if got != c.want {
				t.Fatalf("FormatCPUMax(%d, %d) = %q, want %q", c.quota, c.period, got, c.want)
			}
		})
	}
}

func TestFormatCPUMaxQuotaZero(t *testing.T) {
	// La función sólo la llama SetCPULimit cuando quota > 0, pero FormatCPUMax
	// misma no filtra quota=0: debe devolver "0 period" para no enmascarar
	// un caller bug. Verificamos el contrato explícito.
	got := FormatCPUMax(0, 100000)
	if want := "0 100000"; got != want {
		t.Fatalf("FormatCPUMax(0, .) = %q, want %q", got, want)
	}
}

func TestContains(t *testing.T) {
	list := []string{"memory", "cpu", "io"}
	if !Contains(list, "cpu") {
		t.Errorf("Contains(%v, %q) = false, want true", list, "cpu")
	}
	if !Contains(list, "memory") {
		t.Errorf("Contains(%v, %q) = false, want true", list, "memory")
	}
	if Contains(list, "blkio") {
		t.Errorf("Contains(%v, %q) = true, want false", list, "blkio")
	}
	if Contains(nil, "cpu") {
		t.Errorf("Contains(nil, %q) = true, want false", "cpu")
	}
	if Contains(list, "") {
		t.Errorf("Contains(%v, \"\") = true, want false", list)
	}
}

func TestContainsEmptyList(t *testing.T) {
	if Contains([]string{}, "anything") {
		t.Errorf("Contains([], \"anything\") = true, want false")
	}
}

// TestSetMemoryLimitSkipsZero verifica el guard de bytes<=0 sin tocar /sys:
// SetMemoryLimit con 0 o negativo debe ser no-op y no error.
// En Linux llama m.write que requiere un Manager válido; en !linux es stub.
// Para no depender de /sys, testeamos el guard vía CheckMemoryLimit (helper
// puro). Si el guard llegase a romperse, capturamos acá.
func TestSetMemoryLimitZeroIsNoop(t *testing.T) {
	m := &Manager{}
	if err := m.SetMemoryLimit(0); err != nil {
		t.Fatalf("SetMemoryLimit(0) = %v, want nil (no-op sin /sys)", err)
	}
	if err := m.SetMemoryLimit(-100); err != nil {
		t.Fatalf("SetMemoryLimit(-100) = %v, want nil (no-op sin /sys)", err)
	}
}
