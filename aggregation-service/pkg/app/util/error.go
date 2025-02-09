package util

import "fmt"

func WrapErr(message string, err error) error {
	return fmt.Errorf("%s; %w", message, err)
}
