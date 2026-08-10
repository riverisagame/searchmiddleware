// 启动器：DETACHED 启动 searchmiddleware（日志到文件）
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func main() {
	cmd := exec.Command("D:\\prj\\searchmiddleware\\searchmiddleware.exe")
	cmd.Dir = "D:\\prj\\searchmiddleware"

	logF, err := os.Create("D:\\prj\\searchmiddleware\\data\\sm_e2e.log")
	if err != nil {
		log.Fatal(err)
	}
	defer logF.Close()
	cmd.Stdout = logF
	cmd.Stderr = logF
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000008}

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("sm started pid=%d (detached)\n", cmd.Process.Pid)

	for i := 0; i < 15; i++ {
		time.Sleep(1 * time.Second)
		out, _ := exec.Command("curl.exe", "-s", "-o", "NUL", "-w", "%{http_code}", "http://localhost:8090/health").Output()
		if string(out) == "200" {
			fmt.Println("health 200")
			return
		}
	}
	fmt.Println("health NOT ready")
}
