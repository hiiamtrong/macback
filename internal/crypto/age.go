package crypto

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"filippo.io/age"
)

// Encryptor encrypts files.
type Encryptor interface {
	EncryptFile(srcPath, dstPath string) (string, error)
}

// Decryptor decrypts files.
type Decryptor interface {
	DecryptFile(srcPath, dstPath string) error
}

// PassphraseEncryptor implements Encryptor using age passphrase (scrypt).
type PassphraseEncryptor struct {
	passphrase string
}

// NewPassphraseEncryptor creates an encryptor with the given passphrase.
func NewPassphraseEncryptor(passphrase string) *PassphraseEncryptor {
	return &PassphraseEncryptor{passphrase: passphrase}
}

// EncryptFile encrypts srcPath and writes to dstPath.
// Returns the path of the encrypted file (with .age extension appended if not already).
func (e *PassphraseEncryptor) EncryptFile(srcPath, dstPath string) (string, error) {
	// Ensure .age extension
	if filepath.Ext(dstPath) != ".age" {
		dstPath += ".age"
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return "", fmt.Errorf("creating destination directory: %w", err)
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("opening source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", fmt.Errorf("creating destination: %w", err)
	}
	defer dstFile.Close()

	recipient, err := age.NewScryptRecipient(e.passphrase)
	if err != nil {
		return "", fmt.Errorf("creating scrypt recipient: %w", err)
	}

	writer, err := age.Encrypt(dstFile, recipient)
	if err != nil {
		return "", fmt.Errorf("initializing encryption: %w", err)
	}

	if _, err := io.Copy(writer, srcFile); err != nil {
		return "", fmt.Errorf("encrypting data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("finalizing encryption: %w", err)
	}

	return dstPath, nil
}

// PassphraseDecryptor implements Decryptor using age passphrase (scrypt).
type PassphraseDecryptor struct {
	passphrase string
}

// NewPassphraseDecryptor creates a decryptor with the given passphrase.
func NewPassphraseDecryptor(passphrase string) *PassphraseDecryptor {
	return &PassphraseDecryptor{passphrase: passphrase}
}

// DecryptFile decrypts srcPath (a .age file) and writes to dstPath.
func (d *PassphraseDecryptor) DecryptFile(srcPath, dstPath string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("opening encrypted file: %w", err)
	}
	defer srcFile.Close()

	identity, err := age.NewScryptIdentity(d.passphrase)
	if err != nil {
		return fmt.Errorf("creating scrypt identity: %w", err)
	}

	reader, err := age.Decrypt(srcFile, identity)
	if err != nil {
		return fmt.Errorf("decryption failed (wrong passphrase?): %w", err)
	}

	dstFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, reader); err != nil {
		return fmt.Errorf("writing decrypted data: %w", err)
	}

	return nil
}

// NullEncryptor is a no-op encryptor used when encryption is disabled.
type NullEncryptor struct{}

// EncryptFile for NullEncryptor just copies the file as-is.
func (n *NullEncryptor) EncryptFile(srcPath, dstPath string) (string, error) {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return "", err
	}

	dstFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return "", err
	}

	return dstPath, nil
}

// NullDecryptor is a no-op decryptor used when encryption is disabled.
type NullDecryptor struct{}

// DecryptFile for NullDecryptor just copies the file as-is.
func (n *NullDecryptor) DecryptFile(srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
