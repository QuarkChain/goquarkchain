package sm3

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto/sha3"
)

var testData = map[string]string{
	"abc": "66c7f0f462eeedd9d1f2d46bdc10e4e24167c4875cf2f7a2297da02b8f4ba8e0",
	"abcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcd": "debe9ff92275b8a138604889c18e5a4d6fdb70e5387e5765293dcba39c0c5732",
	"abcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdasdfasdfffffffffffffffffaseeeeeeeeeeeeeeeeeeeee grhyakjd;lkfjaiefj;laknds;lvkna;sijfqwienfa;ksndfaiewnf;laksdnf;laskfr;owqneflaksndf;lkaq;oewnfsaldknf;aier;qwnefladknsf;iwenf;lanf": "6272aeeb3ae774f2f8b93c60932054c8f3bace0aa187a3440050d504da600e7b"}

func TestPerformance(t *testing.T) {
	tm := time.Now()
	for i := 0; i < 100000; i++ {
		for src, _ := range testData {
			h := NewSM3Hash()
			h.Sum([]byte(src))
		}
	}
	fmt.Println(time.Now().Sub(tm).Seconds())
	tm = time.Now()
	for i := 0; i < 100000; i++ {
		for src, _ := range testData {
			a := sha3.New256()
			a.Sum([]byte(src))
		}
	}
	fmt.Println(time.Now().Sub(tm).Seconds())
}

func TestSum(t *testing.T) {
	for src, expected := range testData {
		testSum(t, src, expected)
	}
}

func TestSm3_Sum(t *testing.T) {
	for src, expected := range testData {
		testSm3Sum(t, src, expected)
	}
}

func testSum(t *testing.T, src string, expected string) {
	hash := Sum([]byte(src))
	hashHex := hex.EncodeToString(hash[:])
	if hashHex != expected {
		t.Errorf("result:%s , not equal expected %s\n", hashHex, expected)
		return
	}
}

func testSm3Sum(t *testing.T, src string, expected string) {
	d := NewSM3Hash()
	d.Write([]byte(src))
	hash := d.Sum(nil)
	hashHex := hex.EncodeToString(hash[:])
	if hashHex != expected {
		t.Errorf("result:%s , not equal expected %s\n", hashHex, expected)
		return
	}
}

func TestSm3_Write(t *testing.T) {
	src1 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	src2 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	src3 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	d := NewSM3Hash()
	d.Write(src1)
	d.Write(src2)
	d.Write(src3)
	digest1 := d.Sum(nil)
	fmt.Printf("1 : %s\n", hex.EncodeToString(digest1))

	d.Reset()
	d.Write(src1)
	d.Write(src2)
	d.Write(src3)
	digest2 := d.Sum(nil)
	fmt.Printf("2 : %s\n", hex.EncodeToString(digest2))

	if !bytes.Equal(digest1, digest2) {
		t.Error("")
		return
	}
}

func TestPerformance2(t *testing.T) {
	src1 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	src2 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	src3 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	tm := time.Now()
	for i := 0; i < 100000; i++ {
		d := NewSM3Hash()
		d.Write(src1)
		d.Write(src2)
		d.Write(src3)
		d.Sum(nil)

		d.Reset()
		d.Write(src1)
		d.Write(src2)
		d.Write(src3)
		d.Sum(nil)
	}
	fmt.Println(time.Now().Sub(tm).Seconds())
}

func TestPerformance3(t *testing.T) {
	src1 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	src2 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	src3 := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	tm := time.Now()
	for i := 0; i < 100000; i++ {
		d := sha3.New256()
		d.Write(src1)
		d.Write(src2)
		d.Write(src3)
		d.Sum(nil)

		d.Reset()
		d.Write(src1)
		d.Write(src2)
		d.Write(src3)
		d.Sum(nil)
	}
	fmt.Println(time.Now().Sub(tm).Seconds())
}

