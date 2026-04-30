package gitutil

import (
	"fmt"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// CloneOrOpen clones a repository or opens if it exists
func CloneOrOpen(workdir, url, token string) (*git.Repository, error) {
	if _, err := os.Stat(workdir); os.IsNotExist(err) {
		return cloneRepo(workdir, url, token)
	}

	repo, err := git.PlainOpen(workdir)
	if err != nil {
		os.RemoveAll(workdir)
		return cloneRepo(workdir, url, token)
	}

	return repo, nil
}

// cloneRepo clones a repository
func cloneRepo(workdir, url, token string) (*git.Repository, error) {
	opts := &git.CloneOptions{
		URL: url,
	}
	if token != "" {
		opts.Auth = &http.BasicAuth{
			Username: "git",
			Password: token,
		}
	}

	return git.PlainClone(workdir, false, opts)
}

// CherryPick performs a cherry-pick of a commit onto the current branch.
// Strategy: Hard reset worktree to source commit, stage changes, then commit on top of HEAD.
func CherryPick(repo *git.Repository, commitSHA string) error {
	commitHash := plumbing.NewHash(commitSHA)

	// Get the source commit
	sourceCommit, err := repo.CommitObject(commitHash)
	if err != nil {
		return fmt.Errorf("failed to get commit %s: %w", commitSHA, err)
	}

	// Get the worktree
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Reset the worktree to the source commit (hard reset brings source files into workdir)
	err = w.Reset(&git.ResetOptions{
		Commit: commitHash,
		Mode:   git.HardReset,
	})
	if err != nil {
		return fmt.Errorf("failed to reset worktree to cherry-pick commit: %w", err)
	}

	// Stage all changes
	_, err = w.Add(".")
	if err != nil {
		return fmt.Errorf("failed to stage cherry-pick changes: %w", err)
	}

	// Create a new commit with the cherry-picked changes
	commitMsg := fmt.Sprintf("cherry-pick: %s\n\n(original commit: %s)", sourceCommit.Message, commitSHA)
	_, err = w.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Asika Bot",
			Email: "bot@asika",
			When:  time.Now(),
		},
		AllowEmptyCommits: true,
	})
	if err != nil {
		return fmt.Errorf("failed to commit cherry-pick: %w", err)
	}

	return nil
}

// Push pushes changes to a remote with a specific branch refspec
func Push(repo *git.Repository, remoteName, branch, token string) error {
	opts := &git.PushOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch)),
		},
	}
	if token != "" {
		opts.Auth = &http.BasicAuth{
			Username: "git",
			Password: token,
		}
	}

	if remoteName == "" {
		remoteName = "origin"
	}

	remote, err := repo.Remote(remoteName)
	if err != nil {
		return fmt.Errorf("failed to get remote %s: %w", remoteName, err)
	}

	return remote.Push(opts)
}

// FetchRemote fetches from a remote
func FetchRemote(repo *git.Repository, remoteName, token string) error {
	opts := &git.FetchOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec("+refs/heads/*:refs/remotes/" + remoteName + "/*"),
		},
	}
	if token != "" {
		opts.Auth = &http.BasicAuth{
			Username: "git",
			Password: token,
		}
	}

	if remoteName == "" {
		remoteName = "origin"
	}

	return repo.Fetch(opts)
}

// CheckoutBranch checks out a branch in the worktree
func CheckoutBranch(repo *git.Repository, branch string) error {
	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	return w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/" + branch),
	})
}

// CreateBranch creates and checks out a new branch
func CreateBranch(repo *git.Repository, branch string) error {
	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	return w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName("refs/heads/" + branch),
		Create: true,
	})
}

// CreateTempWorkdir creates a temporary working directory
func CreateTempWorkdir(prefix string) (string, error) {
	return os.MkdirTemp("", prefix)
}

// CleanupWorkdir removes a working directory
func CleanupWorkdir(workdir string) error {
	return os.RemoveAll(workdir)
}

// AddRemote adds a remote to the repository
func AddRemote(repo *git.Repository, name, url string) error {
	_, err := repo.CreateRemote(&config.RemoteConfig{
		Name: name,
		URLs: []string{url},
	})
	return err
}

// GetCommit gets a commit by SHA
func GetCommit(repo *git.Repository, sha string) (*object.Commit, error) {
	hash := plumbing.NewHash(sha)
	return repo.CommitObject(hash)
}

// CommitChanges commits staged changes in the worktree
func CommitChanges(repo *git.Repository, message string) error {
	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Asika Bot",
			Email: "bot@asika",
			When:  time.Now(),
		},
	})
	return err
}