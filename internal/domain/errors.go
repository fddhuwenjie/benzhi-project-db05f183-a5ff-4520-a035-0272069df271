package domain

import "fmt"

type RuleError struct {
	Code    string
	Message string
}

func (e *RuleError) Error() string { return e.Message }

func rule(code, format string, args ...any) error {
	return &RuleError{Code: code, Message: fmt.Sprintf(format, args...)}
}

func ErrorCode(err error) string {
	if value, ok := err.(*RuleError); ok {
		return value.Code
	}
	return "internal_error"
}
