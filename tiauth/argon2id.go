package tiauth

import (
	"fmt"

	"golang.org/x/crypto/argon2"
)

type argon2idStruct struct {
	time    uint32
	memory  uint32
	threads uint8
}

func newArgon2id(time uint32, memory uint32, threads uint8) *argon2idStruct {
	argon2id := &argon2idStruct{
		time:    time,
		memory:  memory,
		threads: threads,
	}

	return argon2id
}

func (argon2id *argon2idStruct) Id() string {
	return fmt.Sprintf("argon2id.%d.%d.%d.32", argon2id.time, argon2id.memory, argon2id.threads)
}

func (*argon2idStruct) SaltSize() int {
	return 16
}

func (argon2id *argon2idStruct) Hash(password string, salt []byte) ([]byte, error) {
	key := argon2.IDKey([]byte(password), salt, argon2id.time, argon2id.memory, argon2id.threads, 32)
	return key, nil
}
