# go-di
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/cosiner/go-di) 
[![Build Status](https://travis-ci.org/cosiner/go-di.svg?branch=master&style=flat)](https://travis-ci.org/cosiner/go-di)
[![Coverage Status](https://coveralls.io/repos/github/cosiner/go-di/badge.svg?style=flat)](https://coveralls.io/github/cosiner/go-di)
[![Go Report Card](https://goreportcard.com/badge/github.com/cosiner/go-di?style=flat)](https://goreportcard.com/report/github.com/cosiner/go-di)

go-di is a library for [Go](https://golang.org) to do dependency injection. 

# Documentation
Documentation can be found at [Godoc](https://godoc.org/github.com/cosiner/go-di)

# Example
```Go

package di_test

import (
	"fmt"
	"log"
	"net/http"

	di "github.com/cosiner/go-di"
)

type DBConfig struct{}
type DB struct{}
type UserHandler struct{}
type TopicHandler struct{}
type Router struct{}

type Providers struct{}

func (Providers) ProvideDBConfig() DBConfig {
	return DBConfig{}
}
func (Providers) ProvideDB(config DBConfig) (DB, error) {
	return DB{}, nil
}
func (Providers) ProvideHandlers(db DB) (UserHandler, TopicHandler, error) {
	return UserHandler{}, TopicHandler{}, nil
}

func (Providers) ProvideHttpHandler(args struct {
	User  UserHandler
	Topic TopicHandler
}) (res struct {
	Router  Router
	Handler http.Handler
}, err error) {
	res.Router = Router{}
	return res, nil
}

func ExampleInjector() {
	inj := di.New()

	err := inj.Provide(di.OptMethods(Providers{}, ""))
	if err != nil {
		log.Fatal(err)
	}
	err = inj.Run()
	if err != nil {
		log.Fatal(err)
	}
	var args struct {
		Handler http.Handler
	}
	err = inj.Inject(&args)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(args.Handler == nil)
	// Output: true
}
```

# LICENSE
MIT.
