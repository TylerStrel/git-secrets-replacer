package replacer

import (
	"bufio"
	"os"
	"strings"
)

func ReadSecrets(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var secrets []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		secret := strings.TrimSpace(scanner.Text())
		if secret != "" {
			secrets = append(secrets, secret)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return secrets, nil
}
