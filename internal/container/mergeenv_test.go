package container

import (
	"strings"
	"testing"
)

func TestMergeEnvOverrideByExtra(t *testing.T) {
	base := []string{"PATH=/usr/bin", "HOME=/root", "USER=jhon"}
	extra := []string{"PATH=/bin", "FOO=bar"}

	got := mergeEnv(base, extra)

	gotMap := envToMap(t, got)

	if gotMap["PATH"] != "/bin" {
		t.Errorf("PATH = %q, want /bin (override por --env)", gotMap["PATH"])
	}
	if gotMap["HOME"] != "/root" {
		t.Errorf("HOME = %q, want /root", gotMap["HOME"])
	}
	if gotMap["USER"] != "jhon" {
		t.Errorf("USER = %q, want jhon", gotMap["USER"])
	}
	if gotMap["FOO"] != "bar" {
		t.Errorf("FOO = %q, want bar", gotMap["FOO"])
	}

	pathCount := 0
	for _, e := range got {
		if k, _, ok := strings.Cut(e, "="); ok && k == "PATH" {
			pathCount++
		}
	}
	if pathCount != 1 {
		t.Errorf("PATH aparece %d veces, want 1 (sin duplicados)", pathCount)
	}
}

func TestMergeEnvEmptyExtraReturnsBaseUnchanged(t *testing.T) {
	base := []string{"A=1", "B=2"}
	got := mergeEnv(base, nil)
	if len(got) != 2 || got[0] != "A=1" || got[1] != "B=2" {
		t.Fatalf("got=%v, want %v (early return sin tocar base)", got, base)
	}
}

func TestMergeEnvEmptyBaseReturnsExtra(t *testing.T) {
	got := mergeEnv(nil, []string{"X=1", "Y=2"})
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}
}

func TestMergeEnvOverrideMultiple(t *testing.T) {
	base := []string{"A=1", "B=2", "C=3"}
	extra := []string{"B=22", "C=33"}
	got := mergeEnv(base, extra)
	m := envToMap(t, got)
	if m["A"] != "1" {
		t.Errorf("A = %q, want 1 (conservada)", m["A"])
	}
	if m["B"] != "22" {
		t.Errorf("B = %q, want 22 (override)", m["B"])
	}
	if m["C"] != "33" {
		t.Errorf("C = %q, want 33 (override)", m["C"])
	}
}

// envToMap convierte ["K=V", ...] en map[string]string para asertos por clave.
// Falla el test si hay clave duplicada — queremos exactamente una entrada por K.
func envToMap(t *testing.T, env []string) map[string]string {
	t.Helper()
	m := make(map[string]string, len(env))
	for _, e := range env {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			t.Fatalf("entrada sin '=': %q", e)
		}
		if _, exists := m[k]; exists {
			t.Fatalf("clave duplicada en mergeEnv: %q", k)
		}
		m[k] = v
	}
	return m
}
