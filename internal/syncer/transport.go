package syncer

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BundleFile is the name of the sealed bundle in the sync storage.
const BundleFile = "sshkeeper.sync"

// PairingFile is the name of the short-lived pairing blob.
const PairingFile = "sshkeeper.pairing"

// ErrStorageChanged means another device wrote the storage during this sync;
// the engine fetches and merges again.
var ErrStorageChanged = errors.New("sync storage changed during sync")

// Transport moves opaque bytes. It never sees plaintext.
type Transport interface {
	// Fetch returns every bundle found. Folder sync tools can leave conflict
	// copies next to the bundle; all of them are merged.
	Fetch() ([][]byte, error)
	// Store replaces the bundle. It returns ErrStorageChanged when the
	// storage moved on since Fetch.
	Store(bundle []byte) error
	// ReadPairing returns the pending pairing blob, or nil when none exists.
	ReadPairing() ([]byte, error)
	WritePairing(blob []byte) error
	DeletePairing() error
}

// Folder stores the bundle in a directory kept in step by Syncthing,
// Nextcloud, Dropbox, a network share, or similar.
type Folder struct {
	Dir string

	conflicts []string
}

func (f *Folder) path(name string) string { return filepath.Join(f.Dir, name) }

func (f *Folder) Fetch() ([][]byte, error) {
	if err := os.MkdirAll(f.Dir, 0o700); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(f.Dir)
	if err != nil {
		return nil, err
	}
	f.conflicts = nil
	var bundles [][]byte
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isBundleName(name) {
			continue
		}
		data, err := os.ReadFile(f.path(name))
		if err != nil {
			return nil, err
		}
		if name != BundleFile {
			if BundleKeyID(data) == "" {
				continue
			}
			f.conflicts = append(f.conflicts, name)
		}
		bundles = append(bundles, data)
	}
	return bundles, nil
}

// isBundleName matches the bundle and the conflict copies sync tools create,
// e.g. "sshkeeper.sync-conflict-20260930-101500-ABC.sync" (Syncthing) or
// "sshkeeper (conflicted copy 2026-09-30).sync" (Dropbox, Nextcloud).
func isBundleName(name string) bool {
	return strings.HasPrefix(name, "sshkeeper") && strings.HasSuffix(name, ".sync") && !strings.HasPrefix(name, ".")
}

func (f *Folder) Store(bundle []byte) error {
	if err := writeAtomic(f.path(BundleFile), bundle); err != nil {
		return err
	}
	// The merged bundle includes every conflict copy read by Fetch.
	for _, name := range f.conflicts {
		_ = os.Remove(f.path(name))
	}
	f.conflicts = nil
	return nil
}

func (f *Folder) ReadPairing() ([]byte, error) {
	data, err := os.ReadFile(f.path(PairingFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

func (f *Folder) WritePairing(blob []byte) error {
	if err := os.MkdirAll(f.Dir, 0o700); err != nil {
		return err
	}
	return writeAtomic(f.path(PairingFile), blob)
}

func (f *Folder) DeletePairing() error {
	err := os.Remove(f.path(PairingFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func writeAtomic(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".sshkeeper-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

// Git stores the bundle as the only file of a branch in a git repository.
// It works through a private bare cache repository and git plumbing, so it
// never touches a working tree, and it relies on the user's own git
// credentials. Prompts are disabled: a missing credential fails fast instead
// of hanging the TUI.
type Git struct {
	URL    string
	Branch string
	// Cache is the local bare repository used for fetch and push.
	Cache string

	parent string
}

// DefaultGitBranch is used when no branch is configured. A dedicated branch
// keeps sync commits away from anything else in the repository.
const DefaultGitBranch = "sshkeeper"

func (g *Git) branch() string {
	if strings.TrimSpace(g.Branch) == "" {
		return DefaultGitBranch
	}
	return strings.TrimSpace(g.Branch)
}

func (g *Git) pairingBranch() string { return g.branch() + "-pairing" }

func (g *Git) git(stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", g.Cache}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"LC_ALL=C",
		"GIT_AUTHOR_NAME=sshkeeper", "GIT_AUTHOR_EMAIL=sshkeeper@localhost",
		"GIT_COMMITTER_NAME=sshkeeper", "GIT_COMMITTER_EMAIL=sshkeeper@localhost",
	)
	if os.Getenv("GIT_SSH_COMMAND") == "" {
		cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
	}
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return stdout.Bytes(), fmt.Errorf("git %s: %s", args[0], message)
	}
	return stdout.Bytes(), nil
}

func (g *Git) ensureCache() error {
	if strings.TrimSpace(g.URL) == "" {
		return fmt.Errorf("git repository URL is not set")
	}
	if _, err := os.Stat(filepath.Join(g.Cache, "HEAD")); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(g.Cache, 0o700); err != nil {
			return err
		}
		if _, err := g.git(nil, "init", "--bare", "--quiet"); err != nil {
			return err
		}
	}
	if _, err := g.git(nil, "remote", "get-url", "origin"); err != nil {
		_, err = g.git(nil, "remote", "add", "origin", g.URL)
		return err
	}
	_, err := g.git(nil, "remote", "set-url", "origin", g.URL)
	return err
}

// fetchRef fetches a remote branch and returns its commit, or "" when the
// branch does not exist yet.
func (g *Git) fetchRef(branch string) (string, error) {
	out, err := g.git(nil, "ls-remote", "origin", "refs/heads/"+branch)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(out)) == "" {
		return "", nil
	}
	local := "refs/remotes/origin/" + branch
	if _, err := g.git(nil, "fetch", "--quiet", "--no-tags", "origin", "+refs/heads/"+branch+":"+local); err != nil {
		return "", err
	}
	commit, err := g.git(nil, "rev-parse", local)
	return strings.TrimSpace(string(commit)), err
}

func (g *Git) Fetch() ([][]byte, error) {
	if err := g.ensureCache(); err != nil {
		return nil, err
	}
	commit, err := g.fetchRef(g.branch())
	if err != nil {
		return nil, err
	}
	g.parent = commit
	if commit == "" {
		return nil, nil
	}
	data, err := g.git(nil, "cat-file", "blob", commit+":"+BundleFile)
	if err != nil {
		return nil, nil
	}
	return [][]byte{data}, nil
}

// commitFile writes a one-file commit and returns its hash.
func (g *Git) commitFile(name string, data []byte, parent string) (string, error) {
	blob, err := g.git(data, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	tree, err := g.git([]byte(fmt.Sprintf("100644 blob %s\t%s\n", strings.TrimSpace(string(blob)), name)), "mktree")
	if err != nil {
		return "", err
	}
	args := []string{"commit-tree", strings.TrimSpace(string(tree)), "-m", "sshkeeper sync"}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	commit, err := g.git(nil, args...)
	return strings.TrimSpace(string(commit)), err
}

func (g *Git) Store(bundle []byte) error {
	commit, err := g.commitFile(BundleFile, bundle, g.parent)
	if err != nil {
		return err
	}
	if _, err := g.git(nil, "push", "--quiet", "origin", commit+":refs/heads/"+g.branch()); err != nil {
		text := err.Error()
		if strings.Contains(text, "rejected") || strings.Contains(text, "fetch first") || strings.Contains(text, "non-fast-forward") {
			return ErrStorageChanged
		}
		return err
	}
	g.parent = commit
	return nil
}

// Pairing uses its own branch, force-pushed and deleted after use, so the
// blob never enters the history of the bundle branch.
func (g *Git) ReadPairing() ([]byte, error) {
	if err := g.ensureCache(); err != nil {
		return nil, err
	}
	commit, err := g.fetchRef(g.pairingBranch())
	if err != nil || commit == "" {
		return nil, err
	}
	data, err := g.git(nil, "cat-file", "blob", commit+":"+PairingFile)
	if err != nil {
		return nil, nil
	}
	return data, nil
}

func (g *Git) WritePairing(blob []byte) error {
	if err := g.ensureCache(); err != nil {
		return err
	}
	commit, err := g.commitFile(PairingFile, blob, "")
	if err != nil {
		return err
	}
	_, err = g.git(nil, "push", "--quiet", "--force", "origin", commit+":refs/heads/"+g.pairingBranch())
	return err
}

func (g *Git) DeletePairing() error {
	if err := g.ensureCache(); err != nil {
		return err
	}
	out, err := g.git(nil, "ls-remote", "origin", "refs/heads/"+g.pairingBranch())
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return err
	}
	_, err = g.git(nil, "push", "--quiet", "origin", "--delete", g.pairingBranch())
	return err
}
