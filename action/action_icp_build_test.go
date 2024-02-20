package action

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	cmd := exec.Command(DFX_BIN, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}
	str := strings.TrimSpace(string(output))
	fmt.Println("==========")
	fmt.Println(strings.TrimPrefix(str, "dfx "))
	fmt.Println("==========")
}
