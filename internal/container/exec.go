package container

import (
	"fmt"
	"minidocker/internal/config"
)

func ExecInit(cfg *config.Config) error {
	if err := setupContainer(cfg); err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	return execContainer(cfg)
}
