package users

import (
	"errors"
	"strings"
)

var ErrInvalidUser = errors.New("user id must be positive")

type User struct {
	ID   int
	Name string
}

func NewUser(id int, name string) (User, error) {
	name = normalizeName(name)
	if id <= 0 {
		return User{}, ErrInvalidUser
	}
	if name == "" {
		return User{}, errors.New("user name is required")
	}
	return User{ID: id, Name: name}, nil
}

func normalizeName(name string) string {
	return strings.TrimSpace(name)
}
