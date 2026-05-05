package handlers

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "log/slog"
    "net/http"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/go-github/v69/github"
    "golang.org/x/oauth2"

    "asika/common/config"
    "asika/common/version"
)

var httpUpdateClient = &http.Client{Timeout: 60 * time.Second}

const githubOwner = "AsikaProject"
const githubRepo = "asika"

var updateProgressMap = make(map[string]chan UpdateProgress)

type UpdateProgress struct {
	Status    string `json:"status"`    // "downloading", "verifying", "installing", "done", "error"
	Progress  int    `json:"progress"`  // 0-100
	Message   string `json:"message"`
	Error     string `json:"error,omitempty"`
}

// CheckForUpdate checks GitHub for a newer version.
func CheckForUpdate(c *gin.Context) {
	cfg := config.Current()
	var httpClient *http.Client
	if cfg != nil && cfg.Tokens.GitHub != "" {
		// Use authenticated client for higher rate limit
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.Tokens.GitHub})
		httpClient = oauth2.NewClient(context.Background(), ts)
	} else {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	client := github.NewClient(httpClient)

	release, _, err := client.Repositories.GetLatestRelease(context.Background(), githubOwner, githubRepo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to check releases: %v", err)})
		return
	}

	latestVersion := strings.TrimPrefix(release.GetTagName(), "v")

	c.JSON(http.StatusOK, gin.H{
		"current":  version.Version,
		"latest":   latestVersion,
		"upgradable": version.Version != "dev" && latestVersion != version.Version,
		"url":      release.GetHTMLURL(),
		"published_at": release.GetPublishedAt(),
	})
}

// PerformWebUpdate performs the update via SSE progress stream.
func PerformWebUpdate(c *gin.Context) {
	cfg := config.Current()
	if cfg == nil || !cfg.Server.EnableWebUpdate {
		c.JSON(http.StatusForbidden, gin.H{"error": "web update is disabled"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	sendEvent := func(event string, data string) {
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	// Create GitHub client with optional authentication
	var httpClient *http.Client
	if cfg.Tokens.GitHub != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.Tokens.GitHub})
		httpClient = oauth2.NewClient(context.Background(), ts)
	} else {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	client := github.NewClient(httpClient)

	// Get latest release
	release, _, err := client.Repositories.GetLatestRelease(context.Background(), githubOwner, githubRepo)
	if err != nil {
		sendEvent("error", fmt.Sprintf(`{"error":"failed to fetch release: %s"}`, err.Error()))
		return
	}

	binaryName := "asikad"
	assetName := fmt.Sprintf("%s_%s_%s", binaryName, runtime.GOOS, runtime.GOARCH)

	downloadURL := ""
	checksumURL := ""
	for _, asset := range release.Assets {
		if asset.GetName() == assetName {
			downloadURL = asset.GetBrowserDownloadURL()
		}
		if asset.GetName() == "checksums.txt" {
			checksumURL = asset.GetBrowserDownloadURL()
		}
	}
	if downloadURL == "" {
		sendEvent("error", fmt.Sprintf(`{"error":"no asset found for %s"}`, assetName))
		return
	}
	if !isValidGitHubDownloadURL(downloadURL) {
		sendEvent("error", fmt.Sprintf(`{"error":"invalid download URL: %s"}`, downloadURL))
		return
	}

	sendEvent("progress", `{"status":"downloading","progress":0,"message":"Starting download..."}`)

	tmpDir, err := os.MkdirTemp("", "asika_update")
	if err != nil {
		sendEvent("error", fmt.Sprintf(`{"error":"failed to create temp dir: %s"}`, err.Error()))
		return
	}
	defer os.RemoveAll(tmpDir)

	binaryPath := filepath.Join(tmpDir, assetName)
	if err := downloadWithProgress(downloadURL, binaryPath, sendEvent); err != nil {
		sendEvent("error", fmt.Sprintf(`{"error":"download failed: %s"}`, err.Error()))
		return
	}

	if checksumURL == "" {
		sendEvent("error", `{"error":"checksums.txt asset not found, cannot verify download integrity"}`)
		return
	}

	sendEvent("progress", `{"status":"verifying","progress":100,"message":"Verifying checksum..."}`)

	checksumPath := filepath.Join(tmpDir, "checksums.txt")
	resp, err := httpUpdateClient.Get(checksumURL)
	if err != nil {
		sendEvent("error", fmt.Sprintf(`{"error":"failed to download checksums: %s"}`, err.Error()))
		return
	}
	f, err := os.Create(checksumPath)
	if err != nil {
		sendEvent("error", fmt.Sprintf(`{"error":"%s"}`, err.Error()))
		resp.Body.Close()
		return
	}
	io.Copy(f, resp.Body)
	f.Close()
	resp.Body.Close()

	if err := verifyWebChecksum(binaryPath, checksumPath, assetName); err != nil {
		sendEvent("error", fmt.Sprintf(`{"error":"checksum verification failed: %s"}`, err.Error()))
		return
	}

	sendEvent("progress", `{"status":"installing","progress":100,"message":"Installing update..."}`)

	currentPath, err := os.Executable()
	if err != nil {
		sendEvent("error", fmt.Sprintf(`{"error":"%s"}`, err.Error()))
		return
	}
	currentPath, err = filepath.EvalSymlinks(currentPath)
	if err != nil {
		sendEvent("error", fmt.Sprintf(`{"error":"%s"}`, err.Error()))
		return
	}

	backupPath := currentPath + ".old"
	if err := os.Rename(currentPath, backupPath); err != nil {
		sendEvent("error", fmt.Sprintf(`{"error":"backup failed: %s"}`, err.Error()))
		return
	}

in, err := os.Open(binaryPath)
    if err != nil {
        sendEvent("error", fmt.Sprintf(`{"error":"failed to open downloaded binary: %s"}`, err.Error()))
        return
    }
    out, err := os.Create(currentPath)
    if err != nil {
        in.Close()
        // Restore backup
        os.Rename(backupPath, currentPath)
        sendEvent("error", fmt.Sprintf(`{"error":"failed to create target binary: %s"}`, err.Error()))
        return
    }
    io.Copy(out, in)
    in.Close()
    out.Close()
    os.Chmod(currentPath, 0755)

	slog.Info("self-update", "version", release.GetTagName(), "from", "webui")
	sendEvent("done", `{"status":"done","progress":100,"message":"Update complete. Service restarting..."}`)

	go func() {
		os.Exit(0)
	}()
}

func downloadWithProgress(url, dest string, sendEvent func(string, string)) error {
    resp, err := httpUpdateClient.Get(url)
	if err != nil {
		return err
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

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			f.Write(buf[:n])
			downloaded += int64(n)
			if total > 0 {
				pct := int(downloaded * 100 / total)
				sendEvent("progress", fmt.Sprintf(`{"status":"downloading","progress":%d,"message":"Downloading... %d%%"}`, pct, pct))
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	return nil
}

func verifyWebChecksum(binaryPath, checksumPath, assetName string) error {
	checksums, err := os.ReadFile(checksumPath)
	if err != nil {
		return err
	}

	expected := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, assetName) {
			expected = parseChecksumLine(line)
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
		return fmt.Errorf("checksum mismatch")
	}
	return nil
}

func isValidGitHubDownloadURL(url string) bool {
	return strings.HasPrefix(url, "https://github.com/") || strings.HasPrefix(url, "https://objects.githubusercontent.com/")
}

func parseChecksumLine(line string) string {
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
