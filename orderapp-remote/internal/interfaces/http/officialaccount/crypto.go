package officialaccount

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
)

func signature(parts ...string) string {
	sort.Strings(parts)
	h := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(h[:])
}
func same(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }
func EncryptMessage(encodedKey, appID string, msg []byte) (string, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey + "=")
	if err != nil || len(key) != 32 {
		return "", errors.New("invalid encryption key")
	}
	data := make([]byte, 20)
	if _, err = rand.Read(data[:16]); err != nil {
		return "", err
	}
	binary.BigEndian.PutUint32(data[16:20], uint32(len(msg)))
	data = append(data, msg...)
	data = append(data, appID...)
	n := 32 - len(data)%32
	data = append(data, bytes.Repeat([]byte{byte(n)}, n)...)
	block, _ := aes.NewCipher(key)
	cipher.NewCBCEncrypter(block, key[:16]).CryptBlocks(data, data)
	return base64.StdEncoding.EncodeToString(data), nil
}
func DecryptMessage(encodedKey, appID, encoded string) ([]byte, error) {
	fail := errors.New("invalid encrypted message")
	key, err := base64.StdEncoding.DecodeString(encodedKey + "=")
	if err != nil || len(key) != 32 {
		return nil, fail
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) < 32 || len(data)%16 != 0 {
		return nil, fail
	}
	block, _ := aes.NewCipher(key)
	cipher.NewCBCDecrypter(block, key[:16]).CryptBlocks(data, data)
	n := int(data[len(data)-1])
	if n < 1 || n > 32 || n > len(data) {
		return nil, fail
	}
	for _, v := range data[len(data)-n:] {
		if int(v) != n {
			return nil, fail
		}
	}
	data = data[:len(data)-n]
	if len(data) < 20 {
		return nil, fail
	}
	size := int(binary.BigEndian.Uint32(data[16:20]))
	if size > len(data)-20 || !same(string(data[20+size:]), appID) {
		return nil, fail
	}
	return data[20 : 20+size], nil
}
