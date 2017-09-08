package proxy

import (
	"math/rand"
	"strconv"
)

// User data required for authentication
type User struct {
	Username      string
	Password      string
	PassEncrypted bool
}

func (user *User) hasPassword() bool {
	return len(user.Password) > 0
}

func randomUser() *User {
	return &User{
		Username:      "dummyUser",
		PassEncrypted: true,
		Password:      strconv.FormatInt(rand.Int63(), 3)}
}
