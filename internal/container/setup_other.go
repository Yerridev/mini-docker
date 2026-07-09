//go:build !linux

package container

import "minidocker/internal/config"

func setupContainer(cfg *config.Config) error {
	_ = cfg
	return nil
}
