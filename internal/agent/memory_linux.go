package agent

import (
	"context"
	"os"
	"unicode/utf8"
)

func totalMemory(context.Context) (uint32, bool) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil || !utf8.Valid(data) {
		return 0, false
	}
	return parseMemTotal(string(data))
}
