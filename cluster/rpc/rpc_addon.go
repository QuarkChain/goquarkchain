package rpc

import "errors"

func (r *Response) ErrMsg() error {
	return errors.New(string(r.ErrorMessage))
}
