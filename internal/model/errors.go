package model

import "errors"

var (
	ErrExtractionFailed = errors.New("extraction failed")
	ErrLLMUnavailable   = errors.New("LLM unavailable")
	ErrConfigInvalid    = errors.New("invalid configuration")
	ErrFileNotFound     = errors.New("file not found")
	ErrInvalidInput     = errors.New("invalid input")
)
