package cli

import (
	"fmt"
	"os"
)

// readBodyFile reads the entire contents of path as a string, used
// to support -body-file as an alternative to passing a large JSON
// payload inline via -body on the command line.
func readBodyFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cli: failed to read body file %q: %w", path, err)
	}
	return string(data), nil
}
