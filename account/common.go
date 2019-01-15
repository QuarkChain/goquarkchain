package account

import (
	"crypto/aes"
	"crypto/cipher"
	"io/ioutil"
	"os"
	"path/filepath"
)

//Uint32ToBytes trans uint32 num to bytes
func Uint32ToBytes(n uint32) []byte {
	return []byte{
		byte(n >> 24),
		byte(n >> 16),
		byte(n >> 8),
		byte(n),
	}
}

//IsP2 is check num is 2^x
func IsP2(shardSize ShardKeyType) bool {
	return (shardSize & (shardSize - 1)) == 0
}

//IntLeftMostBit left most bit
func IntLeftMostBit(v ShardKeyType) ShardKeyType {
	b := 0
	for v != 0 {
		v /= 2
		b++
	}
	return ShardKeyType(b)
}

func writeTemporaryKeyFile(file string, content []byte) (string, error) {
	const dirPerm = 0700
	if err := os.MkdirAll(filepath.Dir(file), dirPerm); err != nil {
		return "", err
	}

	f, err := ioutil.TempFile(filepath.Dir(file), "."+filepath.Base(file)+".tmp")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

func writeKeyFile(file string, content []byte) error {
	name, err := writeTemporaryKeyFile(file, content)
	if err != nil {
		return err
	}
	return os.Rename(name, file)
}

func ensureInt(x interface{}) int {
	res, ok := x.(int)
	if !ok {
		res = int(x.(float64))
	}
	return res
}

func aesCTRXOR(key, inText, iv []byte) ([]byte, error) {
	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	stream := cipher.NewCTR(aesBlock, iv)
	outText := make([]byte, len(inText))
	stream.XORKeyStream(outText, inText)
	return outText, err
}
