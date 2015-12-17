package util

import (
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

/*
   references:
   - https://no-color.org/
   - https://github.com/sitkevij/no_color
*/

// IsTerminal checks if stdout is a terminal using go-isatty
func IsTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}

// ShouldUseColors determines if coloured output should be used
