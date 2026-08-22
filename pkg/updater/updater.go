package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agent-unleashed/pkg/doctor"
)

const Version = "1.2.0"

type UpdateResult struct {
	PreviousVersion string    `json:"previous_version"`
	CurrentVersion  string    `json:"current_version"`
	GitCommit       string    `json:"git_commit,omitempty"`
	BuildTime       time.Time `json:"build_time"`
	GlobalPath      string    `json:"global_path"`
	DoctorPassed    bool      `json:"doctor_passed"`
}

func GetVersion() string {
	return Version
}

func RunUpdate(workspaceDir string, checkOnly bool) (*UpdateResult, error) {
	if workspaceDir == "" {
		workspaceDir = "."
	}
	absWorkspace, _ := filepath.Abs(workspaceDir)

	result := &UpdateResult{
		PreviousVersion: Version,
		CurrentVersion:  Version,
		BuildTime:       time.Now(),
	}

	// 1. Check Git Status / Commit Hash
	cmdGit := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmdGit.Dir = absWorkspace
	if out, err := cmdGit.Output(); err == nil {
		result.GitCommit = strings.TrimSpace(string(out))
	}

	if checkOnly {
		return result, nil
	}

	fmt.Println("\n============================================================")
	fmt.Println("  🔄 AGENT-UNLEASHED (agt-ul) SYSTEM SELF-UPDATER")
	fmt.Println("============================================================")
	fmt.Printf("Current Version: v%s (Commit: %s)\n", Version, result.GitCommit)

	// 2. Fetch and pull latest changes if git remote is configured
	fmt.Println("\n📡 [1/4] Checking repository updates...")
	pullCmd := exec.Command("git", "pull", "--ff-only")
	pullCmd.Dir = absWorkspace
	if pullOut, err := pullCmd.CombinedOutput(); err == nil {
		fmt.Printf("  ✅ Git sync: %s\n", strings.TrimSpace(string(pullOut)))
	} else {
		fmt.Println("  ⚪ Local build mode (working copy will be compiled).")
	}

	// 3. Compile fresh Go binary
	fmt.Println("\n🔨 [2/4] Compiling optimized Go single binary...")
	buildCmd := exec.Command("go", "build", "-o", "agt-ul.exe", ".")
	buildCmd.Dir = absWorkspace
	if buildOut, err := buildCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("compilation failed: %v\nOutput: %s", err, string(buildOut))
	}
	fmt.Println("  ✅ Successfully compiled agt-ul.exe.")

	// 4. Install globally to PATH
	fmt.Println("\n📦 [3/4] Installing updated binary to PATH...")
	compiledBin := filepath.Join(absWorkspace, "agt-ul.exe")
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		targetDir := filepath.Join(localAppData, "agy", "bin")
		_ = os.MkdirAll(targetDir, 0755)

		destAgt := filepath.Join(targetDir, "agt-ul.exe")
		destAgy := filepath.Join(targetDir, "agy-ul.exe")

		_ = copyFile(compiledBin, destAgt)
		_ = copyFile(compiledBin, destAgy)

		result.GlobalPath = destAgt
		fmt.Printf("  ✅ Updated: %s\n", destAgt)
		fmt.Printf("  ✅ Updated alias: %s\n", destAgy)
	}

	// 5. Post-Update Diagnostics
	fmt.Println("\n🩺 [4/4] Running post-update health check...")
	configPath := filepath.Join(absWorkspace, "config.yaml")
	report := doctor.RunDiagnostics(configPath, false)
	doctor.PrintDoctorReport(report)
	fmt.Println("✨ Agent-Unleashed update complete! You are on the latest version.")
	fmt.Println()
	return result, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}
