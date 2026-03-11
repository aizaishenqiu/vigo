package nacos

import (
	"fmt"
	"os/exec"
	"runtime"
)

// CheckNacos checks if Nacos is running on the specified host and port
func CheckNacos(host string, port int) bool {
	url := fmt.Sprintf("http://%s:%d/nacos/v1/console/health/liveness", host, port)

	// Simple check by trying to access Nacos health endpoint
	cmd := exec.Command("curl", "-s", "-f", url)
	err := cmd.Run()

	return err == nil
}

// StartNacos tries to start Nacos using the startup script
func StartNacos(nacosDir string) error {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", "startup.cmd", "-m", "standalone")
	} else {
		cmd = exec.Command("sh", "startup.sh", "-m", "standalone")
	}

	cmd.Dir = nacosDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start Nacos: %v, output: %s", err, string(output))
	}

	fmt.Printf("Nacos startup initiated. Output: %s\n", string(output))
	return nil
}

// IsNacosInstalled checks if Nacos directory exists and contains necessary files
func IsNacosInstalled(nacosDir string) bool {
	// Check if Nacos directory exists and contains startup script
	var startupScript string
	if runtime.GOOS == "windows" {
		startupScript = "startup.cmd"
	} else {
		startupScript = "startup.sh"
	}

	path := fmt.Sprintf("%s/bin/%s", nacosDir, startupScript)

	cmd := exec.Command("ls", path)
	err := cmd.Run()

	return err == nil
}

// DownloadAndInstallNacos downloads and installs Nacos
func DownloadAndInstallNacos(version string, installDir string) error {
	// This is a placeholder for actual download and installation logic
	// In practice, this would download the Nacos distribution and extract it
	fmt.Printf("Downloading Nacos version %s to %s...\n", version, installDir)

	// For now, we just notify the user about the manual steps
	fmt.Println("To install Nacos:")
	fmt.Println("1. Download from https://github.com/alibaba/nacos/releases")
	fmt.Println("2. Extract to your desired directory")
	fmt.Println("3. Run the startup script")

	return nil
}
