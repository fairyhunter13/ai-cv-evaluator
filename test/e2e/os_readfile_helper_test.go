//go:build e2e

package e2e_test

import (
	"os"
)

func osReadFileImpl(p string) ([]byte, error) { return os.ReadFile(p) }
