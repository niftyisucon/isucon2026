package isureal

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"strconv"
	"testing"
)

func referenceSignature(key []byte, id string, exp int64) []byte {
	value := []byte(strconv.FormatInt(exp, 10) + ":" + id)
	for range imageSignatureIterations {
		mac := hmac.New(sha256.New, key)
		mac.Write(value)
		value = mac.Sum(nil)
	}
	return value
}
func TestSignatureEquivalence(t *testing.T) {
	for _, size := range []int{0, 1, 32, 64, 100} {
		for _, id := range []string{"", "a", "01234567-89ab-cdef-0123-456789abcdef", "あ"} {
			for _, exp := range []int64{-1, 0, 1, 2147483647, 999999999999} {
				key := bytes.Repeat([]byte{0x92}, size)
				if !bytes.Equal(imageSignature(key, id, exp), referenceSignature(key, id, exp)) {
					t.Fatal(fmt.Sprintf("mismatch size=%d id=%q exp=%d", size, id, exp))
				}
			}
		}
	}
}
func BenchmarkSignatureOriginal(b *testing.B) {
	key := bytes.Repeat([]byte{2}, 32)
	b.ReportAllocs()
	for range b.N {
		referenceSignature(key, "01234567-89ab-cdef-0123-456789abcdef", 1234567890)
	}
}
func BenchmarkSignatureOptimized(b *testing.B) {
	key := bytes.Repeat([]byte{2}, 32)
	b.ReportAllocs()
	for range b.N {
		imageSignature(key, "01234567-89ab-cdef-0123-456789abcdef", 1234567890)
	}
}
