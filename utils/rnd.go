package utils

import (
	"math/rand"
	"sync"
	"time"
)

var (
	customRand      = rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
	customRandMutex sync.Mutex
)

func RandInt() int {
	customRandMutex.Lock()
	defer customRandMutex.Unlock()
	return customRand.Int()
}

func RandInt64(max int64) int64 {
	customRandMutex.Lock()
	defer customRandMutex.Unlock()
	return customRand.Int63n(max)
}

func RandomAlphanumeric(length int) string {
	return RandomString(length, "qwertyuiopasdfghjklzxcvbnm1234567890")
}

func RandomNumeric(length int) string {
	return RandomString(length, "0123456789")
}

func RandomString(length int, chars string) string {
	if length <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < length; i++ {
		result += string(chars[RandInt()%len(chars)])
	}
	return result
}
