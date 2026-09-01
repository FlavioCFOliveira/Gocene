package models

import "fmt"

// LukeException is used for internal errors when accessing the index in Luke.
type LukeException struct {
	Message string
	Err     error
}

func (e *LukeException) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func NewLukeException(message string, err error) error {
	return &LukeException{
		Message: message,
		Err:     err,
	}
}
