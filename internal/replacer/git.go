package replacer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var CommitMap = make(map[string]string)

func GetRefs() ([]string, error) {
	output, err := exec.Command("git", "for-each-ref", "--format=%(refname)").Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(string(output)), "\n"), nil
}

func GetCommits(ref string) ([]string, error) {
	output, err := exec.Command("git", "rev-list", ref).Output()
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(string(output)), "\n"), nil
}

func UpdateRef(ref, newCommitHash string) error {
	fmt.Printf("Updating ref %s to new commit hash %s\n", ref, newCommitHash)
	err := exec.Command("git", "update-ref", ref, newCommitHash).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating ref %s: %v\n", ref, err)
	}
	return err
}

func ForcePush(ref string) error {
	fmt.Printf("Force pushing ref %s to origin\n", ref)
	err := exec.Command("git", "push", "origin", "--force", ref).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error force pushing ref %s: %v\n", ref, err)
	}
	return err
}
