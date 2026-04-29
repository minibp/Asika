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
        // If open fails, try to clone fresh
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

// CherryPick performs a cherry-pick of a commit
func CherryPick(repo *git.Repository, commitSHA string) error {
    commitHash := plumbing.NewHash(commitSHA)

    // Create a worktree
    w, err := repo.Worktree()
    if err != nil {
        return fmt.Errorf("failed to get worktree: %w", err)
    }

    // Cherry-pick by applying the changes
    return w.Reset(&git.ResetOptions{
        Commit: commitHash,
        Mode:   git.MixedReset,
    })
}

// Push pushes changes to remote
func Push(repo *git.Repository, remoteName, branch, token string) error {
    opts := &git.PushOptions{}
    if token != "" {
        opts.Auth = &http.BasicAuth{
            Username: "git",
            Password: token,
        }
    }

    if remoteName == "" {
        remoteName = "origin"
    }

    return repo.Push(opts)
}

// FetchRemote fetches from a remote
func FetchRemote(repo *git.Repository, remoteName, token string) error {
    opts := &git.FetchOptions{}
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

// CommitChanges commits changes in the worktree
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
