// Package agent implements the TTstack host agent.
package agent

import (
	_ "embed"
	"flag"
	"io"
	"os"

	"github.com/TTstack/tt/internal/commandline"
)

//go:embed help.txt
var help string

type Config struct {
	EnableDiskBackup                                                      bool
	ContainerRuntime, SSHPublicAddress, SSHIngressNetNS, SSHIngressTarget string
	PortStart, PortEnd                                                    uint
	Listen, ImageDir, RuntimeDir, DataDir, Storage                        string
	CPUTotal, MemTotal, DiskTotal                                         uint
	HostID, APIKey                                                        string
}

func Run(args []string, stdout, stderr io.Writer) int {
	var cfg Config
	fs := flag.NewFlagSet("tt-agent", flag.ContinueOnError)
	fs.BoolVar(&cfg.EnableDiskBackup, "enable-disk-backup", false, "Enable explicit disk backup admission")
	fs.StringVar(&cfg.ContainerRuntime, "container-runtime", os.Getenv("TT_CONTAINER_RUNTIME"), "Container runtime for this inventory")
	fs.StringVar(&cfg.SSHPublicAddress, "ssh-public-address", "", "Client-reachable IPv4 address")
	fs.StringVar(&cfg.SSHIngressNetNS, "ssh-ingress-netns", "", "Outer network namespace")
	fs.StringVar(&cfg.SSHIngressTarget, "ssh-ingress-target", "", "Agent namespace address")
	fs.UintVar(&cfg.PortStart, "port-start", 20000, "First TCP host port")
	fs.UintVar(&cfg.PortEnd, "port-end", 65535, "Last TCP host port")
	fs.StringVar(&cfg.Listen, "listen", "0.0.0.0:9100", "Listen address for the HTTP API")
	fs.StringVar(&cfg.ImageDir, "image-dir", "/home/ttstack/images", "Base image directory or dataset")
	fs.StringVar(&cfg.RuntimeDir, "runtime-dir", "/home/ttstack/runtime", "Runtime clone directory or dataset")
	fs.StringVar(&cfg.DataDir, "data-dir", "/home/ttstack/data", "Directory for persistent state")
	fs.StringVar(&cfg.Storage, "storage", "file", "Storage backend")
	fs.UintVar(&cfg.CPUTotal, "cpu-total", 0, "CPU capacity; 0 = auto-detect")
	fs.UintVar(&cfg.MemTotal, "mem-total", 0, "Memory capacity MiB; 0 = auto-detect")
	fs.UintVar(&cfg.DiskTotal, "disk-total", 204800, "Disk scheduling budget MiB")
	fs.StringVar(&cfg.HostID, "host-id", "", "Unique host identifier")
	fs.StringVar(&cfg.APIKey, "api-key", os.Getenv("TT_API_KEY"), "API key for authentication")
	return commandline.Stub(fs, args, help, stdout, stderr)
}
