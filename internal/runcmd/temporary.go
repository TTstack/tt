package runcmd

import "os"

// AnonymousFile creates a private scratch file and immediately removes its
// directory entry. After this returns, an interruption leaves no named file.
// Callers retain the descriptor and must close it when finished.
func AnonymousFile(directory, pattern string) (*os.File, error) {
	file, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return nil, err
	}
	if err := os.Remove(file.Name()); err != nil {
		file.Close()
		_ = os.Remove(file.Name())
		return nil, err
	}
	return file, nil
}
