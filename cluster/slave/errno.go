package slave

import (
	"errors"
	"fmt"
)

func ErrMsg(str string) error {
	return errors.New(fmt.Sprintf("incorrect branch when call %s", str))
}
