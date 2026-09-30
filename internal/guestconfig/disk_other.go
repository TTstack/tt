//go:build !linux

package guestconfig

import (
	"context"
	"fmt"
	"github.com/TTstack/tt/internal/model"
)

func WriteDisk(ctx context.Context, directory string, files model.GuestConfig) error {
	return fmt.Errorf("guest configuration disks are not supported on this platform")
}
