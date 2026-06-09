package ghimg

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"errors"
	"fmt"
)

var cbcIV = [16]byte{
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
}

func deriveKey(password []byte) []byte {
	key, err := pbkdf2.Key(sha1.New, string(password), []byte("saltysalt"), 1003, 16)
	if err != nil {
		panic(err) // fixed params — cannot fail
	}
	return key
}

func decryptCookie(encrypted []byte, password []byte, dbVersion int) ([]byte, error) {
	if len(encrypted) < 4 {
		return nil, errors.New("encrypted value too short")
	}
	if encrypted[0] != 'v' || encrypted[1] != '1' || encrypted[2] != '0' {
		return nil, errors.New("unrecognized cookie encryption prefix")
	}
	ciphertext := encrypted[3:]
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length %d not a multiple of block size", len(ciphertext))
	}

	key := deriveKey(password)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, cbcIV[:]).CryptBlocks(plain, ciphertext)

	pad := int(plain[len(plain)-1])
	if pad < 1 || pad > 16 {
		return nil, fmt.Errorf("invalid PKCS7 padding byte %d", pad)
	}
	plain = plain[:len(plain)-pad]

	prefixLen := 0
	if dbVersion >= 24 {
		prefixLen = 32
	}
	if len(plain) < prefixLen {
		return nil, fmt.Errorf("decrypted value too short (%d) for prefix strip (%d)", len(plain), prefixLen)
	}
	return plain[prefixLen:], nil
}
