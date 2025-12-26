package open

import (
	"os/exec"
	"runtime"
	"strings"
)

func Explorer(path string) {
	// open file in explorer
	// https://askubuntu.com/questions/133597/reveal-file-in-file-explorer
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", "/select,", path)
	case "darwin":
		cmd = exec.Command("open", "-R", path)
	case "linux":
		// cSpell: words: dbus freedesktop
		cmd = exec.Command("dbus-send", "--session", "-dest=org.freedesktop.FileManager1", "--type=method_call", "/org/freedesktop/FileManager1", "org.freedesktop.FileManager1.ShowItems", `array:string:"`+strings.ReplaceAll(path, "\"", "\\\"")+`"`, `string:""`)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	cmd.Run() //nolint:errcheck

	/* ignore error, windows explorer always returns 1 as exit status
	// err := cmd.Run()
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error open path: %s", err)
		log.Printf("Program output: %s", string(out))
	}
	*/
}
