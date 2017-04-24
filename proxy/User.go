package proxy

import (
	"math/rand"
	"strconv"
)

type User struct {
	Username      string
	Password      string
	PassEncrypted bool
}

func (user *User) HasPassword() bool {
	return len(user.Password) > 0
}

func RandomUser() *User {
	return &User{
		Username:      "dummyUser",
		PassEncrypted: true,
		Password:      strconv.FormatInt(rand.Int63(), 3)}
}
