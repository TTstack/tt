package commandline

import (
	"flag"
	"strconv"
	"strings"
)

type decimalUint struct {
	target *uint
	bits   int
}

func (v *decimalUint) String() string {
	if v.target == nil {
		return "0"
	}
	return strconv.FormatUint(uint64(*v.target), 10)
}
func (v *decimalUint) Set(text string) error {
	n, err := strconv.ParseUint(strings.TrimPrefix(text, "+"), 10, v.bits)
	if err != nil {
		return err
	}
	*v.target = uint(n)
	return nil
}
func (v *decimalUint) Get() any { return *v.target }

// UintVar uses Rust's decimal unsigned grammar and width. flag.UintVar accepts
// octal/hexadecimal prefixes, which can silently change resource/port budgets.
func UintVar(fs *flag.FlagSet, target *uint, name string, value uint, bits int, usage string) {
	*target = value
	fs.Var(&decimalUint{target: target, bits: bits}, name, usage)
}
