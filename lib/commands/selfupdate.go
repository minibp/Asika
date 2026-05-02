package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/google/go-github/v69/github"
	"github.com/spf13/cobra"

	"asika/common/version"
)

const (
	githubOwner = "AsikaProject"
	githubRepo  = "asika"
)

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Update asika/asikad to the latest version",
	Long: `Download and install the latest version of asika or asikad.

The command detects whether you are running asika (CLI) or asikad (daemon),
downloads the matching binary from GitHub Releases, verifies its SHA256
checksum, replaces the current binary, and exits. The process manager
(systemd, launchd, etc.) will restart the service automatically.

A backup of the old binary is saved as {binary}.old for easy rollback.`,
	Run: runSelfUpdate,
}

func init() {
	selfUpdateCmd.Flags().String("version", "", "Install a specific version tag")
	selfUpdateCmd.Flags().Bool("check", false, "Only check for new versions, do not download")
	selfUpdateCmd.Flags().Bool("dry-run", false, "Print what would be done without making changes")
	selfUpdateCmd.Flags().Bool("yes", false, "Skip confirmation prompt")
	selfUpdateCmd.Flags().Bool("rollback", false, "Rollback to the previous version (.old)")
	selfUpdateCmd.Flags().Bool("restart", false, "Restart in-place after update (standalone mode)")
	RootCmd.AddCommand(selfUpdateCmd)
}

func runSelfUpdate(cmd *cobra.Command, args []string) {
	rollback, _ := cmd.Flags().GetBool("rollback")
	if rollback {
		doRollback()
		return
	}

	checkOnly, _ := cmd.Flags().GetBool("check")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	skipConfirm, _ := cmd.Flags().GetBool("yes")
	specifiedVersion, _ := cmd.Flags().GetString("version")
	restart, _ := cmd.Flags().GetBool("restart")

	currentVersion := version.Version
	binaryName := filepath.Base(detectBinary())

	client := github.NewClient(nil)

	release, err := fetchRelease(client, specifiedVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching release: %v\n", err)
		os.Exit(1)
	}

	latestVersion := strings.TrimPrefix(release.GetTagName(), "v")

	fmt.Printf("Current version: %s\n", currentVersion)
	fmt.Printf("Latest version:   %s\n", latestVersion)

	if currentVersion != "dev" && !isNewer(latestVersion, currentVersion) {
		fmt.Println("Already up to date.")
		if checkOnly {
			return
		}
		os.Exit(0)
	}

	if currentVersion == "dev" {
		fmt.Println("(running development build, proceeding with update)")
	}

	if checkOnly {
		fmt.Println("A new version is available!")
		return
	}

	fmt.Printf("Release date: %s\n", release.GetPublishedAt().Format("2006-01-02"))
	fmt.Printf("URL: %s\n", release.GetHTMLURL())

	if !skipConfirm {
		fmt.Print("\nProceed with update? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Update cancelled.")
			return
		}
	}

	assetName := fmt.Sprintf("%s_%s_%s", binaryName, runtime.GOOS, runtime.GOARCH)
	downloadURL, checksumAsset := findAssets(release, assetName)
	if downloadURL == "" {
		fmt.Fprintf(os.Stderr, "Error: no binary asset found for %s\n", assetName)
		os.Exit(1)
	}

	if dryRun {
		fmt.Printf("\nWould download: %s\n", assetName)
		fmt.Printf("From: %s\n", downloadURL)
		fmt.Println("Dry run complete. No changes made.")
		return
	}

	tmpDir, err := os.MkdirTemp("", "asika_update")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath := filepath.Join(tmpDir, assetName)
	fmt.Printf("\nDownloading %s...\n", assetName)
	if err := downloadFile(downloadURL, binaryPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error downloading binary: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Download complete.")

	if checksumAsset != "" {
		fmt.Println("Verifying checksum...")
		checksumPath := filepath.Join(tmpDir, "checksums.txt")
		if err := downloadFile(checksumAsset, checksumPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error downloading checksums: %v\n", err)
			os.Exit(1)
		}
		if err := verifyChecksum(binaryPath, checksumPath, assetName); err != nil {
			fmt.Fprintf(os.Stderr, "Checksum verification failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Checksum OK.")
	}

	currentPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding current binary: %v\n", err)
		os.Exit(1)
	}
	currentPath, err = filepath.EvalSymlinks(currentPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving binary path: %v\n", err)
		os.Exit(1)
	}

	backupPath := currentPath + ".old"

	fmt.Println("Backing up current binary to", backupPath)
	if err := os.Rename(currentPath, backupPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error backing up binary: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Installing new binary to", currentPath)
	if err := copyFile(binaryPath, currentPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error installing binary: %v\n", err)
		fmt.Fprintf(os.Stderr, "Attempting to restore backup...\n")
		os.Rename(backupPath, currentPath)
		os.Exit(1)
	}

	if err := os.Chmod(currentPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not set executable permission: %v\n", err)
	}

	fmt.Println("Update complete!")
	fmt.Println("To rollback, run: asika self-update --rollback")

	if restart {
		fmt.Println("Restarting...")
		execSelf(currentPath)
	} else {
		fmt.Println("Exiting for process manager to restart...")
		os.Exit(0)
	}
}

func detectBinary() string {
	path, err := os.Executable()
	if err != nil {
		return "asika"
	}
	return path
}

func fetchRelease(client *github.Client, tag string) (*github.RepositoryRelease, error) {
	ctx := context.Background()
	if tag != "" {
		release, _, err := client.Repositories.GetReleaseByTag(ctx, githubOwner, githubRepo, tag)
		return release, err
	}
	release, _, err := client.Repositories.GetLatestRelease(ctx, githubOwner, githubRepo)
	return release, err
}

func findAssets(release *github.RepositoryRelease, binaryName string) (binaryURL, checksumURL string) {
	for _, asset := range release.Assets {
		name := asset.GetName()
		if name == binaryName {
			binaryURL = asset.GetBrowserDownloadURL()
		}
		if name == "checksums.txt" {
			checksumURL = asset.GetBrowserDownloadURL()
		}
	}
	return
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func verifyChecksum(binaryPath, checksumPath, assetName string) error {
	checksums, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("cannot read checksums: %w", err)
	}

	expected := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, assetName) {
			expected = parseChecksumLine(line, assetName)
			if expected != "" {
				break
			}
		}
	}
	if expected == "" {
		return fmt.Errorf("no checksum entry for %s", assetName)
	}

	data, err := os.ReadFile(binaryPath)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	actual := hex.EncodeToString(hash[:])
	if actual != expected {
		return fmt.Errorf("checksum mismatch\nexpected: %s\nactual:   %s", expected, actual)
	}
	return nil
}

func parseChecksumLine(line, assetName string) string {
	if strings.Contains(line, " = ") {
		idx := strings.LastIndex(line, " = ")
		if idx >= 0 {
			return strings.TrimSpace(line[idx+3:])
		}
	}
	parts := strings.Fields(line)
	if len(parts) >= 1 {
		return parts[0]
	}
	return ""
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func execSelf(path string) {
	args := os.Args
	env := os.Environ()
	if err := syscall.Exec(path, args, env); err != nil {
		fmt.Fprintf(os.Stderr, "Error restarting: %v\n", err)
		os.Exit(1)
	}
}

func doRollback() {
	currentPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding current binary: %v\n", err)
		os.Exit(1)
	}
	currentPath, err = filepath.EvalSymlinks(currentPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving binary path: %v\n", err)
		os.Exit(1)
	}

	backupPath := currentPath + ".old"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: no backup file found at %s\n", backupPath)
		os.Exit(1)
	}

	brokenPath := currentPath + ".broken"
	fmt.Println("Moving current binary to", brokenPath)
	if err := os.Rename(currentPath, brokenPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error moving current binary: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Restoring backup from", backupPath)
	if err := os.Rename(backupPath, currentPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error restoring backup: %v\n", err)
		os.Rename(brokenPath, currentPath)
		os.Exit(1)
	}

	if err := os.Chmod(currentPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not set executable permission: %v\n", err)
	}

	fmt.Println("Rollback complete. Please restart the service manually.")
}

func isNewer(latest, current string) bool {
	return latest != current
}
