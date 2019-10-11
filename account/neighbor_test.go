package account

import (
	"fmt"
	"testing"
)

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}
func TestIsNeighbor(t *testing.T) {
	for index := 0; index <= 30; index++ {
		asd := make([]int, 0)
		for sb := -1; sb < index; sb++ {
			asd = append(asd, sb+1)
		}

		fmt.Println("+++++++++++++++++")
		spn := len(asd) / 3
		for index := 0; index < spn; index++ {
			fmt.Println(asd[index*3 : (index+1)*3])
		}
		if len(asd)%3 != 0 {
			fmt.Println(asd[spn*3:])
		}
		fmt.Println("+++++++++++++++=end")
	}

}
