package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
)

const (
	CLWNR_TOKEN_ENV = "CLWNR_TOKEN"
)

func getEnv(env string) (string, error) {
	value, ok := os.LookupEnv(env)
	if !ok {
		return "", fmt.Errorf("set environment variable %q", env)
	}
	return value, nil
}

func main() {
	verbose := false

	flag.BoolVar(&verbose, "v", false, "")
	flag.BoolVar(&verbose, "verbose", false, "print more logs")
	flag.Parse()

	if verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	token, err := getEnv(CLWNR_TOKEN_ENV)
	if err != nil {
		log.Fatal(err)
	}

	clowner, err := NewClowner(token)
	if err != nil {
		log.Fatal(err)
	}

	clowner.Run()
}
