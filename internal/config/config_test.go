package config

import (
	"path/filepath"
	"testing"
)

func TestParseMemory(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"", 0, false},      // sin límite
		{"0", 0, false},     // sin límite explícito
		{"100", 100, false}, // bytes puros
		{"100b", 0, true},   // sufijo b no soportado
		{"512k", 524288, false},
		{"1m", 1048576, false},
		{"100m", 104857600, false},
		{"1g", 1073741824, false},
		{"2g", 2147483648, false},
		// case insensitivo
		{"1K", 1024, false},
		{"1M", 1048576, false},
		{"1G", 1073741824, false},
		// espacios tolerados
		{"  64m  ", 67108864, false},
		// casos borde de error
		{"-5", 0, true},
		{"abc", 0, true},
		{"1.5g", 0, true},
		{"g", 0, true},
		{"k", 0, true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseMemory(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("ParseMemory(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			}
			if !c.wantErr && got != c.want {
				t.Fatalf("ParseMemory(%q) = %d, want %d", c.in, got, c.want)
			}
			if c.wantErr && err == nil {
				t.Fatalf("ParseMemory(%q) err=nil, want error", c.in)
			}
		})
	}
}

func TestParseMemoryIntegerOverflow(t *testing.T) {
	// Un valor absurdamente grande debe dar error de ParseInt (overflow int64),
	// no silenciosamente cero. Verificamos que el error se propague.
	_, err := ParseMemory("99999999999999999999g")
	if err == nil {
		t.Fatal("ParseMemory(huge) err=nil, want overflow error")
	}
}

func TestParseEnv(t *testing.T) {
	t.Run("valid_single", func(t *testing.T) {
		got, err := ParseEnv([]string{"FOO=bar"})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(got) != 1 || got[0] != "FOO=bar" {
			t.Fatalf("got=%v, want [FOO=bar]", got)
		}
	})

	t.Run("valid_multiple", func(t *testing.T) {
		got, err := ParseEnv([]string{"A=1", "B=2", "C=3"})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(got) != 3 {
			t.Fatalf("len=%d, want 3", len(got))
		}
	})

	t.Run("value_can_be_empty", func(t *testing.T) {
		// KEY= es válido (clave presente,Valor vacío): es un patrón para unset
		// o setear variable vacía. El contrato sólo exige clave no vacía.
		got, err := ParseEnv([]string{"EMPTY="})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got[0] != "EMPTY=" {
			t.Fatalf("got=%v, want [EMPTY=]", got)
		}
	})

	t.Run("value_can_contain_equals", func(t *testing.T) {
		// La primera '=' separa clave del resto; el valor puede tener más '='.
		got, err := ParseEnv([]string{"PATH=/usr/bin:/bin"})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got[0] != "PATH=/usr/bin:/bin" {
			t.Fatalf("got=%v, want intact", got)
		}
	})

	t.Run("missing_equals_is_error", func(t *testing.T) {
		if _, err := ParseEnv([]string{"NOEQUALS"}); err == nil {
			t.Fatal("err=nil, want error for missing '='")
		}
	})

	t.Run("empty_key_is_error", func(t *testing.T) {
		if _, err := ParseEnv([]string{"=value"}); err == nil {
			t.Fatal("err=nil, want error for empty key")
		}
	})

	t.Run("empty_input_returns_empty", func(t *testing.T) {
		got, err := ParseEnv(nil)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(got) != 0 {
			t.Fatalf("len=%d, want 0", len(got))
		}
	})
}

func TestParseVolumes(t *testing.T) {
	t.Run("valid_absolute_target", func(t *testing.T) {
		got, err := ParseVolumes([]string{"/tmp:/data"})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(got) != 1 || got[0].Target != "/data" {
			t.Fatalf("got=%+v, want target=/data", got)
		}
		// Source se normaliza a ruta absoluta vía filepath.Abs — no asertamos
		// valor exacto porque depende del cwd del test.
		if got[0].Source == "" {
			t.Fatal("source vacío tras Abs")
		}
	})

	t.Run("valid_relative_source_normalized_to_absolute", func(t *testing.T) {
		got, err := ParseVolumes([]string{"relpath:/mnt"})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if !filepath.IsAbs(got[0].Source) {
			t.Fatalf("source no absoluto: %q", got[0].Source)
		}
	})

	t.Run("missing_colon_is_error", func(t *testing.T) {
		if _, err := ParseVolumes([]string{"nopath"}); err == nil {
			t.Fatal("err=nil, want error for missing ':'")
		}
	})

	t.Run("empty_source_is_error", func(t *testing.T) {
		if _, err := ParseVolumes([]string{":/data"}); err == nil {
			t.Fatal("err=nil, want error for empty source")
		}
	})

	t.Run("empty_target_is_error", func(t *testing.T) {
		if _, err := ParseVolumes([]string{"/tmp:"}); err == nil {
			t.Fatal("err=nil, want error for empty target")
		}
	})

	t.Run("relative_target_is_error", func(t *testing.T) {
		// El destino debe ser absoluto dentro del rootfs del contenedor;
		// relativo es ambiguo y se rechaza temprano.
		if _, err := ParseVolumes([]string{"/tmp:relative"}); err == nil {
			t.Fatal("err=nil, want error for relative target")
		}
	})

	t.Run("multiple_volumes", func(t *testing.T) {
		got, err := ParseVolumes([]string{"/a:/b", "/c:/d", "/e:/f"})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(got) != 3 {
			t.Fatalf("len=%d, want 3", len(got))
		}
	})

	t.Run("empty_input_returns_empty", func(t *testing.T) {
		got, err := ParseVolumes(nil)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(got) != 0 {
			t.Fatalf("len=%d, want 0", len(got))
		}
	})
}
