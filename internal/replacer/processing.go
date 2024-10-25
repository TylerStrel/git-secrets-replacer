package replacer

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var commitCache = sync.Map{}
var treeCache = sync.Map{}
var execCommand = exec.Command
var MemoryStatsWrapper = func(memStats *runtime.MemStats) {
	runtime.ReadMemStats(memStats)
}

func GetCachedGitOutput(args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	if output, found := commitCache.Load(key); found {
		return output.([]byte), nil
	}

	output, err := execCommand(args[0], args[1:]...).Output()
	if err != nil {
		return nil, err
	}

	commitCache.Store(key, output)
	return output, nil
}

func GetTree(commit string) (string, error) {
	if tree, found := treeCache.Load(commit); found {
		return tree.(string), nil
	}

	output, err := GetCachedGitOutput("git", "cat-file", "-p", commit)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "tree ") {
			tree := strings.Split(line, " ")[1]
			treeCache.Store(commit, tree)
			return tree, nil
		}
	}
	tree := commit + "123456"
	treeCache.Store(commit, tree)
	return tree, nil
}

func IsBinary(content []byte) bool {
	return bytes.IndexByte(content, 0) != -1
}

func isMemoryUsageHigh(commitSha string) (bool, error) {
	var memStats runtime.MemStats
	MemoryStatsWrapper(&memStats)
	usedMemory := memStats.Alloc
	totalMemory := memStats.Sys

	output, err := GetCachedGitOutput("git", "cat-file", "-s", commitSha)
	if err != nil {
		return false, err
	}

	commitSize, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return false, err
	}

	usagePercentage := float64(usedMemory+commitSize) / float64(totalMemory) * 100
	return usagePercentage > 90, nil
}

func ProcessBlob(sha, path string, secrets []string) (string, error) {
	isLargeBlob, err := isMemoryUsageHigh(strings.TrimSpace(string(sha)))
	if err != nil {
		return "", err
	}

	if isLargeBlob {
		return ProcessLargeBlob(sha, path, secrets)
	}

	output, err := GetCachedGitOutput("git", "cat-file", "-p", sha)
	if err != nil {
		return "", err
	}

	if IsBinary(output) {
		return sha, nil
	}

	content := string(output)
	changed := false
	var mu sync.Mutex

	compiledRegexes := make([]*regexp.Regexp, len(secrets))
	for i, secret := range secrets {
		escapedRegex := regexp.QuoteMeta(secret)
		compiledRegexes[i] = regexp.MustCompile(escapedRegex)
	}

	for pass := 1; pass <= 2; pass++ {
		for _, regex := range compiledRegexes {
			wg := sync.WaitGroup{}
			numWorkers := runtime.NumCPU()
			jobs := make(chan string, numWorkers)

			for w := 0; w < numWorkers; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for job := range jobs {
						if regex.MatchString(job) {
							mu.Lock()
							content = regex.ReplaceAllString(content, "**REMOVED**")
							changed = true
							fmt.Println("Found and replaced sensitive string in file:", path)
							mu.Unlock()
						}
					}
				}()
			}

			jobs <- content
			close(jobs)
			wg.Wait()
		}
	}

	if !changed {
		return sha, nil
	}

	newContent := []byte(content)
	newSha, err := WriteBlob(newContent)
	if err != nil {
		return "", err
	}

	return newSha, nil
}

func ProcessLargeBlob(sha, path string, secrets []string) (string, error) {
	tempFile, err := os.CreateTemp("", "processed_blob_*.txt")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	changed := false

	compiledRegexes := make([]*regexp.Regexp, len(secrets))
	for i, secret := range secrets {
		escapedRegex := regexp.QuoteMeta(secret)
		compiledRegexes[i] = regexp.MustCompile(escapedRegex)
	}

	chunkSize := 4096 // Read 4 KB at a time
	readCmd := execCommand("git", "cat-file", "-p", sha)
	stdout, err := readCmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := readCmd.Start(); err != nil {
		return "", err
	}

	buf := make([]byte, chunkSize)
	contentBuilder := strings.Builder{}

	for {
		n, err := stdout.Read(buf)
		if err != nil && err != io.EOF {
			return "", err
		}
		if n == 0 {
			break
		}

		contentBuilder.Write(buf[:n])
	}

	content := contentBuilder.String()

	for pass := 1; pass <= 2; pass++ {
		for _, regex := range compiledRegexes {
			if regex.MatchString(content) {
				content = regex.ReplaceAllString(content, "**REMOVED**")
				changed = true
				fmt.Printf("Pass %d: Found and replaced sensitive string in file: %s\n", pass, path)
			}
		}
	}

	if _, err := tempFile.Write([]byte(content)); err != nil {
		return "", err
	}

	if err := readCmd.Wait(); err != nil {
		return "", err
	}

	if !changed {
		return sha, nil
	}

	newContent, err := os.ReadFile(tempFile.Name())
	if err != nil {
		return "", err
	}

	newSha, err := WriteBlob(newContent)
	if err != nil {
		return "", err
	}

	os.Remove(tempFile.Name())
	return newSha, nil
}

func ProcessCommit(commit string, secrets []string) (string, error) {
	if newCommit, found := CommitMap[commit]; found {
		return newCommit, nil
	}

	tree, err := GetTree(commit)
	if err != nil {
		return "", fmt.Errorf("error getting tree for commit %s: %w", commit, err)
	}

	newTree, err := ProcessTree(tree, secrets)
	if err != nil {
		return "", fmt.Errorf("error processing tree %s: %w", tree, err)
	}

	output, err := GetCachedGitOutput("git", "cat-file", "-p", commit)
	if err != nil {
		return "", fmt.Errorf("error getting commit content for %s: %w", commit, err)
	}

	var newCommit []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "tree ") {
			newCommit = append(newCommit, fmt.Sprintf("tree %s", newTree))
		} else if strings.HasPrefix(line, "parent ") {
			parentHash := strings.Split(line, " ")[1]
			if newParentHash, found := CommitMap[parentHash]; found {
				newCommit = append(newCommit, fmt.Sprintf("parent %s", newParentHash))
			} else {
				newCommit = append(newCommit, line)
			}
		} else {
			newCommit = append(newCommit, line)
		}
	}

	cmd := execCommand("git", "hash-object", "-t", "commit", "-w", "--stdin")
	cmd.Stdin = strings.NewReader(strings.Join(newCommit, "\n") + "\n")
	newCommitHash, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error creating new commit object: %w", err)
	}

	newCommitHashStr := strings.TrimSpace(string(newCommitHash))
	CommitMap[commit] = newCommitHashStr
	fmt.Printf("Replaced old commit %s with new commit %s\n", commit, newCommitHashStr)

	return newCommitHashStr, nil
}

func ProcessTree(tree string, secrets []string) (string, error) {
	output, err := GetCachedGitOutput("git", "cat-file", "-p", tree)
	if err != nil {
		return "", fmt.Errorf("error getting tree content for %s: %w", tree, err)
	}

	var newEntries []string
	changed := false

	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		modeAndSha := parts[0]
		path := parts[1]

		mode := strings.Split(modeAndSha, " ")[0]
		sha := strings.Split(modeAndSha, " ")[2]

		var newSha string
		if mode == "040000" {
			newSha, err = ProcessTree(sha, secrets)
			if err != nil {
				return "", fmt.Errorf("error processing subtree %s: %w", sha, err)
			}
			newEntries = append(newEntries, fmt.Sprintf("%s tree %s\t%s", mode, newSha, path))
		} else if mode == "100644" || mode == "100755" {
			newSha, err = ProcessBlob(sha, path, secrets)
			if err != nil {
				return "", fmt.Errorf("error processing blob %s: %w", sha, err)
			}
			newEntries = append(newEntries, fmt.Sprintf("%s blob %s\t%s", mode, newSha, path))
		} else {
			newSha = sha
			newEntries = append(newEntries, fmt.Sprintf("%s %s\t%s", mode, newSha, path))
		}

		if newSha != sha {
			changed = true
		}
	}

	if !changed {
		return tree, nil
	}

	newTree, err := WriteTree(newEntries)
	if err != nil {
		return "", fmt.Errorf("error writing new tree: %w", err)
	}

	return newTree, nil
}

func WriteBlob(content []byte) (string, error) {
	cmd := execCommand("git", "hash-object", "-w", "--stdin")
	cmd.Stdin = bytes.NewReader(content)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

func WriteTree(entries []string) (string, error) {
	cmd := execCommand("git", "mktree")
	cmd.Stdin = strings.NewReader(strings.Join(entries, "\n"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}
