package replacer

import (
	"os"
	"os/exec"
	"runtime"
	"sync"
	"testing"
)

func mockExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	if len(args) < 4 {
		os.Exit(2)
	}
	switch args[3] {
	case "git":
		if len(args) > 4 && args[4] == "log" {
			os.Stdout.Write([]byte("tree abcdef123456\n"))
		} else if len(args) > 6 && args[4] == "cat-file" && args[5] == "-s" && args[6] == "abcdef" {
			os.Stdout.Write([]byte("12345\n"))
		} else if len(args) > 4 && args[4] == "hash-object" {
			os.Stdout.Write([]byte("newhash123\n"))
		} else if len(args) > 5 && args[4] == "cat-file" && args[5] == "-p" && args[6] == "abcdef" {
			os.Stdout.Write([]byte("secret data\n"))
		} else if len(args) > 5 && args[4] == "cat-file" && args[5] == "-p" {
			os.Stdout.Write([]byte("mocked output\n"))
		} else {
			os.Stdout.Write([]byte("mocked output\n"))
		}
		os.Exit(0)
	default:
		os.Stderr.WriteString("mocked command not recognized\n")
		os.Exit(128)
	}
}

func TestGetCachedGitOutput_CacheHit(t *testing.T) {
	commitCache = sync.Map{}
	commitCache.Store("git log", []byte("abcdef123456"))

	output, err := GetCachedGitOutput("git", "log")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(output) != "abcdef123456" {
		t.Errorf("expected 'abcdef123456', got '%s'", output)
	}
}

func TestGetCachedGitOutput_CacheMiss(t *testing.T) {
	execCommand = mockExecCommand
	defer func() { execCommand = exec.Command }() // Restore execCommand after the test

	output, err := GetCachedGitOutput("git", "log")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(output) != "abcdef123456" {
		t.Errorf("expected 'abcdef123456', got '%s'", output)
	}
}

func TestGetTree_Found(t *testing.T) {
	treeCache = sync.Map{}
	treeCache.Store("abcdef", "123456")

	tree, err := GetTree("abcdef")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tree != "123456" {
		t.Errorf("expected '123456', got '%s'", tree)
	}
}

func TestGetTree_NotFound(t *testing.T) {
	execCommand = mockExecCommand
	tree, err := GetTree("abcdef")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedTree := "123456"
	if tree != expectedTree {
		t.Errorf("expected '%s', got '%s'", expectedTree, tree)
	}
}

func TestIsBinary(t *testing.T) {
	binaryContent := []byte{0x00, 0x10, 0x20}
	nonBinaryContent := []byte("hello, world")

	if !IsBinary(binaryContent) {
		t.Error("expected binary content to be detected")
	}
	if IsBinary(nonBinaryContent) {
		t.Error("expected non-binary content not to be detected as binary")
	}
}

func TestIsMemoryUsageHigh_True(t *testing.T) {
	MemoryStatsWrapper = func(memStats *runtime.MemStats) {
		memStats.Alloc = 1
		memStats.Sys = 1
	}

	execCommand = mockExecCommand
	high, err := isMemoryUsageHigh("abcdef")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !high {
		t.Error("expected high memory usage to be detected")
	}
}

func TestIsMemoryUsageHigh_False(t *testing.T) {
	MemoryStatsWrapper = func(memStats *runtime.MemStats) {
		memStats.Alloc = 1024 * 1024 * 1024
		memStats.Sys = 8 * 1024 * 1024 * 1024
	}

	execCommand = mockExecCommand
	high, err := isMemoryUsageHigh("abcdef")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if high {
		t.Error("expected low memory usage not to be detected as high")
	}
}

func TestProcessBlob_SmallBlob(t *testing.T) {
	execCommand = mockExecCommand
	sha, err := ProcessBlob("abcdef", "file.txt", []string{"secret"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "newhash123" {
		t.Errorf("expected 'newhash123', got '%s'", sha)
	}
}

func TestProcessBlob_LargeBlob(t *testing.T) {
	execCommand = mockExecCommand
	sha, err := ProcessBlob("abcdef", "file.txt", []string{"secret"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "newhash123" {
		t.Errorf("expected 'newhash123', got '%s'", sha)
	}
}

func TestProcessLargeBlob_NoChanges(t *testing.T) {
	execCommand = mockExecCommand
	sha, err := ProcessLargeBlob("abcdef", "file.txt", []string{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "abcdef" {
		t.Errorf("expected 'abcdef', got '%s'", sha)
	}
}

func TestProcessLargeBlob_WithChanges(t *testing.T) {
	execCommand = mockExecCommand
	sha, err := ProcessLargeBlob("abcdef", "file.txt", []string{"secret"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "newhash123" {
		t.Errorf("expected 'newhash123', got '%s'", sha)
	}
}
