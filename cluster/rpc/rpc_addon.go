package rpc

import "errors"

func (r *Response) GetError() error {
	return errors.New(string(r.ErrorMessage))
}
