//go:build !windows
package main

import (
	"os/exec"
)
func hideCmdWindow(cmd *exec.Cmd) {
	// Mac 和 Linux 下不需要，留空即可
}