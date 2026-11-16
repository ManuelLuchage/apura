// Package all registers every built-in shell integration.
// Adding a shell = one blank import here.
package all

import (
	_ "github.com/ManuelLuchage/apura/internal/shell/fish"
	// _ "github.com/ManuelLuchage/apura/internal/shell/zsh"
	// _ "github.com/ManuelLuchage/apura/internal/shell/bash"
)
