package guestconfig

import (
	"context"
	"github.com/TTstack/tt/internal/model"
)

func WriteDisk(ctx context.Context, directory string, files model.GuestConfig) error {
	return writeDisk(ctx, directory, files)
}
