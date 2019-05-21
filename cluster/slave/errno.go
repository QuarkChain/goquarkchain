package slave

import (
	"errors"
	"fmt"
)


var(
	NoSuchBranch =errors.New("no such branch")
)
func ErrMsg(str string,err error) error {
	return errors.New(fmt.Sprintf("incorrect branch when call %s:err %v", str,err))
}
