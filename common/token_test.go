package common

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestTokenCharDecode(t *testing.T) {
	ans := []int{1, 2, 3, 4, 5}

	if len(ans) < 5 {
		fmt.Println("<5", ans)
	}
	fmt.Println("??", ans[0:])
}

func TestTokenCharEncode(t *testing.T) {
	EmptyString := []byte{0x80}
	fmt.Println(TOKENIDMAX)
	fmt.Println(len(EmptyString), hex.EncodeToString(EmptyString))
}

func TestTokenIdDecode(t *testing.T) {

}
func TestTokenIDEncode(t *testing.T) {

}
