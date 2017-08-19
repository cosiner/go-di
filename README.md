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

func ExampleInjector() {
	inj := New()

	err := inj.Provide(
		[]int{1, 2, 3},
		func(v []int) int {
			var sum int
			for _, n := range v {
				sum += n
			}
			return sum
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	err = inj.Run()
	if err != nil {
		log.Fatal(err)
	}
	var sum int
	err = inj.Inject(&sum)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(sum)
	// Output: 6
}

func Example() {
	inj := New()

	type User struct {
		FirstName string
		LastName  string
	}
	type NameFn func(string, string) string
	u := User{
		"F", "L",
	}
	err := inj.Provide(
		OptDecompose(u),
		OptFuncObj(NameFn(func(f, l string) string {
			return f + " " + l
		})),
	)
	if err != nil {
		log.Fatal(err)
	}
	var (
		first, last string
		nameFn      NameFn
	)
	err = inj.Inject(
		OptNamed("FirstName", &first),
		OptNamed("LastName", &last),
		&nameFn,
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(nameFn(first, last))
	//Output: F L
}
```

# LICENSE
MIT.
