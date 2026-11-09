package constant

import "errors"

type StaticError error

var (
    RELATION_ALREADY_EXISTS    StaticError = errors.New("parent-student relation already exists")
)