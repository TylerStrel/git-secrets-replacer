package replacer

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestProcessTree(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitrepo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := exec.Command("git", "init", tmpDir).Run(); err != nil {
		t.Fatalf("git init error: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir error: %v", err)
	}

	filePath := "testfile.txt"
	content := []byte("this is a secret\n")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("write file error: %v", err)
	}
	if err := exec.Command("git", "add", filePath).Run(); err != nil {
		t.Fatalf("git add error: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "initial commit").Run(); err != nil {
		t.Fatalf("git commit error: %v", err)
	}

	commitHash, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse error: %v", err)
	}
	commitHashStr := strings.TrimSpace(string(commitHash))

	treeHash, err := GetTree(commitHashStr)
	if err != nil {
		t.Fatalf("GetTree() error = %v", err)
	}

	secrets := []string{"secret"}

	newTree, err := ProcessTree(treeHash, secrets)
	if err != nil {
		t.Fatalf("ProcessTree() error = %v", err)
	}

	if newTree == treeHash {
		t.Fatalf("expected new tree to be different from the original tree")
	}

	newBlobHash, err := exec.Command("git", "cat-file", "-p", newTree).Output()
	if err != nil {
		t.Fatalf("git cat-file error: %v", err)
	}

	newBlobContent, err := exec.Command("git", "cat-file", "-p", strings.Fields(string(newBlobHash))[2]).Output()
	if err != nil {
		t.Fatalf("git cat-file error: %v", err)
	}

	expectedContent := "this is a **REMOVED**\n"
	if string(newBlobContent) != expectedContent {
		t.Errorf("expected blob content %q, got %q", expectedContent, string(newBlobContent))
	}
}

func TestProcessCommitOrder(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitrepo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := exec.Command("git", "init", tmpDir).Run(); err != nil {
		t.Fatalf("git init error: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir error: %v", err)
	}

	filePath := "testfile.txt"
	content := []byte("this is a secret\n")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("write file error: %v", err)
	}
	if err := exec.Command("git", "add", filePath).Run(); err != nil {
		t.Fatalf("git add error: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "initial commit").Run(); err != nil {
		t.Fatalf("git commit error: %v", err)
	}

	content = []byte("this is another secret\n")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("write file error: %v", err)
	}
	if err := exec.Command("git", "add", filePath).Run(); err != nil {
		t.Fatalf("git add error: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "second commit").Run(); err != nil {
		t.Fatalf("git commit error: %v", err)
	}

	commitHashes, err := exec.Command("git", "rev-list", "--all").Output()
	if err != nil {
		t.Fatalf("git rev-list error: %v", err)
	}
	commitHashList := strings.Split(strings.TrimSpace(string(commitHashes)), "\n")

	secrets := []string{"secret"}

	var newCommitHashes []string
	for _, commitHash := range commitHashList {
		newCommitHash, err := ProcessCommit(commitHash, secrets)
		if err != nil {
			t.Fatalf("ProcessCommit() error = %v", err)
		}
		newCommitHashes = append(newCommitHashes, newCommitHash)
	}

	if len(newCommitHashes) != len(commitHashList) {
		t.Fatalf("expected %d commits, got %d", len(commitHashList), len(newCommitHashes))
	}

	for i, newCommitHash := range newCommitHashes {
		if newCommitHash == commitHashList[i] {
			t.Fatalf("expected new commit hash to be different from the original commit hash")
		}
	}
}

func TestProcessBlobWithGoroutines(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitrepo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := exec.Command("git", "init", tmpDir).Run(); err != nil {
		t.Fatalf("git init error: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir error: %v", err)
	}

	filePath := "testfile.txt"
	content := []byte("this is a secret\n")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("write file error: %v", err)
	}
	if err := exec.Command("git", "add", filePath).Run(); err != nil {
		t.Fatalf("git add error: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "initial commit").Run(); err != nil {
		t.Fatalf("git commit error: %v", err)
	}

	commitHash, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse error: %v", err)
	}
	commitHashStr := strings.TrimSpace(string(commitHash))

	treeHash, err := GetTree(commitHashStr)
	if err != nil {
		t.Fatalf("GetTree() error = %v", err)
	}

	secrets := []string{"secret"}

	newTree, err := ProcessTree(treeHash, secrets)
	if err != nil {
		t.Fatalf("ProcessTree() error = %v", err)
	}

	if newTree == treeHash {
		t.Fatalf("expected new tree to be different from the original tree")
	}

	newBlobHash, err := exec.Command("git", "cat-file", "-p", newTree).Output()
	if err != nil {
		t.Fatalf("git cat-file error: %v", err)
	}

	newBlobContent, err := exec.Command("git", "cat-file", "-p", strings.Fields(string(newBlobHash))[2]).Output()
	if err != nil {
		t.Fatalf("git cat-file error: %v", err)
	}

	expectedContent := "this is a **REMOVED**\n"
	if string(newBlobContent) != expectedContent {
		t.Errorf("expected blob content %q, got %q", expectedContent, string(newBlobContent))
	}
}

func TestIsBinary(t *testing.T) {
	textContent := []byte("this is a text file")
	binaryContent := []byte{0x00, 0x01, 0x02, 0x03}

	if IsBinary(textContent) {
		t.Errorf("expected text content to be non-binary")
	}

	if !IsBinary(binaryContent) {
		t.Errorf("expected binary content to be binary")
	}
}

func TestWriteBlob(t *testing.T) {
	// Create a temporary directory for the Git repository
	tmpDir, err := os.MkdirTemp("", "gitrepo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a new Git repository
	if err := exec.Command("git", "init", tmpDir).Run(); err != nil {
		t.Fatalf("git init error: %v", err)
	}

	// Change to the temporary directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir error: %v", err)
	}

	content := []byte("this is a test blob")

	sha, err := WriteBlob(content)
	if err != nil {
		t.Fatalf("WriteBlob() error = %v", err)
	}

	output, err := exec.Command("git", "cat-file", "-p", sha).Output()
	if err != nil {
		t.Fatalf("git cat-file error: %v, output: %s", err, output)
	}

	if string(output) != string(content) {
		t.Errorf("expected blob content %q, got %q", string(content), string(output))
	}
}
