package auth

import (
	"crypto/rand"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

const charset = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GenerateRandomPassword generates a random secure password of the given length.
func GenerateRandomPassword(length int) string {
	if length <= 0 {
		length = 10
	}
	bytes := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			bytes[i] = charset[i%len(charset)]
		} else {
			bytes[i] = charset[num.Int64()]
		}
	}
	return string(bytes)
}

// HashPassword hashes a plain password using bcrypt.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPasswordHash checks if a plain password matches the bcrypt hash.
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
