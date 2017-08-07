package goapp

import (
	"errors"
	"log"
	"os"
	"reflect"
	"strings"
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
	err := d.Provide(func() int {
		return 0
	}, func(v uint) uint8 {
		return uint8(v)
	}, int(1))
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatal(err)
	}

	d = Dependencies{}
	err = d.Provide(func() (error, int) {
		return errors.New("ERROR"), 0
	})
	if err != nil {
		t.Fatal(err)
	}
	err = d.Run()
	if err == nil || !strings.Contains(err.Error(), "ERROR") {
		t.Fatal(err)
	}

	d = Dependencies{}
	err = d.Provide(func() (error, error) {
		return errors.New(""), nil
	})
	if err == nil || !strings.Contains(err.Error(), "more than one error") {
		t.Fatal(err)
	}

	d = Dependencies{}
	d.Provide(func(l *log.Logger) *log.Logger {
		return l
	})
	err = d.Run()
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatal(err)
	}

	d = Dependencies{}
	d.Provide(
		uint8(0),
		func(uint8) {},
		func(int) uint { return 0 },
		func(uint) int { return 0 },
	)
	err = d.Run()
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatal(err)
	}

	var n float64
	d = Dependencies{}
	if d.Inject(&n) == nil || d.Inject(n) == nil {
		t.Fatal()
	}
}

func TestAncestor(t *testing.T) {
	var d Dependencies
	err := d.Provide(func() uint {
		return 1
	}, func(u uint, i int) float64 {
		return float64(u) + float64(i)
	}, func(n uint) int {
		return int(n + 1)
	})
	if err != nil {
		t.Fatal(err)
	}
	err = d.Run()
	if err != nil {
		t.Fatal(err)
	}
	var f float64
	err = d.Inject(&f)
	if err != nil {
		t.Fatal(err)
	}
	if f != 3 {
		t.Fatal()
	}
}

func TestDecompose(t *testing.T) {
	var d = NewDependencies()
	type Vars struct {
		A uint
		B int
		C float64
	}
	expected := Vars{
		A: 1,
		B: 2,
		C: 3,
	}
	err := d.Provide(d.Decompose(expected))
	if err != nil {
		t.Fatal(err)
	}
	err = d.Run()
	if err != nil {
		t.Fatal(err)
	}
	var got Vars
	err = d.Inject(&got.A, &got.B, &got.C)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatal()
	}

	d = NewDependencies()
	d.Provide(expected)
	got = Vars{}
	err = d.Inject(&got.A, &got.B, &got.C)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatal()
	}
}
