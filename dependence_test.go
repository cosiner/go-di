package goapp

import (
	"errors"
	"log"
	"os"
	"reflect"
	"testing"
)

func TestDependence(t *testing.T) {
	type vars struct {
		Age    uint
		Name   string
		First  string
		Last   string
		Grades []int
		Logger *log.Logger

		Vars *vars
	}
	var expected vars
	expected.Age = 1
	expected.First = "first"
	expected.Last = "last"
	expected.Name = expected.First + " " + expected.Last
	expected.Grades = []int{1, 2, 3}
	expected.Logger = log.New(os.Stdout, "", log.LstdFlags)
	expected.Vars = &vars{}

	var d Dependencies
	err := d.Provide(
		expected.Grades,
		struct {
			Age uint
		}{expected.Age},
		func() (s struct {
			skip string
			Skip string `dep:"-"`
		}) {
			return
		},
		func() *vars { return expected.Vars },
		func() (*log.Logger, error) {
			return expected.Logger, nil
		},
		func(logger *log.Logger) {

		},
		func() (f struct{ First string }, l struct{ Last string }) {
			f.First = expected.First
			l.Last = expected.Last
			return
		},
		func(
			arg struct {
				First string
				Last  string
			},
		) (res struct{ Name string }) {
			res.Name = arg.First + " " + arg.Last
			return
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	err = d.Run()
	if err != nil {
		t.Fatal(err)
	}

	var got vars
	d.Inject(&got)
	if !reflect.DeepEqual(expected, got) {
		t.Fatal()
	}
	var logger *log.Logger
	d.Inject(&logger)
	if logger != expected.Logger {
		t.Fatal()
	}
}

func TestDependenceError(t *testing.T) {
	var d Dependencies
	d.Provide(func() int {
		return 0
	}, func(v uint) uint8 {
		return uint8(v)
	}, int(1))
	if d.Run() == nil {
		t.Fatal()
	}

	d = Dependencies{}
	d.Provide(func() (error, int) {
		return errors.New(""), 0
	})
	if d.Run() == nil {
		t.Fatal()
	}

	d = Dependencies{}
	err := d.Provide(func() (error, error) {
		return errors.New(""), nil
	})
	if err == nil {
		t.Fatal()
	}

	d = Dependencies{}
	err = d.Provide(func(l *log.Logger) *log.Logger {
		return l
	})
	if err == nil {
		t.Fatal()
	}

	d = Dependencies{}
	d.Provide(uint8(0), func(uint8) {}, func(int) uint { return 0 }, func(uint) int { return 0 })
	if d.Run() == nil {
		t.Fatal()
	}

	var n float64
	d = Dependencies{}
	if d.Inject(&n) == nil || d.Inject(n) == nil {
		t.Fatal()
	}
}
