package handler

import "github.com/rs/zerolog/log"

var rf = func(r interface{}) {}

func RunWithRecover(f func()) {
	if rf == nil {
		rf = func(r interface{}) {
			log.Fatal().Msgf("panic occurred: %v", r)
		}
	}

	defer func() {
		if r := recover(); r != nil {
			rf(r)
		}
	}()

	f()
}

func SetRecoverFunc(recoverFunc func(r interface{})) {
	rf = recoverFunc
}
