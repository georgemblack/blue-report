package util

import (
	"regexp"
)

func Match(pattern string, input string) (bool, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return false, WrapErr("failed to compile regex", err)
	}

	return regex.MatchString(input), nil
}
