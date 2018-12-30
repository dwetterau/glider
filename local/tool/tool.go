package tool

import "os/exec"

func Run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	stdout, err := cmd.CombinedOutput()
	return string(stdout), err
}
