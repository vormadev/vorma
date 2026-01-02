package cryptoutil

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/vormadev/vorma/kit/bytesutil"
	"golang.org/x/crypto/nacl/auth"
)

const (
	aesNonceSize               = 12 // Size of AES-GCM nonce
	xChaCha20Poly1305NonceSize = 24 // Size of XChaCha20-Poly1305 nonce
)

func new32() *[32]byte {
	return &[32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
}

func TestRandom(t *testing.T) {
	// Test generating random bytes
	byteLen := 16
	randomBytes, err := RandomBytes(byteLen)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(randomBytes) != byteLen {
		t.Fatalf("expected random byte slice of length %d, got %d", byteLen, len(randomBytes))
	}

	// Test randomness by generating another set and comparing
	anotherRandomBytes, err := RandomBytes(byteLen)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if bytes.Equal(randomBytes, anotherRandomBytes) {
		t.Fatalf("expected different random byte slices, got identical slices")
	}

	// Test Random with a byte length of 0
	zeroLengthBytes, err := RandomBytes(0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(zeroLengthBytes) != 0 {
		t.Fatalf("expected empty byte slice, got %d bytes", len(zeroLengthBytes))
	}
}

func TestSignSymmetric(t *testing.T) {
	secretKey := new32()
	message := []byte("test message")

	signedMsg, err := SignSymmetric(message, secretKey)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(signedMsg) != auth.Size+len(message) {
		t.Fatalf("expected signed message length %d, got %d", auth.Size+len(message), len(signedMsg))
	}

	// Test that the signed message contains the original message
	if !bytes.Equal(signedMsg[auth.Size:], message) {
		t.Fatalf("expected signed message to contain original message")
	}
}

func TestVerifyAndReadSymmetric(t *testing.T) {
	secretKey := new32()
	message := []byte("test message")

	signedMsg, _ := SignSymmetric(message, secretKey)

	// Successful verification
	retrievedMsg, err := VerifyAndReadSymmetric(signedMsg, secretKey)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !bytes.Equal(retrievedMsg, message) {
		t.Fatalf("expected retrieved message to equal original message")
	}

	// Invalid signature (corrupt the signed message)
	signedMsg[0] ^= 0xFF // flip a bit in the signature
	_, err = VerifyAndReadSymmetric(signedMsg, secretKey)
	if err == nil {
		t.Fatalf("expected error due to invalid signature, got nil")
	}

	// Truncated message
	truncatedMsg := signedMsg[:auth.Size-1]
	_, err = VerifyAndReadSymmetric(truncatedMsg, secretKey)
	if err == nil {
		t.Fatalf("expected error due to truncated message, got nil")
	}
}

func TestVerifyAndReadAsymmetric(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	message := []byte("test message")
	signature := ed25519.Sign(privateKey, message)
	signedMsg := append(signature, message...)

	// Convert public key to [32]byte format
	var publicKey32 [32]byte
	copy(publicKey32[:], publicKey)

	// Successful verification
	retrievedMsg, err := VerifyAndReadAsymmetric(signedMsg, &publicKey32)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !bytes.Equal(retrievedMsg, message) {
		t.Fatalf("expected retrieved message to equal original message")
	}

	// Invalid signature (corrupt the signed message)
	signedMsg[0] ^= 0xFF // flip a bit in the signature
	_, err = VerifyAndReadAsymmetric(signedMsg, &publicKey32)
	if err == nil {
		t.Fatalf("expected error due to invalid signature, got nil")
	}

	// Truncated message
	truncatedMsg := signedMsg[:len(signedMsg)-1]
	_, err = VerifyAndReadAsymmetric(truncatedMsg, &publicKey32)
	if err == nil {
		t.Fatalf("expected error due to truncated message, got nil")
	}
}

func TestVerifyAndReadAsymmetricBase64(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}

	message := []byte("test message")
	signature := ed25519.Sign(privateKey, message)
	signedMsg := append(signature, message...)

	signedMsgBase64 := bytesutil.ToBase64(signedMsg)
	publicKeyBase64 := bytesutil.ToBase64(publicKey)

	// Successful verification
	retrievedMsg, err := VerifyAndReadAsymmetricBase64(signedMsgBase64, publicKeyBase64)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !bytes.Equal(retrievedMsg, message) {
		t.Fatalf("expected retrieved message to equal original message")
	}

	// Invalid base64 signature
	_, err = VerifyAndReadAsymmetricBase64("invalid_base64", publicKeyBase64)
	if err == nil {
		t.Fatalf("expected error due to invalid base64 signature, got nil")
	}

	// Invalid base64 public key
	_, err = VerifyAndReadAsymmetricBase64(signedMsgBase64, "invalid_base64")
	if err == nil {
		t.Fatalf("expected error due to invalid base64 public key, got nil")
	}

	// Invalid signature (corrupt the signed message)
	tampered := make([]byte, len(signedMsg))
	copy(tampered, signedMsg)
	tampered[0] ^= 0xFF
	tamperedBase64 := bytesutil.ToBase64(tampered)
	_, err = VerifyAndReadAsymmetricBase64(tamperedBase64, publicKeyBase64)
	if err == nil {
		t.Fatalf("expected error due to invalid signature, got nil")
	}
}

func TestEdgeCases(t *testing.T) {
	secretKey := new32()
	publicKey, _, _ := ed25519.GenerateKey(rand.Reader)
	var publicKey32 [32]byte
	copy(publicKey32[:], publicKey)

	// Empty message for symmetric signing
	signedMsg, err := SignSymmetric([]byte{}, secretKey)
	if err != nil {
		t.Fatalf("expected no error for empty message, got %v", err)
	}
	if len(signedMsg) != auth.Size {
		t.Fatalf("expected signed message length %d, got %d", auth.Size, len(signedMsg))
	}

	// Empty signed message for symmetric verification
	_, err = VerifyAndReadSymmetric([]byte{}, secretKey)
	if err == nil {
		t.Fatalf("expected error due to empty signed message, got nil")
	}

	// Empty signed message for asymmetric verification
	_, err = VerifyAndReadAsymmetric([]byte{}, &publicKey32)
	if err == nil {
		t.Fatalf("expected error due to empty signed message, got nil")
	}

	// Nil secret key for symmetric signing
	_, err = SignSymmetric([]byte("test"), nil)
	if err == nil {
		t.Fatalf("expected error due to nil secret key, got nil")
	}

	// Nil secret key for symmetric verification
	_, err = VerifyAndReadSymmetric([]byte("test"), nil)
	if err == nil {
		t.Fatalf("expected error due to nil secret key, got nil")
	}

	// Nil public key for asymmetric verification
	_, err = VerifyAndReadAsymmetric([]byte("test"), nil)
	if err == nil {
		t.Fatalf("expected error due to nil public key, got nil")
	}
}

func TestEncryptSymmetricAESGCM(t *testing.T) {
	secretKey := new32()
	message := []byte("test message for encryption")

	// Test successful encryption
	encrypted, err := EncryptSymmetricAESGCM(message, secretKey)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(encrypted) <= aesNonceSize {
		t.Fatalf("expected encrypted message to be longer than nonce, got length %d", len(encrypted))
	}

	// Test that encrypted message is different from original
	if bytes.Equal(encrypted[aesNonceSize:], message) {
		t.Fatalf("encrypted message should not be equal to original message")
	}

	// Test encryption with nil secret key
	_, err = EncryptSymmetricAESGCM(message, nil)
	if err == nil {
		t.Fatalf("expected error with nil secret key, got nil")
	}

	// Test encryption of empty message
	emptyEncrypted, err := EncryptSymmetricAESGCM([]byte{}, secretKey)
	if err != nil {
		t.Fatalf("expected no error for empty message, got %v", err)
	}
	if len(emptyEncrypted) <= aesNonceSize {
		t.Fatalf("expected encrypted empty message to be longer than nonce, got length %d", len(emptyEncrypted))
	}
}

func TestDecryptSymmetricAESGCM(t *testing.T) {
	secretKey := new32()
	message := []byte("test message for decryption")

	// Test successful encryption and decryption
	encrypted, _ := EncryptSymmetricAESGCM(message, secretKey)
	decrypted, err := DecryptSymmetricAESGCM(encrypted, secretKey)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !bytes.Equal(decrypted, message) {
		t.Fatalf("decrypted message does not match original")
	}

	// Test decryption with wrong key
	wrongKey := new32()
	wrongKey[0] ^= 0xFF // Flip a bit to make it different
	_, err = DecryptSymmetricAESGCM(encrypted, wrongKey)
	if err == nil {
		t.Fatalf("expected error with wrong key, got nil")
	}

	// Test decryption of tampered message
	tampered := make([]byte, len(encrypted))
	copy(tampered, encrypted)
	tampered[len(tampered)-1] ^= 0xFF // Flip the last bit
	_, err = DecryptSymmetricAESGCM(tampered, secretKey)
	if err == nil {
		t.Fatalf("expected error with tampered message, got nil")
	}

	// Test decryption with nil secret key
	_, err = DecryptSymmetricAESGCM(encrypted, nil)
	if err == nil {
		t.Fatalf("expected error with nil secret key, got nil")
	}

	// Test decryption of message that's too short
	_, err = DecryptSymmetricAESGCM(encrypted[:aesNonceSize-1], secretKey)
	if err == nil {
		t.Fatalf("expected error with short message, got nil")
	}

	// Test decryption of empty message
	_, err = DecryptSymmetricAESGCM([]byte{}, secretKey)
	if err == nil {
		t.Fatalf("expected error with empty message, got nil")
	}
}

func TestEncryptSymmetricXChaCha20(t *testing.T) {
	secretKey := new32()
	message := []byte("test message for encryption")

	// Test successful encryption
	encrypted, err := EncryptSymmetricXChaCha20Poly1305(message, secretKey)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(encrypted) <= xChaCha20Poly1305NonceSize {
		t.Fatalf("expected encrypted message to be longer than nonce, got length %d", len(encrypted))
	}

	// Test that encrypted message is different from original
	if bytes.Equal(encrypted[xChaCha20Poly1305NonceSize:], message) {
		t.Fatalf("encrypted message should not be equal to original message")
	}

	// Test encryption with nil secret key
	_, err = EncryptSymmetricXChaCha20Poly1305(message, nil)
	if err == nil {
		t.Fatalf("expected error with nil secret key, got nil")
	}

	// Test encryption of empty message
	emptyEncrypted, err := EncryptSymmetricXChaCha20Poly1305([]byte{}, secretKey)
	if err != nil {
		t.Fatalf("expected no error for empty message, got %v", err)
	}
	if len(emptyEncrypted) <= xChaCha20Poly1305NonceSize {
		t.Fatalf("expected encrypted empty message to be longer than nonce, got length %d", len(emptyEncrypted))
	}
}

func TestDecryptSymmetricXChaCha20(t *testing.T) {
	secretKey := new32()
	message := []byte("test message for decryption")

	// Test successful encryption and decryption
	encrypted, _ := EncryptSymmetricXChaCha20Poly1305(message, secretKey)
	decrypted, err := DecryptSymmetricXChaCha20Poly1305(encrypted, secretKey)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !bytes.Equal(decrypted, message) {
		t.Fatalf("decrypted message does not match original")
	}

	// Test decryption with wrong key
	wrongKey := new32()
	wrongKey[0] ^= 0xFF // Flip a bit to make it different
	_, err = DecryptSymmetricXChaCha20Poly1305(encrypted, wrongKey)
	if err == nil {
		t.Fatalf("expected error with wrong key, got nil")
	}

	// Test decryption of tampered message
	tampered := make([]byte, len(encrypted))
	copy(tampered, encrypted)
	tampered[len(tampered)-1] ^= 0xFF // Flip the last bit
	_, err = DecryptSymmetricXChaCha20Poly1305(tampered, secretKey)
	if err == nil {
		t.Fatalf("expected error with tampered message, got nil")
	}

	// Test decryption with nil secret key
	_, err = DecryptSymmetricXChaCha20Poly1305(encrypted, nil)
	if err == nil {
		t.Fatalf("expected error with nil secret key, got nil")
	}

	// Test decryption of message that's too short
	_, err = DecryptSymmetricXChaCha20Poly1305(encrypted[:xChaCha20Poly1305NonceSize-1], secretKey)
	if err == nil {
		t.Fatalf("expected error with short message, got nil")
	}

	// Test decryption of empty message
	_, err = DecryptSymmetricXChaCha20Poly1305([]byte{}, secretKey)
	if err == nil {
		t.Fatalf("expected error with empty message, got nil")
	}
}

func TestCrossEncryptionCompatibility(t *testing.T) {
	secretKey := new32()
	message := []byte("test message for cross-compatibility")

	// Test that AES-GCM encrypted messages can't be decrypted by XChaCha20
	encryptedAES, _ := EncryptSymmetricAESGCM(message, secretKey)
	_, err := DecryptSymmetricXChaCha20Poly1305(encryptedAES, secretKey)
	if err == nil {
		t.Fatalf("expected error decrypting AES-GCM message with XChaCha20, got nil")
	}

	// Test that XChaCha20 encrypted messages can't be decrypted by AES-GCM
	encryptedXChaCha, _ := EncryptSymmetricXChaCha20Poly1305(message, secretKey)
	_, err = DecryptSymmetricAESGCM(encryptedXChaCha, secretKey)
	if err == nil {
		t.Fatalf("expected error decrypting XChaCha20 message with AES-GCM, got nil")
	}
}

func TestReplayProtection(t *testing.T) {
	secretKey := new32()
	message := []byte("test message for replay protection")

	// Test AES-GCM replay protection (nonce reuse)
	encrypted1, _ := EncryptSymmetricAESGCM(message, secretKey)
	encrypted2, _ := EncryptSymmetricAESGCM(message, secretKey)
	if bytes.Equal(encrypted1, encrypted2) {
		t.Fatalf("AES-GCM: two encryptions of same message should produce different ciphertexts")
	}

	// Test XChaCha20 replay protection (nonce reuse)
	encrypted3, _ := EncryptSymmetricXChaCha20Poly1305(message, secretKey)
	encrypted4, _ := EncryptSymmetricXChaCha20Poly1305(message, secretKey)
	if bytes.Equal(encrypted3, encrypted4) {
		t.Fatalf("XChaCha20: two encryptions of same message should produce different ciphertexts")
	}
}

func TestConcurrentEncryption(t *testing.T) {
	secretKey := new32()
	message := []byte("test message for concurrent encryption")
	iterations := 100

	// Test concurrent AES-GCM encryption
	done := make(chan bool)
	for i := 0; i < iterations; i++ {
		go func() {
			encrypted, err := EncryptSymmetricAESGCM(message, secretKey)
			if err != nil {
				t.Errorf("concurrent AES-GCM encryption failed: %v", err)
			}
			decrypted, err := DecryptSymmetricAESGCM(encrypted, secretKey)
			if err != nil {
				t.Errorf("concurrent AES-GCM decryption failed: %v", err)
			}
			if !bytes.Equal(decrypted, message) {
				t.Errorf("concurrent AES-GCM decrypted message does not match original")
			}
			done <- true
		}()
	}

	// Wait for all AES-GCM goroutines
	for i := 0; i < iterations; i++ {
		<-done
	}

	// Test concurrent XChaCha20 encryption
	for i := 0; i < iterations; i++ {
		go func() {
			encrypted, err := EncryptSymmetricXChaCha20Poly1305(message, secretKey)
			if err != nil {
				t.Errorf("concurrent XChaCha20 encryption failed: %v", err)
			}
			decrypted, err := DecryptSymmetricXChaCha20Poly1305(encrypted, secretKey)
			if err != nil {
				t.Errorf("concurrent XChaCha20 decryption failed: %v", err)
			}
			if !bytes.Equal(decrypted, message) {
				t.Errorf("concurrent XChaCha20 decrypted message does not match original")
			}
			done <- true
		}()
	}

	// Wait for all XChaCha20 goroutines
	for i := 0; i < iterations; i++ {
		<-done
	}
}

func TestNonceUniqueness(t *testing.T) {
	secretKey := new32()
	message := []byte("test message for nonce uniqueness")
	nonceCount := 1000
	aesNonces := make(map[string]bool)
	chachaNonces := make(map[string]bool)

	// Test XChaCha20Poly1305 nonce uniqueness
	for i := 0; i < nonceCount; i++ {
		encrypted, err := EncryptSymmetricXChaCha20Poly1305(message, secretKey)
		if err != nil {
			t.Fatalf("XChaCha20Poly1305 encryption failed: %v", err)
		}
		nonce := string(encrypted[:xChaCha20Poly1305NonceSize])
		if chachaNonces[nonce] {
			t.Fatalf("XChaCha20Poly1305: duplicate nonce detected after %d iterations", i)
		}
		chachaNonces[nonce] = true
	}

	// Test AES-GCM nonce uniqueness
	for i := 0; i < nonceCount; i++ {
		encrypted, err := EncryptSymmetricAESGCM(message, secretKey)
		if err != nil {
			t.Fatalf("AES-GCM encryption failed: %v", err)
		}
		nonce := string(encrypted[:aesNonceSize])
		if aesNonces[nonce] {
			t.Fatalf("AES-GCM: duplicate nonce detected after %d iterations", i)
		}
		aesNonces[nonce] = true
	}
}

func TestLargeMessageEncryption(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large message test in short mode")
	}

	secretKey := new32()
	sizes := []int{
		64 * 1024,        // 64 KB
		1 * 1024 * 1024,  // 1 MB
		16 * 1024 * 1024, // 16 MB - tests chunking/buffering behavior
	}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size-%d", size), func(t *testing.T) {
			largeMessage := make([]byte, size)
			if _, err := rand.Read(largeMessage); err != nil {
				t.Fatalf("failed to generate random message: %v", err)
			}

			// Test XChaCha20Poly1305
			encrypted1, err := EncryptSymmetricXChaCha20Poly1305(largeMessage, secretKey)
			if err != nil {
				t.Fatalf("XChaCha20Poly1305 encryption failed: %v", err)
			}
			decrypted1, err := DecryptSymmetricXChaCha20Poly1305(encrypted1, secretKey)
			if err != nil {
				t.Fatalf("XChaCha20Poly1305 decryption failed: %v", err)
			}
			if !bytes.Equal(decrypted1, largeMessage) {
				t.Fatal("XChaCha20Poly1305: decrypted message does not match original")
			}

			// Test AES-GCM with same data
			encrypted2, err := EncryptSymmetricAESGCM(largeMessage, secretKey)
			if err != nil {
				t.Fatalf("AES-GCM encryption failed: %v", err)
			}
			decrypted2, err := DecryptSymmetricAESGCM(encrypted2, secretKey)
			if err != nil {
				t.Fatalf("AES-GCM decryption failed: %v", err)
			}
			if !bytes.Equal(decrypted2, largeMessage) {
				t.Fatal("AES-GCM: decrypted message does not match original")
			}
		})
	}
}

func TestTinyMessages(t *testing.T) {
	secretKey := new32()
	messages := [][]byte{
		{},           // empty
		{0},          // single byte
		{1, 2},       // two bytes
		{1, 2, 3, 4}, // few bytes
	}

	for _, msg := range messages {
		// Test XChaCha20Poly1305
		encrypted1, err := EncryptSymmetricXChaCha20Poly1305(msg, secretKey)
		if err != nil {
			t.Errorf("XChaCha20Poly1305 failed to encrypt %d byte message: %v", len(msg), err)
		}
		decrypted1, err := DecryptSymmetricXChaCha20Poly1305(encrypted1, secretKey)
		if err != nil {
			t.Errorf("XChaCha20Poly1305 failed to decrypt %d byte message: %v", len(msg), err)
		}
		if !bytes.Equal(decrypted1, msg) {
			t.Errorf("XChaCha20Poly1305 failed to round-trip %d byte message", len(msg))
		}

		// Test AES-GCM
		encrypted2, err := EncryptSymmetricAESGCM(msg, secretKey)
		if err != nil {
			t.Errorf("AES-GCM failed to encrypt %d byte message: %v", len(msg), err)
		}
		decrypted2, err := DecryptSymmetricAESGCM(encrypted2, secretKey)
		if err != nil {
			t.Errorf("AES-GCM failed to decrypt %d byte message: %v", len(msg), err)
		}
		if !bytes.Equal(decrypted2, msg) {
			t.Errorf("AES-GCM failed to round-trip %d byte message", len(msg))
		}
	}
}

func TestAuthenticationTag(t *testing.T) {
	secretKey := new32()
	message := []byte("test message")

	// Test XChaCha20Poly1305 authentication
	encrypted1, err := EncryptSymmetricXChaCha20Poly1305(message, secretKey)
	if err != nil {
		t.Fatal(err)
	}
	// Modify each byte at the end (where auth tag likely is)
	for i := 1; i <= 16; i++ {
		corrupt := make([]byte, len(encrypted1))
		copy(corrupt, encrypted1)
		corrupt[len(corrupt)-i] ^= 0x1
		_, err := DecryptSymmetricXChaCha20Poly1305(corrupt, secretKey)
		if err == nil {
			t.Errorf("XChaCha20Poly1305 did not detect corruption at byte -%d", i)
		}
	}

	// Test AES-GCM authentication
	encrypted2, err := EncryptSymmetricAESGCM(message, secretKey)
	if err != nil {
		t.Fatal(err)
	}
	// Modify each byte at the end (where auth tag likely is)
	for i := 1; i <= 16; i++ {
		corrupt := make([]byte, len(encrypted2))
		copy(corrupt, encrypted2)
		corrupt[len(corrupt)-i] ^= 0x1
		_, err := DecryptSymmetricAESGCM(corrupt, secretKey)
		if err == nil {
			t.Errorf("AES-GCM did not detect corruption at byte -%d", i)
		}
	}
}

func TestInvalidKeySize(t *testing.T) {
	// Test truncated public key
	shortKey := bytesutil.ToBase64(make([]byte, 16)) // too short
	longKey := bytesutil.ToBase64(make([]byte, 64))  // too long
	message := []byte("test")

	_, err := VerifyAndReadAsymmetricBase64(bytesutil.ToBase64(message), shortKey)
	if err == nil {
		t.Error("expected error with short public key")
	}

	_, err = VerifyAndReadAsymmetricBase64(bytesutil.ToBase64(message), longKey)
	if err == nil {
		t.Error("expected error with long public key")
	}

	_, err = VerifyAndReadAsymmetricBase64(bytesutil.ToBase64(message), "")
	if err == nil {
		t.Error("expected error with empty public key")
	}
}

func TestEmptyInputs(t *testing.T) {
	secretKey := new32()
	publicKey, _, _ := ed25519.GenerateKey(rand.Reader)
	var publicKey32 [32]byte
	copy(publicKey32[:], publicKey)

	// Test nil/empty byte slices for each function
	// SignSymmetric still needs to work with empty message
	signed, err := SignSymmetric(nil, secretKey)
	if err != nil {
		t.Errorf("SignSymmetric failed with nil message: %v", err)
	}
	if len(signed) != auth.Size {
		t.Errorf("SignSymmetric with nil message: expected length %d, got %d", auth.Size, len(signed))
	}

	// Test signature operations with empty string base64
	_, err = VerifyAndReadAsymmetricBase64("", bytesutil.ToBase64(publicKey))
	if err == nil {
		t.Error("expected error with empty signed message")
	}

	// Test nil/empty inputs for encryption
	for _, f := range []struct {
		name    string
		encrypt func([]byte, Key32) ([]byte, error)
	}{
		{"XChaCha20Poly1305", EncryptSymmetricXChaCha20Poly1305},
		{"AESGCM", EncryptSymmetricAESGCM},
	} {
		t.Run(f.name, func(t *testing.T) {
			// nil message should work (encrypting empty message)
			encrypted, err := f.encrypt(nil, secretKey)
			if err != nil {
				t.Errorf("encryption failed with nil message: %v", err)
			}
			if len(encrypted) < 16 { // at least nonce + tag
				t.Error("encrypted output too short")
			}
		})
	}
}

func TestExtremeMessageSizes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping extreme size tests in short mode")
	}

	secretKey := new32()
	// Test various message sizes including edge cases
	sizes := []int{
		0,             // empty
		1,             // minimum
		15,            // below typical block size
		16,            // typical block size
		17,            // above typical block size
		1024*1024 - 1, // just below 1MB
		1024 * 1024,   // exactly 1MB
		1024*1024 + 1, // just above 1MB
	}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size-%d", size), func(t *testing.T) {
			msg := make([]byte, size)
			rand.Read(msg)

			// Test both encryption methods
			encrypted1, err := EncryptSymmetricXChaCha20Poly1305(msg, secretKey)
			if err != nil {
				t.Errorf("XChaCha20Poly1305 failed with size %d: %v", size, err)
			}
			decrypted1, err := DecryptSymmetricXChaCha20Poly1305(encrypted1, secretKey)
			if err != nil || !bytes.Equal(decrypted1, msg) {
				t.Errorf("XChaCha20Poly1305 round-trip failed with size %d", size)
			}

			encrypted2, err := EncryptSymmetricAESGCM(msg, secretKey)
			if err != nil {
				t.Errorf("AESGCM failed with size %d: %v", size, err)
			}
			decrypted2, err := DecryptSymmetricAESGCM(encrypted2, secretKey)
			if err != nil || !bytes.Equal(decrypted2, msg) {
				t.Errorf("AESGCM round-trip failed with size %d", size)
			}
		})
	}
}

func TestInputValidation(t *testing.T) {
	secretKey := new32()
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	var publicKey32 [32]byte
	copy(publicKey32[:], publicKey)
	message := []byte("test message")

	// Prepare some valid encrypted/signed messages for testing
	validEncryptedAESGCM, _ := EncryptSymmetricAESGCM(message, secretKey)
	validEncryptedXChaCha, _ := EncryptSymmetricXChaCha20Poly1305(message, secretKey)
	validSignedSymmetric, _ := SignSymmetric(message, secretKey)
	validSignedAsymmetric := append(ed25519.Sign(privateKey, message), message...)

	tests := []struct {
		name    string
		f       func() error
		wantErr bool
		errMsg  string
	}{
		// SignSymmetric validation
		{
			name: "SignSymmetric nil key",
			f: func() error {
				_, err := SignSymmetric(message, nil)
				return err
			},
			wantErr: true,
			errMsg:  "secret key is required",
		},
		{
			name: "SignSymmetric nil message",
			f: func() error {
				_, err := SignSymmetric(nil, secretKey)
				return err
			},
			wantErr: false,
		},

		// VerifyAndReadSymmetric validation
		{
			name: "VerifyAndReadSymmetric nil key",
			f: func() error {
				_, err := VerifyAndReadSymmetric(validSignedSymmetric, nil)
				return err
			},
			wantErr: true,
			errMsg:  "secret key is required",
		},
		{
			name: "VerifyAndReadSymmetric short message",
			f: func() error {
				_, err := VerifyAndReadSymmetric(make([]byte, auth.Size-1), secretKey)
				return err
			},
			wantErr: true,
			errMsg:  "invalid signature",
		},

		// VerifyAndReadAsymmetric validation
		{
			name: "VerifyAndReadAsymmetric nil key",
			f: func() error {
				_, err := VerifyAndReadAsymmetric(validSignedAsymmetric, nil)
				return err
			},
			wantErr: true,
			errMsg:  "public key is required",
		},
		{
			name: "VerifyAndReadAsymmetric short message",
			f: func() error {
				_, err := VerifyAndReadAsymmetric(make([]byte, ed25519.SignatureSize-1), &publicKey32)
				return err
			},
			wantErr: true,
			errMsg:  "message shorter than signature size",
		},

		// VerifyAndReadAsymmetricBase64 validation
		{
			name: "VerifyAndReadAsymmetricBase64 invalid base64 message",
			f: func() error {
				_, err := VerifyAndReadAsymmetricBase64("invalid-base64", bytesutil.ToBase64(publicKey))
				return err
			},
			wantErr: true,
		},
		{
			name: "VerifyAndReadAsymmetricBase64 invalid base64 key",
			f: func() error {
				_, err := VerifyAndReadAsymmetricBase64(bytesutil.ToBase64(validSignedAsymmetric), "invalid-base64")
				return err
			},
			wantErr: true,
		},
		{
			name: "VerifyAndReadAsymmetricBase64 wrong key size",
			f: func() error {
				wrongSizeKey := make([]byte, 31) // Not 32 bytes
				_, err := VerifyAndReadAsymmetricBase64(
					bytesutil.ToBase64(validSignedAsymmetric),
					bytesutil.ToBase64(wrongSizeKey),
				)
				return err
			},
			wantErr: true,
			errMsg:  "invalid public key size",
		},

		// EncryptSymmetricXChaCha20Poly1305 validation
		{
			name: "EncryptSymmetricXChaCha20Poly1305 nil key",
			f: func() error {
				_, err := EncryptSymmetricXChaCha20Poly1305(message, nil)
				return err
			},
			wantErr: true,
			errMsg:  ErrSecretKeyIsNil.Error(),
		},
		{
			name: "EncryptSymmetricXChaCha20Poly1305 nil message",
			f: func() error {
				_, err := EncryptSymmetricXChaCha20Poly1305(nil, secretKey)
				return err
			},
			wantErr: false,
		},

		// DecryptSymmetricXChaCha20Poly1305 validation
		{
			name: "DecryptSymmetricXChaCha20Poly1305 nil key",
			f: func() error {
				_, err := DecryptSymmetricXChaCha20Poly1305(validEncryptedXChaCha, nil)
				return err
			},
			wantErr: true,
			errMsg:  ErrSecretKeyIsNil.Error(),
		},
		{
			name: "DecryptSymmetricXChaCha20Poly1305 short message (< nonce)",
			f: func() error {
				_, err := DecryptSymmetricXChaCha20Poly1305(make([]byte, xChaCha20Poly1305NonceSize-1), secretKey)
				return err
			},
			wantErr: true,
			errMsg:  ErrCipherTextTooShort.Error(),
		},
		{
			name: "DecryptSymmetricXChaCha20Poly1305 short message (no tag)",
			f: func() error {
				_, err := DecryptSymmetricXChaCha20Poly1305(make([]byte, xChaCha20Poly1305NonceSize+15), secretKey)
				return err
			},
			wantErr: true,
			errMsg:  ErrCipherTextTooShort.Error(),
		},

		// EncryptSymmetricAESGCM validation
		{
			name: "EncryptSymmetricAESGCM nil key",
			f: func() error {
				_, err := EncryptSymmetricAESGCM(message, nil)
				return err
			},
			wantErr: true,
			errMsg:  ErrSecretKeyIsNil.Error(),
		},
		{
			name: "EncryptSymmetricAESGCM nil message",
			f: func() error {
				_, err := EncryptSymmetricAESGCM(nil, secretKey)
				return err
			},
			wantErr: false,
		},

		// DecryptSymmetricAESGCM validation
		{
			name: "DecryptSymmetricAESGCM nil key",
			f: func() error {
				_, err := DecryptSymmetricAESGCM(validEncryptedAESGCM, nil)
				return err
			},
			wantErr: true,
			errMsg:  ErrSecretKeyIsNil.Error(),
		},
		{
			name: "DecryptSymmetricAESGCM short message (< nonce)",
			f: func() error {
				_, err := DecryptSymmetricAESGCM(make([]byte, aesNonceSize-1), secretKey)
				return err
			},
			wantErr: true,
			errMsg:  ErrCipherTextTooShort.Error(),
		},
		{
			name: "DecryptSymmetricAESGCM short message (no tag)",
			f: func() error {
				_, err := DecryptSymmetricAESGCM(make([]byte, aesNonceSize+15), secretKey)
				return err
			},
			wantErr: true,
			errMsg:  ErrCipherTextTooShort.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.f()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSha256Hash(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string // hex encoded expected hash
	}{
		{
			name:     "empty input",
			input:    []byte{},
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "hello world",
			input:    []byte("hello world"),
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name:     "single byte",
			input:    []byte{0x00},
			expected: "6e340b9cffb37a989ca544e6bb780a2c78901d3fb33738768511a30617afa01d",
		},
		{
			name:     "long message",
			input:    []byte("The quick brown fox jumps over the lazy dog"),
			expected: "d7a8fbb307d7809469ca9abcb0082e4f8d5651e46d3cdb762d02d0bf37c9e592",
		},
		{
			name:     "binary data",
			input:    []byte{0xFF, 0x00, 0xFF, 0x00, 0xFF},
			expected: "2f42a71a80110f9f3382ddf5af06d1aad32abfc64ceaafde005fc45443df1b42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sha256Hash(tt.input)
			resultHex := hex.EncodeToString(result)
			if resultHex != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, resultHex)
			}
			// Verify length is always 32 bytes
			if len(result) != sha256.Size {
				t.Errorf("expected hash length %d, got %d", sha256.Size, len(result))
			}
		})
	}
}

func TestHmacSha256(t *testing.T) {
	tests := []struct {
		name        string
		message     []byte
		key         []byte
		expectError bool
	}{
		{
			name:    "standard test",
			message: []byte("test message"),
			key:     []byte("secret key"),
		},
		{
			name:    "empty message",
			message: []byte{},
			key:     []byte("secret key"),
		},
		{
			name:        "empty key",
			message:     []byte("test message"),
			key:         []byte{},
			expectError: true,
		},
		{
			name:    "long key",
			message: []byte("test"),
			key:     make([]byte, 100), // key longer than block size
		},
		{
			name:        "nil key",
			message:     []byte("test"),
			key:         nil,
			expectError: true,
		},
		{
			name:    "nil message",
			message: nil,
			key:     []byte("key"),
		},
		{
			name:    "32 byte key",
			message: []byte("test message"),
			key:     make([]byte, 32),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := HmacSha256(tt.message, tt.key)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// HMAC-SHA256 should always be 32 bytes
			if len(result) != sha256.Size {
				t.Errorf("expected HMAC length %d, got %d", sha256.Size, len(result))
			}

			// Verify that the same input produces the same output
			result2, err := HmacSha256(tt.message, tt.key)
			if err != nil {
				t.Fatalf("unexpected error on second call: %v", err)
			}
			if !bytes.Equal(result, result2) {
				t.Error("HMAC should be deterministic")
			}

			// Verify that different keys produce different outputs
			if len(tt.key) > 0 {
				differentKey := make([]byte, len(tt.key))
				copy(differentKey, tt.key)
				differentKey[0] ^= 0xFF
				result3, _ := HmacSha256(tt.message, differentKey)
				if bytes.Equal(result, result3) {
					t.Error("different keys should produce different HMACs")
				}
			}
		})
	}
}

func TestValidateHmacSha256(t *testing.T) {
	message := []byte("test message")
	correctKey := []byte("correct key")
	wrongKey := []byte("wrong key")

	// Generate correct HMAC
	correctMAC, _ := HmacSha256(message, correctKey)

	tests := []struct {
		name        string
		message     []byte
		key         []byte
		knownMAC    []byte
		expectValid bool
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid HMAC",
			message:     message,
			key:         correctKey,
			knownMAC:    correctMAC,
			expectValid: true,
		},
		{
			name:        "invalid HMAC - wrong key",
			message:     message,
			key:         wrongKey,
			knownMAC:    correctMAC,
			expectValid: false,
		},
		{
			name:        "invalid HMAC - tampered MAC",
			message:     message,
			key:         correctKey,
			knownMAC:    append([]byte{0xFF}, correctMAC[1:]...),
			expectValid: false,
		},
		{
			name:        "invalid HMAC - different message",
			message:     []byte("different message"),
			key:         correctKey,
			knownMAC:    correctMAC,
			expectValid: false,
		},
		{
			name:        "nil key",
			message:     message,
			key:         nil,
			knownMAC:    correctMAC,
			expectError: true,
			errorMsg:    "attemptedKey is nil",
		},
		{
			name:        "wrong MAC length",
			message:     message,
			key:         correctKey,
			knownMAC:    []byte("too short"),
			expectError: true,
			errorMsg:    "knownGoodMAC must be 32 bytes",
		},
		{
			name:        "empty message",
			message:     []byte{},
			key:         correctKey,
			knownMAC:    mustHmacSha256([]byte{}, correctKey),
			expectValid: true,
		},
		{
			name:        "nil message",
			message:     nil,
			key:         correctKey,
			knownMAC:    mustHmacSha256(nil, correctKey),
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := ValidateHmacSha256(tt.message, tt.key, tt.knownMAC)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("expected error %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if valid != tt.expectValid {
				t.Errorf("expected valid=%v, got %v", tt.expectValid, valid)
			}
		})
	}
}

func TestHkdfSha256(t *testing.T) {
	secretKey := new32()

	tests := []struct {
		name        string
		secretKey   Key32
		salt        []byte
		info        string
		expectError bool
		errorMsg    string
	}{
		{
			name:      "basic derivation",
			secretKey: secretKey,
			salt:      []byte("salt"),
			info:      "info",
		},
		{
			name:      "nil salt",
			secretKey: secretKey,
			salt:      nil,
			info:      "info",
		},
		{
			name:      "empty salt",
			secretKey: secretKey,
			salt:      []byte{},
			info:      "info",
		},
		{
			name:      "empty info",
			secretKey: secretKey,
			salt:      []byte("salt"),
			info:      "",
		},
		{
			name:      "nil salt and empty info",
			secretKey: secretKey,
			salt:      nil,
			info:      "",
		},
		{
			name:      "long salt",
			secretKey: secretKey,
			salt:      make([]byte, 100),
			info:      "info",
		},
		{
			name:      "long info",
			secretKey: secretKey,
			salt:      []byte("salt"),
			info:      "very long info string that contains a lot of context information",
		},
		{
			name:        "nil secret key",
			secretKey:   nil,
			salt:        []byte("salt"),
			info:        "info",
			expectError: true,
			errorMsg:    ErrSecretKeyIsNil.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			derivedKey, err := HkdfSha256(tt.secretKey, tt.salt, tt.info)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("expected error %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if derivedKey == nil {
				t.Fatal("expected non-nil derived key")
			}
			// Verify key is 32 bytes
			if len(*derivedKey) != KeySize {
				t.Errorf("expected key size %d, got %d", KeySize, len(*derivedKey))
			}

			// Verify deterministic - same inputs produce same output
			derivedKey2, err := HkdfSha256(tt.secretKey, tt.salt, tt.info)
			if err != nil {
				t.Fatalf("unexpected error on second derivation: %v", err)
			}
			if !bytes.Equal((*derivedKey)[:], (*derivedKey2)[:]) {
				t.Error("HKDF is not deterministic")
			}

			// Verify different inputs produce different outputs
			if len(tt.salt) > 0 {
				differentSalt := make([]byte, len(tt.salt))
				copy(differentSalt, tt.salt)
				differentSalt[0] ^= 0xFF
				derivedKey3, _ := HkdfSha256(tt.secretKey, differentSalt, tt.info)
				if bytes.Equal((*derivedKey)[:], (*derivedKey3)[:]) {
					t.Error("different salt should produce different key")
				}
			}

			if tt.info != "" {
				derivedKey4, _ := HkdfSha256(tt.secretKey, tt.salt, tt.info+"different")
				if bytes.Equal((*derivedKey)[:], (*derivedKey4)[:]) {
					t.Error("different info should produce different key")
				}
			}
		})
	}
}

func TestToKey32(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectError bool
		errorMsg    string
	}{
		{
			name:  "valid 32 byte slice",
			input: make([]byte, 32),
		},
		{
			name:  "32 bytes with data",
			input: bytes.Repeat([]byte{0xAB}, 32),
		},
		{
			name:        "too short",
			input:       make([]byte, 31),
			expectError: true,
			errorMsg:    "byte slice must be exactly 32 bytes",
		},
		{
			name:        "too long",
			input:       make([]byte, 33),
			expectError: true,
			errorMsg:    "byte slice must be exactly 32 bytes",
		},
		{
			name:        "empty slice",
			input:       []byte{},
			expectError: true,
			errorMsg:    "byte slice must be exactly 32 bytes",
		},
		{
			name:        "nil slice",
			input:       nil,
			expectError: true,
			errorMsg:    "byte slice must be exactly 32 bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := ToKey32(tt.input)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("expected error %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if key == nil {
				t.Fatal("expected non-nil key")
			}
			// Verify the content matches
			if !bytes.Equal((*key)[:], tt.input) {
				t.Error("key content doesn't match input")
			}
			// Verify it's a proper array (not sharing underlying memory)
			if len(tt.input) > 0 {
				original := tt.input[0]
				tt.input[0] = ^tt.input[0]
				if (*key)[0] != original {
					t.Error("key should not share underlying array with input")
				}
				tt.input[0] = original // restore
			}
		})
	}
}

func TestFromKey32(t *testing.T) {
	validKey := new32()
	validKey[0] = 0xFF
	validKey[31] = 0xAA

	tests := []struct {
		name        string
		key         Key32
		expected    []byte
		expectError bool
		errorMsg    string
	}{
		{
			name:     "valid key",
			key:      validKey,
			expected: (*validKey)[:],
		},
		{
			name:     "new32 key",
			key:      new32(),
			expected: (*new32())[:],
		},
		{
			name:        "nil key",
			key:         nil,
			expectError: true,
			errorMsg:    "key is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FromKey32(tt.key)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("expected error %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !bytes.Equal(result, tt.expected) {
				t.Error("result doesn't match expected")
			}
			// Verify length
			if len(result) != KeySize {
				t.Errorf("expected length %d, got %d", KeySize, len(result))
			}
		})
	}
}

func TestKey32RoundTrip(t *testing.T) {
	// Test that ToKey32 and FromKey32 are inverse operations
	testData := [][]byte{
		make([]byte, 32),
		bytes.Repeat([]byte{0xFF}, 32),
		bytes.Repeat([]byte{0x55, 0xAA}, 16),
		{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
	}

	for i, data := range testData {
		t.Run(fmt.Sprintf("test-%d", i), func(t *testing.T) {
			// Convert to Key32
			key, err := ToKey32(data)
			if err != nil {
				t.Fatalf("ToKey32 failed: %v", err)
			}

			// Convert back to []byte
			result, err := FromKey32(key)
			if err != nil {
				t.Fatalf("FromKey32 failed: %v", err)
			}

			// Verify round trip
			if !bytes.Equal(result, data) {
				t.Error("round trip failed - data doesn't match")
			}
		})
	}
}

func TestHmacAndHkdfIntegration(t *testing.T) {
	// Test using HKDF-derived keys with HMAC
	masterKey := new32()
	salt := []byte("application salt")

	// Derive keys for different purposes
	authKey, err := HkdfSha256(masterKey, salt, "authentication")
	if err != nil {
		t.Fatalf("failed to derive auth key: %v", err)
	}

	encKey, err := HkdfSha256(masterKey, salt, "encryption")
	if err != nil {
		t.Fatalf("failed to derive enc key: %v", err)
	}

	// Verify derived keys are different
	if bytes.Equal((*authKey)[:], (*encKey)[:]) {
		t.Error("different info strings should produce different keys")
	}

	// Use derived key for HMAC
	message := []byte("important message")
	authKeyBytes, _ := FromKey32(authKey)
	mac, err := HmacSha256(message, authKeyBytes)
	if err != nil {
		t.Fatalf("failed to compute HMAC: %v", err)
	}

	// Validate HMAC with derived key
	valid, err := ValidateHmacSha256(message, authKeyBytes, mac)
	if err != nil {
		t.Fatalf("failed to validate HMAC: %v", err)
	}
	if !valid {
		t.Error("HMAC validation should succeed")
	}

	// Verify wrong key fails
	encKeyBytes, _ := FromKey32(encKey)
	valid, err = ValidateHmacSha256(message, encKeyBytes, mac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid {
		t.Error("HMAC validation should fail with wrong key")
	}
}

func TestHashAndHmacConsistency(t *testing.T) {
	// Test that our implementations are consistent with expected behavior
	message := []byte("consistency test message")

	// SHA256 should produce same hash for same input
	hash1 := Sha256Hash(message)
	hash2 := Sha256Hash(message)
	if !bytes.Equal(hash1, hash2) {
		t.Error("SHA256 should be deterministic")
	}

	// HMAC with same key should produce same MAC
	key := []byte("test key")
	mac1, _ := HmacSha256(message, key)
	mac2, _ := HmacSha256(message, key)
	if !bytes.Equal(mac1, mac2) {
		t.Error("HMAC should be deterministic")
	}

	// Different messages should produce different hashes
	differentMessage := append(message, byte('X'))
	hash3 := Sha256Hash(differentMessage)
	if bytes.Equal(hash1, hash3) {
		t.Error("different messages should produce different hashes")
	}

	// Different messages should produce different MACs
	mac3, _ := HmacSha256(differentMessage, key)
	if bytes.Equal(mac1, mac3) {
		t.Error("different messages should produce different MACs")
	}
}

// Helper function for tests
func mustHmacSha256(msg, key []byte) []byte {
	mac, err := HmacSha256(msg, key)
	if err != nil {
		panic(err)
	}
	return mac
}
