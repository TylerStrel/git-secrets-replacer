package replacer

import (
	"os"
	"testing"
)

func TestReadSecrets(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "secrets")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up

	content := []byte("secret1\nsecret2\nsecret3\n")
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	secrets, err := ReadSecrets(tmpfile.Name())
	if err != nil {
		t.Fatalf("ReadSecrets() error = %v", err)
	}

	expected := []string{"secret1", "secret2", "secret3"}
	if len(secrets) != len(expected) {
		t.Fatalf("expected %d secrets, got %d", len(expected), len(secrets))
	}
	for i, secret := range secrets {
		if secret != expected[i] {
			t.Errorf("expected secret %q, got %q", expected[i], secret)
		}
	}
}

func TestReadSecretsSorted(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "secrets")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := []byte("short\nmediumlength\nveryverylongsecret\n")
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	secrets, err := ReadSecrets(tmpfile.Name())
	if err != nil {
		t.Fatalf("ReadSecrets() error = %v", err)
	}

	expected := []string{"veryverylongsecret", "mediumlength", "short"}
	if len(secrets) != len(expected) {
		t.Fatalf("expected %d secrets, got %d", len(expected), len(secrets))
	}
	for i, secret := range secrets {
		if secret != expected[i] {
			t.Errorf("expected secret %q, got %q", expected[i], secret)
		}
	}
}
