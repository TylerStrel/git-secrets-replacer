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

func getBanner() string {
	return `
  ____ _ _   ____                     _       ____            _                
 / ___(_) |_/ ___|  ___  ___ _ __ ___| |_ ___|  _ \ ___ _ __ | | __ _  ___ ___  _ __ 
| |  _| | __\___ \ / _ \/ __| '__/ _ \ __/ __| |_) / _ \ '_ \| |/ _' |/ __/ _ \| '__|
| |_| | | |_ ___) |  __/ (__| | |  __/ |_\__ \  _ <  __/ |_) | | (_| | (_|  __/| |   
 \____|_|\__|____/ \___|\___|_|  \___|\__|___/_| \_\___| .__/|_|\__,_|\___\___||_|   
                                                       |_|                     
`
}

func displayUsageInstructions() {
	fmt.Println(`Usage Instructions:
1. Enter the path to the repository where the code will run.
2. Enter the path to the file containing all the secrets that need to be removed.
3. Choose whether the changes should be force pushed to the remote/origin (true/false).

For any issues, feature requests, or more information, visit:
https://github.com/TylerStrel/git-secrets-replacer`)
}
func readSecretsFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var secrets []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		secrets = append(secrets, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return secrets, nil
}

func main() {
	fmt.Println(getBanner())
	displayUsageInstructions()

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

	var err error
	secrets, err = readSecretsFile(secretsFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading secrets file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nPlease validate the settings:")
	fmt.Println("Repository Path:", repoPath)
	fmt.Println("Secrets File Path:", secretsFilePath)
	fmt.Println("Force Push to Origin:", forcePushToOrigin)
	fmt.Println("Secrets:")
	for _, secret := range secrets {
		fmt.Println("-", secret)
	}

	fmt.Print("\nAre these settings correct? (yes/no): ")
	validationResponse, _ := reader.ReadString('\n')
	validationResponse = strings.TrimSpace(validationResponse)

	if !(strings.ToLower(validationResponse) == "yes" || strings.ToLower(validationResponse) == "y") {
		fmt.Println("Exiting. Please run the program again with the correct settings.")
		os.Exit(1)
	}

	fmt.Println("Changing directory to:", repoPath)
	if err := os.Chdir(repoPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error changing directory: %v\n", err)
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

	fmt.Println("\nFor any issues, feature requests, or more information, visit:")
	fmt.Println("https://github.com/TylerStrel/git-secrets-replacer")
}
