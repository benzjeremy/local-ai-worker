package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// PBKDF2Iterations defines the mandatory 100k rounds for Jeremy Benz security standards.
	PBKDF2Iterations = 100000
	// KeySize is 32 bytes for AES-256.
	KeySize = 32
	// SaltSize is 32 bytes for PBKDF2 salt.
	SaltSize = 32
	// NonceSize is 12 bytes for GCM.
	NonceSize = 12
)

// EncryptedPayload represents the serialized format of encrypted disk data.
type EncryptedPayload struct {
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

// VaultData holds internal persistent records including feedback loops and config.
type VaultData struct {
	FeedbackHistory []FeedbackRecord  `json:"feedback_history"`
	Settings        map[string]string `json:"settings"`
}

// FeedbackRecord stores an explicit correction from a user or supervisor.
type FeedbackRecord struct {
	ID                string   `json:"id"`
	Query             string   `json:"query"`
	OriginalResponse  string   `json:"original_response"`
	CorrectedResponse string   `json:"corrected_response"`
	ContextTags       []string `json:"context_tags"`
	Timestamp         string   `json:"timestamp"`
}

// Vault manages safe encrypted storage on disk.
type Vault struct {
	mu       sync.RWMutex
	filePath string
	key      []byte
	data     VaultData
}

// DeriveKey generates a 32-byte key from a passphrase and salt using PBKDF2 with 100k rounds.
func DeriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, PBKDF2Iterations, KeySize, sha256.New)
}

// Encrypt encrypts plaintext using AES-256-GCM.
func Encrypt(key, plaintext []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create gcm: %w", err)
	}

	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return nonce, ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM.
func Decrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcm: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (authentication tag mismatch): %w", err)
	}

	return plaintext, nil
}

// OpenVault opens or initializes an encrypted vault file.
func OpenVault(filePath, passphrase string) (*Vault, error) {
	if passphrase == "" {
		return nil, errors.New("passphrase cannot be empty")
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create vault dir: %w", err)
	}

	v := &Vault{
		filePath: filePath,
		data: VaultData{
			FeedbackHistory: make([]FeedbackRecord, 0),
			Settings:        make(map[string]string),
		},
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// New vault: generate fresh salt
		salt := make([]byte, SaltSize)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			return nil, fmt.Errorf("failed to generate salt: %w", err)
		}
		v.key = DeriveKey(passphrase, salt)
		if err := v.saveWithSalt(salt); err != nil {
			return nil, err
		}
		return v, nil
	}

	// Read existing vault
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read vault file: %w", err)
	}

	var payload EncryptedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("corrupted vault structure: %w", err)
	}

	if len(payload.Salt) < 16 || len(payload.Nonce) < 12 {
		return nil, errors.New("invalid cryptographic parameters in vault")
	}

	v.key = DeriveKey(passphrase, payload.Salt)
	plaintext, err := Decrypt(v.key, payload.Nonce, payload.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt vault (wrong password or altered data): %w", err)
	}

	if err := json.Unmarshal(plaintext, &v.data); err != nil {
		return nil, fmt.Errorf("failed to parse decrypted vault data: %w", err)
	}

	return v, nil
}

func (v *Vault) saveWithSalt(salt []byte) error {
	marshaled, err := json.Marshal(v.data)
	if err != nil {
		return fmt.Errorf("failed to marshal vault data: %w", err)
	}

	nonce, ciphertext, err := Encrypt(v.key, marshaled)
	if err != nil {
		return err
	}

	payload := EncryptedPayload{
		Salt:       salt,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}

	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(v.filePath, out, 0600)
}

func (v *Vault) saveLocked() error {
	raw, err := os.ReadFile(v.filePath)
	if err != nil {
		return fmt.Errorf("cannot read existing vault salt: %w", err)
	}

	var existing EncryptedPayload
	if err := json.Unmarshal(raw, &existing); err != nil {
		return fmt.Errorf("cannot parse existing vault salt: %w", err)
	}

	return v.saveWithSalt(existing.Salt)
}

// Save persists the current vault state using the existing key.
func (v *Vault) Save() error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.saveLocked()
}

// AddFeedback records a feedback entry.
func (v *Vault) AddFeedback(rec FeedbackRecord) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.data.FeedbackHistory = append(v.data.FeedbackHistory, rec)
	return v.saveLocked()
}

// GetFeedback returns a slice of all recorded feedback items.
func (v *Vault) GetFeedback() []FeedbackRecord {
	v.mu.RLock()
	defer v.mu.RUnlock()

	res := make([]FeedbackRecord, len(v.data.FeedbackHistory))
	copy(res, v.data.FeedbackHistory)
	return res
}

// SetSetting saves a key-value setting.
func (v *Vault) SetSetting(k, val string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.data.Settings[k] = val
	return v.saveLocked()
}

// GetSetting retrieves a setting value.
func (v *Vault) GetSetting(k string) string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	return v.data.Settings[k]
}
