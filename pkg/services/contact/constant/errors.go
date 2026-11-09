package constant

import "errors"

type StaticError error

var (
	RELATION_ALREADY_EXISTS StaticError = errors.New("parent-student relation already exists")
	ERR_ACCESS_DENIED       StaticError = errors.New("access denied")
	ERR_NOTE_NOT_FOUND      StaticError = errors.New("note not found")
)
