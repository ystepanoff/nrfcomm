package protocol

import "errors"

var (
	ErrInvalidPayload = errors.New("invalid payload size")
	ErrNotPaired      = errors.New("device not paired")
	ErrTimeout        = errors.New("operation timed out")
	ErrInvalidChannel = errors.New("invalid channel (valid range: 0-125)")
)
