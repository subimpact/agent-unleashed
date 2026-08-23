package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	Installed       bool      `json:"installed"`
	DoctorPassed    bool      `json:"doctor_passed"`
}

// installDir resolves where the binary belongs on this platform. It used to be
// hardcoded to %LOCALAPPDATA%\agy\bin, which is another product's directory and
// does not exist off Windows.
func installDir() string {
	if dir := os.Getenv("AGT_UL_INSTALL_DIR"); dir != "" {
		return dir
	}
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "agt-ul", "bin")
		}
		return ""
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "bin")
	}
	return ""
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func commitLabel(commit string) string {
	if commit == "" {
		return "the local working copy"
	}
	return "commit " + commit
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "not a git checkout"
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
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
		if out, gitErr := gitOutput(absWorkspace, "rev-parse", "--short", "HEAD"); gitErr == nil {
			result.CurrentVersion = Version
			result.GitCommit = out
		}
	} else {
		// Not a checkout, no remote, or local commits in the way. Say which,
		// instead of implying the sync was skipped on purpose.
		fmt.Printf("  ⚪ Skipping git sync (%s). Building the current working copy.\n", firstLine(string(pullOut)))
	}

	// 3. Compile fresh Go binary
	if _, err := exec.LookPath("go"); err != nil {
		return nil, fmt.Errorf("the Go toolchain is required to self-update from source but was not found on PATH; install Go from https://go.dev/dl/ or download a release binary from https://github.com/subimpact/agent-unleashed/releases")
	}

	binName := "agt-ul"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}

	fmt.Println("\n🔨 [2/4] Compiling optimized Go single binary...")
	buildCmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", binName, ".")
	buildCmd.Dir = absWorkspace
	if buildOut, err := buildCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("compilation failed: %v\nOutput: %s", err, string(buildOut))
	}
	fmt.Printf("  ✅ Successfully compiled %s.\n", binName)

	// 4. Install globally to PATH
	fmt.Println("\n📦 [3/4] Installing updated binary to PATH...")
	compiledBin := filepath.Join(absWorkspace, binName)
	targetDir := installDir()

	if targetDir == "" {
		fmt.Println("  ⚪ No install directory resolved; the fresh binary is at " + compiledBin)
	} else if err := os.MkdirAll(targetDir, 0o755); err != nil {
		fmt.Printf("  ❌ Could not create %s: %v\n", targetDir, err)
	} else {
		aliasName := "agy-ul"
		if runtime.GOOS == "windows" {
			aliasName += ".exe"
		}
		destAgt := filepath.Join(targetDir, binName)
		destAgy := filepath.Join(targetDir, aliasName)

		// These errors used to be discarded and the next line printed a tick
		// regardless. On Windows the running executable cannot be replaced, so
		// the usual outcome was a silent no-op reported as a successful update.
		if err := copyFile(compiledBin, destAgt); err != nil {
			fmt.Printf("  ❌ Could not install %s: %v\n", destAgt, err)
			if runtime.GOOS == "windows" {
				fmt.Println("     A running agt-ul.exe cannot be overwritten. Stop it and run 'agt-ul update' again.")
			}
			fmt.Println("     The freshly built binary is at " + compiledBin)
		} else {
			result.GlobalPath = destAgt
			result.Installed = true
			fmt.Printf("  ✅ Updated: %s\n", destAgt)

			if err := copyFile(compiledBin, destAgy); err != nil {
				fmt.Printf("  ⚠ Could not update alias %s: %v\n", destAgy, err)
			} else {
				fmt.Printf("  ✅ Updated alias: %s\n", destAgy)
			}
		}
	}

	// 5. Post-Update Diagnostics
	fmt.Println("\n🩺 [4/4] Running post-update health check...")
	configPath := filepath.Join(absWorkspace, "config.yaml")
	report := doctor.RunDiagnostics(configPath, false)
	doctor.PrintDoctorReport(report)
	result.DoctorPassed = report.Failures == 0

	if result.Installed {
		fmt.Printf("✨ Agent-Unleashed rebuilt from %s and installed to %s.\n", commitLabel(result.GitCommit), result.GlobalPath)
	} else {
		fmt.Println("⚠ Agent-Unleashed was rebuilt but NOT installed to PATH - see the errors above.")
	}
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
