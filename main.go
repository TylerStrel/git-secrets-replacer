package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/TylerStrel/git-secrets-replacer/internal/replacer"
)

var (
	repoPath          string
	secretsFilePath   string
	forcePushToOrigin bool
	secrets           []string
)

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter the path to the repo that the code will run on: ")
	repoPath, _ = reader.ReadString('\n')
	repoPath = strings.TrimSpace(repoPath)

	fmt.Print("Enter the path to the file containing all the secrets that need to be removed: ")
	secretsFilePath, _ = reader.ReadString('\n')
	secretsFilePath = strings.TrimSpace(secretsFilePath)

	fmt.Print("Should the code force push the changes to the remote/origin (true/false)? ")
	shouldForcePush, _ := reader.ReadString('\n')
	shouldForcePush = strings.TrimSpace(shouldForcePush)
	forcePushToOrigin = strings.ToLower(shouldForcePush) == "true"

	fmt.Println("Changing directory to:", repoPath)
	if err := os.Chdir(repoPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error changing directory: %v\n", err)
		os.Exit(1)
	}

	var err error
	secrets, err = replacer.ReadSecrets(secretsFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading secrets file: %v\n", err)
		os.Exit(1)
	}

	refs, err := replacer.GetRefs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting refs: %v\n", err)
		os.Exit(1)
	}

	for _, ref := range refs {
		fmt.Println("Processing ref:", ref)
		commits, err := replacer.GetCommits(ref)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting commits for ref %s: %v\n", ref, err)
			os.Exit(1)
		}

		// Process commits in reverse order
		var newHead string
		for i := len(commits) - 1; i >= 0; i-- {
			commit := commits[i]
			fmt.Println("Processing commit:", commit)
			newCommit, err := replacer.ProcessCommit(commit, secrets)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error processing commit %s: %v\n", commit, err)
				os.Exit(1)
			}
			replacer.CommitMap[commit] = newCommit
			newHead = newCommit
		}

		fmt.Println("Updating ref:", ref, "to new commit hash:", newHead)
		if err := replacer.UpdateRef(ref, newHead); err != nil {
			fmt.Fprintf(os.Stderr, "Error updating ref %s: %v\n", ref, err)
			os.Exit(1)
		}

		if forcePushToOrigin {
			if err := replacer.ForcePush(ref); err != nil {
				fmt.Fprintf(os.Stderr, "Error force pushing to origin: %v\n", err)
				os.Exit(1)
			}
		}
	}

	fmt.Println("Repository has been rewritten successfully.")
}
