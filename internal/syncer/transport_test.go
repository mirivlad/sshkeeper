package syncer

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFolderMergesConflictCopiesAndCleansThemUp(t *testing.T) {
	dir := t.TempDir()
	key, _ := NewKey()
	first, _ := Seal(key, nil)
	second, _ := Seal(key, nil)
	if err := os.WriteFile(filepath.Join(dir, BundleFile), first, 0o600); err != nil {
		t.Fatal(err)
	}
	conflict := "sshkeeper.sync-conflict-20260930-101500-ABCDEF.sync"
	if err := os.WriteFile(filepath.Join(dir, conflict), second, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sshkeeper (notes).sync"), []byte("not a bundle"), 0o600); err != nil {
		t.Fatal(err)
	}

	folder := &Folder{Dir: dir}
	bundles, err := folder.Fetch()
	if err != nil || len(bundles) != 2 {
		t.Fatalf("fetch = %d bundles, %v", len(bundles), err)
	}
	if err := folder.Store(first); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, conflict)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a merged conflict copy should be removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "sshkeeper (notes).sync")); err != nil {
		t.Fatal("unrelated files must stay")
	}
	info, _ := os.Stat(filepath.Join(dir, BundleFile))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("bundle mode = %v", info.Mode().Perm())
	}
}

func TestFolderPairingLifecycle(t *testing.T) {
	folder := &Folder{Dir: filepath.Join(t.TempDir(), "new")}
	if blob, err := folder.ReadPairing(); err != nil || blob != nil {
		t.Fatalf("no pairing expected: %v", err)
	}
	if err := folder.WritePairing([]byte("blob")); err != nil {
		t.Fatal(err)
	}
	if blob, _ := folder.ReadPairing(); string(blob) != "blob" {
		t.Fatal("pairing not readable")
	}
	if err := folder.DeletePairing(); err != nil {
		t.Fatal(err)
	}
	if err := folder.DeletePairing(); err != nil {
		t.Fatal("deleting a missing pairing is fine")
	}
}

func requireGit(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	remote := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "--quiet", remote).CombinedOutput(); err != nil {
		t.Fatalf("init remote: %v %s", err, out)
	}
	return remote
}

func TestGitStoresBundleOnItsOwnBranchAndDetectsRaces(t *testing.T) {
	remote := requireGit(t)
	a := &Git{URL: remote, Cache: filepath.Join(t.TempDir(), "a")}
	b := &Git{URL: remote, Cache: filepath.Join(t.TempDir(), "b")}

	if bundles, err := a.Fetch(); err != nil || len(bundles) != 0 {
		t.Fatalf("empty remote fetch = %d, %v", len(bundles), err)
	}
	if bundles, err := b.Fetch(); err != nil || len(bundles) != 0 {
		t.Fatalf("empty remote fetch = %d, %v", len(bundles), err)
	}
	if err := a.Store([]byte("from a")); err != nil {
		t.Fatal(err)
	}
	// b fetched before a stored: its push must be refused, not overwrite.
	if err := b.Store([]byte("from b")); !errors.Is(err, ErrStorageChanged) {
		t.Fatalf("racing store error = %v", err)
	}
	bundles, err := b.Fetch()
	if err != nil || len(bundles) != 1 || !bytes.Equal(bundles[0], []byte("from a")) {
		t.Fatalf("refetch = %q, %v", bundles, err)
	}
	if err := b.Store([]byte("from b")); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", remote, "log", "--format=%s", DefaultGitBranch).Output()
	if err != nil || bytes.Count(out, []byte("sshkeeper sync")) != 2 {
		t.Fatalf("history = %q, %v", out, err)
	}
}

func TestGitPairingBranchIsRemovedAfterUse(t *testing.T) {
	remote := requireGit(t)
	a := &Git{URL: remote, Cache: filepath.Join(t.TempDir(), "a")}
	b := &Git{URL: remote, Cache: filepath.Join(t.TempDir(), "b")}
	if err := a.WritePairing([]byte("pair")); err != nil {
		t.Fatal(err)
	}
	blob, err := b.ReadPairing()
	if err != nil || string(blob) != "pair" {
		t.Fatalf("read pairing = %q, %v", blob, err)
	}
	if err := b.DeletePairing(); err != nil {
		t.Fatal(err)
	}
	out, _ := exec.Command("git", "-C", remote, "branch", "--list").Output()
	if bytes.Contains(out, []byte("pairing")) {
		t.Fatalf("pairing branch left behind: %s", out)
	}
}
