package data

import (
	"errors"
	"strconv"
	"strings"
)

var ErrInvalidRuntimeFormat = errors.New("invalid runtime format")

type Runtime int32

// format: <runtime> mins
func (r *Runtime) UnmarshalJSON(jsonvalue []byte) error {
	// remove the quote first
	unquotedJSONValue, err := strconv.Unquote(string(jsonvalue))
	if err != nil {
		return ErrInvalidRuntimeFormat
	}

	parts := strings.Split(unquotedJSONValue, " ")
	if len(parts) != 2 || (parts[1] != "mins" && parts[1] != "min") {
		return ErrInvalidRuntimeFormat
	}

	i, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return ErrInvalidRuntimeFormat
	}

	*r = Runtime(i)
	return nil
}
