package pkg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-logr/logr"
)

const (
	mirrorDir  = "./.mirror"
	stageDir   = "./.stage"
	jobsDir    = "./Jobs"
	pollPeriod = 10 * time.Second
)

func Watcher(ctx context.Context, log logr.Logger, trigChan chan<- struct{}, errChan chan<- error) {
	repoURL := os.Getenv("REPO_URL")

	if len(repoURL) == 0 {
		fmt.Println("Empty RepoURL")
		errChan <- fmt.Errorf("Failed to read repoURL")
	}

	ticker := time.NewTicker(pollPeriod)
	defer ticker.Stop()

	if err := syncRepo(trigChan, repoURL); err != nil {
		errChan <- fmt.Errorf("initial sync failed: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := syncRepo(trigChan, repoURL); err != nil {
				errChan <- fmt.Errorf("sync failed: %w", err)
			}
		}
	}
}

func syncRepo(trigChan chan<- struct{}, repoURL string) error {
	if _, err := os.Stat(mirrorDir); os.IsNotExist(err) {
		fmt.Println("Cloning repo...")
		if err := exec.Command("git", "clone", repoURL, mirrorDir).Run(); err != nil {
			return fmt.Errorf("clone failed: %w", err)
		}
	}

	cmdPull := exec.Command("git", "pull")
	cmdPull.Dir = mirrorDir
	if err := cmdPull.Run(); err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	cmdRev := exec.Command("git", "rev-parse", "HEAD")
	cmdRev.Dir = mirrorDir
	out, err := cmdRev.Output()
	if err != nil {
		return fmt.Errorf("rev-parse failed: %w", err)
	}
	newCommit := strings.TrimSpace(string(out))

	currentCommit := getCurrentCommit()

	if newCommit != currentCommit || !dirExists(jobsDir) {
		fmt.Printf("Change detected (Old: %s, New: %s). Swapping directories...\n", currentCommit, newCommit)

		_ = os.RemoveAll(stageDir)

		if err := exec.Command("cp", "-r", mirrorDir, stageDir).Run(); err != nil {
			return fmt.Errorf("copy to stage failed: %w", err)
		}

		if dirExists(jobsDir) {
			if err := os.RemoveAll(jobsDir); err != nil {
				return fmt.Errorf("failed to delete old Jobs dir: %w", err)
			}
		}

		if err := os.Rename(stageDir, jobsDir); err != nil {
			return fmt.Errorf("rename failed: %w", err)
		}

		saveCurrentCommit(newCommit)

		select {
		case trigChan <- struct{}{}:
		default:
		}
	}
	return nil
}

func dirExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func getCurrentCommit() string {
	data, _ := os.ReadFile(".commit")
	return string(data)
}

func saveCurrentCommit(hash string) {
	_ = os.WriteFile(".commit", []byte(hash), 0644)
}
