package crypto

import (
	"testing"
)

func TestNewAESGCM(t *testing.T) {
	crypto, err := NewAESGCM("test-key")
	if err != nil {
		t.Fatalf("NewAESGCM failed: %v", err)
	}

	if crypto == nil {
		t.Error("Expected AESGCM instance to be created")
	}
}

func TestAESGCMEncryptDecrypt(t *testing.T) {
	crypto, err := NewAESGCM("test-key-0123456789abcdef")
	if err != nil {
		t.Fatalf("NewAESGCM failed: %v", err)
	}

	plaintext := "Hello, World!"

	ciphertext, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if ciphertext == plaintext {
		t.Error("Ciphertext should not equal plaintext")
	}

	decrypted, err := crypto.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Decrypted text = %s, expected %s", decrypted, plaintext)
	}
}

func TestAESGCMEncryptWithDifferentKeys(t *testing.T) {
	crypto1, _ := NewAESGCM("key-one-0123456789abcdef")
	crypto2, _ := NewAESGCM("key-two-0123456789abcdef")

	plaintext := "Hello, World!"

	ciphertext, err := crypto1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = crypto2.Decrypt(ciphertext)
	if err == nil {
		t.Error("Expected decrypt to fail with wrong key")
	}
}

func TestAESGCMEncryptEmptyString(t *testing.T) {
	crypto, _ := NewAESGCM("test-key-0123456789abcdef")

	plaintext := ""

	ciphertext, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := crypto.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Decrypted text = %s, expected %s", decrypted, plaintext)
	}
}

func TestAESGCMDecryptInvalidCiphertext(t *testing.T) {
	crypto, _ := NewAESGCM("test-key-0123456789abcdef")

	_, err := crypto.Decrypt("invalid-ciphertext")
	if err == nil {
		t.Error("Expected decrypt to fail with invalid ciphertext")
	}
}

func TestAESGCMEncryptWithNonce(t *testing.T) {
	crypto, _ := NewAESGCM("test-key-0123456789abcdef")

	plaintext := "Hello, World!"

	ciphertext1, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	ciphertext2, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if ciphertext1 == ciphertext2 {
		t.Error("Same plaintext should produce different ciphertexts due to nonce")
	}
}

func TestAESGCMEncryptDecryptLargeData(t *testing.T) {
	crypto, _ := NewAESGCM("test-key-0123456789abcdef")

	plaintext := ""
	for i := 0; i < 10000; i++ {
		plaintext += "x"
	}

	ciphertext, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := crypto.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Error("Decrypted data does not match original")
	}
}

func TestAESGCMEncryptDecryptUnicode(t *testing.T) {
	crypto, _ := NewAESGCM("test-key-0123456789abcdef")

	plaintext := "你好，世界！🎉🌍"

	ciphertext, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := crypto.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Decrypted text = %s, expected %s", decrypted, plaintext)
	}
}

func TestAESGCMEncryptDecryptJSON(t *testing.T) {
	crypto, _ := NewAESGCM("test-key-0123456789abcdef")

	plaintext := `{"name":"John","age":30,"active":true}`

	ciphertext, err := crypto.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := crypto.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Decrypted text = %s, expected %s", decrypted, plaintext)
	}
}

func TestAESGCMKeyHashing(t *testing.T) {
	shortKeyCrypto, _ := NewAESGCM("short")
	longKeyCrypto, _ := NewAESGCM("this-is-a-very-long-key-that-exceeds-32-bytes")

	plaintext := "test data"

	ciphertext1, _ := shortKeyCrypto.Encrypt(plaintext)
	ciphertext2, _ := longKeyCrypto.Encrypt(plaintext)

	decrypted1, _ := shortKeyCrypto.Decrypt(ciphertext1)
	decrypted2, _ := longKeyCrypto.Decrypt(ciphertext2)

	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Error("Both keys should work correctly after SHA-256 hashing")
	}
}
