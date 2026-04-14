package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newUpgradeCmd creates the "hermes upgrade" command. It queries the
// GitHub Releases API for the latest tag, downloads the matching
// archive for the host OS/arch, extracts the hermes binary, and
// atomically replaces the current executable via rename.
func newUpgradeCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Download and install the latest hermes release",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(cmd.Context())
		},
	}
	return cmd
}

const githubRepo = "odysseythink/hermind"

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func runUpgrade(ctx context.Context) error {
	client := &http.Client{Timeout: 60 * time.Second}
	// Fetch latest release.
	req, _ := http.NewRequestWithContext(ctx, "GET",
		"https://api.github.com/repos/"+githubRepo+"/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upgrade: fetch release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("upgrade: github api status %d: %s", resp.StatusCode, string(body))
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return err
	}
	if rel.TagName == "" {
		return fmt.Errorf("upgrade: empty tag in release response")
	}

	// Pick the asset matching the host platform.
	assetName, arch := archiveNameFor(rel.TagName, runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	for _, a := range rel.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("upgrade: no asset for %s/%s (expected %s)", runtime.GOOS, arch, assetName)
	}

	fmt.Printf("downloading hermes %s (%s_%s)...\n", rel.TagName, runtime.GOOS, arch)
	tmpDir, err := os.MkdirTemp("", "hermes-upgrade-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	archivePath := filepath.Join(tmpDir, assetName)

	// Download the archive.
	dlReq, _ := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	dlResp, err := client.Do(dlReq)
	if err != nil {
		return fmt.Errorf("upgrade: download: %w", err)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode != 200 {
		return fmt.Errorf("upgrade: download status %d", dlResp.StatusCode)
	}
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, dlResp.Body); err != nil {
		f.Close()
		return err
	}
	f.Close()

	// Extract hermes binary from the tar.gz.
	binPath, err := extractHermes(archivePath, tmpDir)
	if err != nil {
		return fmt.Errorf("upgrade: extract: %w", err)
	}

	// Replace the current executable.
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// Rename is atomic on POSIX; on Windows we fall back to copy+delete.
	if runtime.GOOS == "windows" {
		if err := copyFile(binPath, exe+".new"); err != nil {
			return err
		}
		if err := os.Rename(exe+".new", exe); err != nil {
			return err
		}
	} else {
		// Move new binary over current path via os.Rename on the
		// same filesystem. The tmp dir is typically on /tmp so we
		// copy first.
		dest := exe
		tmp := dest + ".new"
		if err := copyFile(binPath, tmp); err != nil {
			return err
		}
		if err := os.Chmod(tmp, 0o755); err != nil {
			return err
		}
		if err := os.Rename(tmp, dest); err != nil {
			return err
		}
	}
	fmt.Printf("hermes upgraded to %s\n", rel.TagName)
	return nil
}

// archiveNameFor returns the canonical archive name for a given
// hermes release tag + host platform. Mirrors .goreleaser.yml.
func archiveNameFor(tag, goos, goarch string) (string, string) {
	verNoV := strings.TrimPrefix(tag, "v")
	archLabel := goarch
	if goarch == "amd64" {
		archLabel = "x86_64"
	}
	return fmt.Sprintf("hermes_%s_%s_%s.tar.gz", verNoV, goos, archLabel), archLabel
}

// extractHermes opens a .tar.gz and writes the "hermes" binary to
// outDir. Returns the extracted file path.
func extractHermes(archivePath, outDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if filepath.Base(hdr.Name) != "hermes" {
			continue
		}
		dst := filepath.Join(outDir, "hermes")
		out, err := os.Create(dst)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return "", err
		}
		out.Close()
		if err := os.Chmod(dst, 0o755); err != nil {
			return "", err
		}
		return dst, nil
	}
	return "", fmt.Errorf("no hermes binary in archive")
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
