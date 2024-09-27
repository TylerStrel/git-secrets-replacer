package replacer

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

var commitCache = sync.Map{}
var treeCache = sync.Map{}

func ProcessCommit(commit string, secrets []string) (string, error) {
	if newCommit, found := CommitMap[commit]; found {
		return newCommit, nil
	}

	tree, err := GetTree(commit)
	if err != nil {
		return "", err
	}

	newTree, err := ProcessTree(tree, secrets)
	if err != nil {
		return "", err
	}

	output, err := GetCachedGitOutput("git", "cat-file", "-p", commit)
	if err != nil {
		return "", err
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

	cmd := exec.Command("git", "hash-object", "-t", "commit", "-w", "--stdin")
	cmd.Stdin = strings.NewReader(strings.Join(newCommit, "\n") + "\n")
	newCommitHash, err := cmd.Output()
	if err != nil {
		return "", err
	}

	newCommitHashStr := strings.TrimSpace(string(newCommitHash))
	CommitMap[commit] = newCommitHashStr
	fmt.Println("New commit hash:", newCommitHashStr)

	return newCommitHashStr, nil
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
	return "", fmt.Errorf("no tree found in commit %s", commit)
}

func ProcessTree(tree string, secrets []string) (string, error) {
	output, err := GetCachedGitOutput("git", "cat-file", "-p", tree)
	if err != nil {
		return "", err
	}

	var newEntries []string
	changed := false

	lines := strings.Split(string(output), "\n")
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(chan string, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		wg.Add(1)
		go func(line string) {
			defer wg.Done()
			parts := strings.Split(line, "\t")
			modeAndSha := parts[0]
			path := parts[1]

			mode := strings.Split(modeAndSha, " ")[0]
			sha := strings.Split(modeAndSha, " ")[2]

			var newSha string
			if mode == "040000" {
				newSha, err = ProcessTree(sha, secrets)
				if err != nil {
					results <- ""
					return
				}
				mu.Lock()
				newEntries = append(newEntries, fmt.Sprintf("%s tree %s\t%s", mode, newSha, path))
				mu.Unlock()
			} else if mode == "100644" || mode == "100755" {
				newSha, err = ProcessBlobWithGoroutines(sha, path, secrets)
				if err != nil {
					results <- ""
					return
				}
				mu.Lock()
				newEntries = append(newEntries, fmt.Sprintf("%s blob %s\t%s", mode, newSha, path))
				mu.Unlock()
			} else {
				newSha = sha
				mu.Lock()
				newEntries = append(newEntries, fmt.Sprintf("%s %s\t%s", mode, newSha, path))
				mu.Unlock()
			}

			if newSha != sha {
				changed = true
			}
			results <- newSha
		}(line)
	}

	wg.Wait()
	close(results)

	if !changed {
		return tree, nil
	}

	newTree, err := WriteTree(newEntries)
	if err != nil {
		return "", err
	}

	return newTree, nil
}

func ProcessBlobWithGoroutines(sha, path string, secrets []string) (string, error) {
	output, err := GetCachedGitOutput("git", "cat-file", "-p", sha)
	if err != nil {
		return "", err
	}

	if IsBinary(output) {
		return sha, nil
	}

	changed := false
	content := string(output)
	var mu sync.Mutex
	wg := sync.WaitGroup{}
	numWorkers := runtime.NumCPU()
	jobs := make(chan string, len(secrets))
	results := make(chan string, len(secrets))

	compiledRegexes := make([]*regexp.Regexp, len(secrets))
	for i, secret := range secrets {
		escapedRegex := regexp.QuoteMeta(secret)
		compiledRegexes[i] = regexp.MustCompile(escapedRegex)
	}

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for secret := range jobs {
				for _, regex := range compiledRegexes {
					if regex.Match(output) {
						mu.Lock()
						content = regex.ReplaceAllString(content, "**REMOVED**")
						changed = true
						mu.Unlock()
					}
				}
				results <- secret
			}
		}()
	}

	for _, secret := range secrets {
		jobs <- secret
	}
	close(jobs)
	wg.Wait()
	close(results)

	if !changed {
		return sha, nil
	}

	fmt.Println("Found and replaced sensitive string in file:", path)
	newContent := []byte(content)

	newSha, err := WriteBlob(newContent)
	if err != nil {
		return "", err
	}

	return newSha, nil
}

func IsBinary(content []byte) bool {
	return bytes.IndexByte(content, 0) != -1
}

func WriteBlob(content []byte) (string, error) {
	cmd := exec.Command("git", "hash-object", "-w", "--stdin")
	cmd.Stdin = bytes.NewReader(content)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

func WriteTree(entries []string) (string, error) {
	cmd := exec.Command("git", "mktree")
	cmd.Stdin = strings.NewReader(strings.Join(entries, "\n"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

func GetCachedGitOutput(args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	if output, found := commitCache.Load(key); found {
		return output.([]byte), nil
	}

	output, err := exec.Command(args[0], args[1:]...).Output()
	if err != nil {
		return nil, err
	}

	commitCache.Store(key, output)
	return output, nil
}
