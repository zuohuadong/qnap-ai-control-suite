//go:build !linux

package devicedirs

import "errors"

func Ensure(root string, request Request) (Receipt, error) {
	return Receipt{}, errors.New("device directory operations require Linux")
}
