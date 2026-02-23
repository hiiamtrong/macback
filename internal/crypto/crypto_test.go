package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSecret(t *testing.T) {
	tests := []struct {
		name             string
		filename         string
		categoryPatterns []string
		globalPatterns   []string
		want             bool
	}{
		{
			name:             "SSH private key matches",
			filename:         "id_ed25519",
			categoryPatterns: []string{"id_*", "!*.pub"},
			want:             true,
		},
		{
			name:             "SSH public key excluded by negation",
			filename:         "id_ed25519.pub",
			categoryPatterns: []string{"id_*", "!*.pub"},
			want:             false,
		},
		{
			name:           "PEM file matches global",
			filename:       "server.pem",
			globalPatterns: []string{"*.pem", "*.key"},
			want:           true,
		},
		{
			name:           "env file matches",
			filename:       ".env",
			globalPatterns: []string{".env", ".env.*"},
			want:           true,
		},
		{
			name:           "env.local matches",
			filename:       ".env.local",
			globalPatterns: []string{".env", ".env.*"},
			want:           true,
		},
		{
			name:             "regular config not secret",
			filename:         "config",
			categoryPatterns: []string{"id_*"},
			globalPatterns:   []string{"*.pem"},
			want:             false,
		},
		{
			name:             "known_hosts not secret",
			filename:         "known_hosts",
			categoryPatterns: []string{"id_*", "!*.pub"},
			want:             false,
		},
		{
			name:             "git-credentials is secret",
			filename:         ".git-credentials",
			categoryPatterns: []string{".git-credentials"},
			want:             true,
		},
		{
			name:     "no patterns means not secret",
			filename: "anything",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSecret(tt.filename, tt.categoryPatterns, tt.globalPatterns)
			if got != tt.want {
				t.Errorf("IsSecret(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestIsSecretWithPath(t *testing.T) {
	// IsSecret should use base name only
	got := IsSecret("/Users/test/.ssh/id_rsa", []string{"id_*"}, nil)
	if !got {
		t.Error("IsSecret should match base name from full path")
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "secret.txt")
	encPath := filepath.Join(dir, "secret.txt.age")
	decPath := filepath.Join(dir, "decrypted.txt")

	originalContent := "this is a secret message"
	if err := os.WriteFile(srcPath, []byte(originalContent), 0600); err != nil {
		t.Fatal(err)
	}

	passphrase := "test-passphrase-123"

	// Encrypt
	enc := NewPassphraseEncryptor(passphrase)
	resultPath, err := enc.EncryptFile(srcPath, encPath)
	if err != nil {
		t.Fatalf("EncryptFile() error: %v", err)
	}

	if resultPath != encPath {
		t.Errorf("result path = %q, want %q", resultPath, encPath)
	}

	// Verify encrypted file exists and differs from original
	encContent, err := os.ReadFile(encPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(encContent) == originalContent {
		t.Error("encrypted content should differ from original")
	}

	// Decrypt
	dec := NewPassphraseDecryptor(passphrase)
	if err := dec.DecryptFile(encPath, decPath); err != nil {
		t.Fatalf("DecryptFile() error: %v", err)
	}

	// Verify decrypted content matches original
	decContent, err := os.ReadFile(decPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(decContent) != originalContent {
		t.Errorf("decrypted = %q, want %q", decContent, originalContent)
	}
}

func TestEncryptAddsAgeExtension(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "secret.txt")
	dstPath := filepath.Join(dir, "secret.txt") // No .age extension

	os.WriteFile(srcPath, []byte("secret"), 0600)

	enc := NewPassphraseEncryptor("pass")
	resultPath, err := enc.EncryptFile(srcPath, dstPath)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if filepath.Ext(resultPath) != ".age" {
		t.Errorf("result path should have .age extension, got %q", resultPath)
	}
}

func TestDecryptWrongPassphrase(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "secret.txt")
	encPath := filepath.Join(dir, "secret.age")
	decPath := filepath.Join(dir, "decrypted.txt")

	os.WriteFile(srcPath, []byte("secret data"), 0600)

	// Encrypt with one passphrase
	enc := NewPassphraseEncryptor("correct-passphrase")
	enc.EncryptFile(srcPath, encPath)

	// Try to decrypt with wrong passphrase
	dec := NewPassphraseDecryptor("wrong-passphrase")
	err := dec.DecryptFile(encPath, decPath)
	if err == nil {
		t.Error("DecryptFile should fail with wrong passphrase")
	}
}

func TestEncryptEmptyFile(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "empty.txt")
	encPath := filepath.Join(dir, "empty.age")
	decPath := filepath.Join(dir, "decrypted.txt")

	os.WriteFile(srcPath, []byte(""), 0600)

	passphrase := "pass"

	enc := NewPassphraseEncryptor(passphrase)
	_, err := enc.EncryptFile(srcPath, encPath)
	if err != nil {
		t.Fatalf("EncryptFile() error: %v", err)
	}

	dec := NewPassphraseDecryptor(passphrase)
	if err := dec.DecryptFile(encPath, decPath); err != nil {
		t.Fatalf("DecryptFile() error: %v", err)
	}

	content, _ := os.ReadFile(decPath)
	if len(content) != 0 {
		t.Errorf("decrypted empty file should be empty, got %d bytes", len(content))
	}
}

func TestNullEncryptor(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "file.txt")
	dstPath := filepath.Join(dir, "copy.txt")

	original := "plain text"
	os.WriteFile(srcPath, []byte(original), 0644)

	enc := &NullEncryptor{}
	_, err := enc.EncryptFile(srcPath, dstPath)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	content, _ := os.ReadFile(dstPath)
	if string(content) != original {
		t.Errorf("NullEncryptor should copy as-is, got %q", content)
	}
}

func TestNullDecryptor(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "file.txt")
	dstPath := filepath.Join(dir, "copy.txt")

	original := "plain text"
	os.WriteFile(srcPath, []byte(original), 0644)

	dec := &NullDecryptor{}
	if err := dec.DecryptFile(srcPath, dstPath); err != nil {
		t.Fatalf("error: %v", err)
	}

	content, _ := os.ReadFile(dstPath)
	if string(content) != original {
		t.Errorf("NullDecryptor should copy as-is, got %q", content)
	}
}
