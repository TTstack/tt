//go:build !linux && !freebsd

package engine

import (
	"fmt"
	"github.com/TTstack/tt/internal/model"
)

func platformEngine(kind model.Engine, containerRuntime *ContainerRuntime) (Engine, error) {
	return nil, fmt.Errorf("engine %s is not supported on this platform", kind)
}
