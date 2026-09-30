//go:build !linux && !freebsd

package netcfg

import (
	"context"
	"fmt"
)

func unsupported() error                                                      { return fmt.Errorf("host networking is not supported on this platform") }
func (*Network) SetupBridge(context.Context) error                            { return unsupported() }
func (*Network) BridgeExists(context.Context) (bool, error)                   { return false, unsupported() }
func (*Network) SetupNAT(context.Context) error                               { return unsupported() }
func (*Network) CreateTap(context.Context, string, string) error              { return unsupported() }
func (*Network) DestroyTap(context.Context, string) error                     { return unsupported() }
func (*Network) TapExists(context.Context, string) (bool, error)              { return false, unsupported() }
func (*Network) PrepareJailedTap(context.Context, string, uint32) error       { return unsupported() }
func (*Network) BhyveTapDevice(context.Context, string) (string, error)       { return "", unsupported() }
func (*Network) AddPortForward(context.Context, uint16, string, uint16) error { return unsupported() }
func (*Network) RemovePortForwards(context.Context, string) error             { return unsupported() }
func (*Network) DenyOutgoing(context.Context, string) error                   { return unsupported() }
func (*Network) AllowOutgoing(context.Context, string) error                  { return unsupported() }
func (*Network) Isolate(context.Context, string, string) error                { return unsupported() }
func (*Network) RemoveIsolation(context.Context, string) error                { return unsupported() }
