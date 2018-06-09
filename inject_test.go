package di

import (
	"errors"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestDI(t *testing.T) {
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

	var d Injector
	d.UseRunner(AsyncRunner())
	err := d.Provide(
		expected.Grades,
		OptNamed("Age", expected.Age),
		func() (s struct {
			skip string
			Skip string `dep:"-"`
		}) {
			return
		},
		func() *vars { return expected.Vars },
		func() (*log.Logger, error) { return expected.Logger, nil },
		func(logger *log.Logger) { /* do stuff */ },
		func() (f struct{ First string }, l struct{ Last string }) {
			f.First = expected.First
			l.Last = expected.Last
			return
		},
		func(arg struct {
			First string
			Last  string
		}) (res struct{ Name string }, err error) {
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

	var (
		got    vars
		logger *log.Logger
	)
	d.Inject(OptDecompose(&got), &logger)
	if !reflect.DeepEqual(expected, got) || logger != expected.Logger {
		t.Fatal()
	}
}

func TestError(t *testing.T) {
	var d Injector
	err := d.Provide(func() int {
		return 0
	}, func(v uint) uint8 {
		return uint8(v)
	}, int(1))
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatal(err)
	}

	d = Injector{}
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

	d = Injector{}
	err = d.Provide(func() (error, error) {
		return errors.New(""), nil
	})
	if err == nil || !strings.Contains(err.Error(), "more than one error") {
		t.Fatal(err)
	}

	d = Injector{}
	d.Provide(func(l *log.Logger) *log.Logger {
		return l
	})
	err = d.Run()
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatal(err)
	}

	d = Injector{}
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
	d = Injector{}
	if d.Inject(&n) == nil || d.Inject(n) == nil {
		t.Fatal()
	}
}

func TestAncestor(t *testing.T) {
	var d Injector
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
	var d = New()
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
	err := d.Provide(OptDecompose(expected))
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

	d = New()
	d.Provide(expected)
	got = Vars{}
	err = d.Inject(&got.A, &got.B, &got.C)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatal()
	}
}

type methodProvider struct{}

func (methodProvider) ProvideArr() []int {
	return []int{1, 2, 3}
}

func (methodProvider) ProvideSum(arr []int) int {
	var sum int
	for _, n := range arr {
		sum += n
	}
	return sum
}

func TestMethods(t *testing.T) {
	d := New()
	err := d.Provide(OptMethods(methodProvider{}, "Provide.*"))
	if err != nil {
		t.Fatal(err)
	}
	err = d.Run()
	if err != nil {
		t.Fatal(err)
	}
	var sum int
	err = d.Inject(&sum)
	if err != nil {
		t.Fatal(err)
	}
	if sum != 6 {
		t.Fatal()
	}
}

func TestNamed(t *testing.T) {
	d := New()
	d.Provide(
		d,
		OptNamed("I1", 2),
		OptNamed("I2", 2),
		OptNamed("I3", 2),
		func(d *Injector) float64 {
			d.Provide(uint8(2))
			return 2
		},
	)
	d.Run()

	var (
		i1, i2, i3 int
		f          float64
		u8         uint8
	)
	d.Inject(
		OptNamed("I1", &i1),
		OptNamed("I2", &i2),
		OptNamed("I3", &i3),
		&f,
		&u8,
	)
	if i1 != 2 || i2 != 2 || i3 != 2 || f != 2 || u8 != 2 {
		t.Fatal()
	}
}

func TestFuncObj(t *testing.T) {
	type Fn func() int
	inj := New()
	err := inj.Provide(OptFuncObj(Fn(func() int { return 1 })))
	if err != nil {
		t.Fatal(err)
	}
	err = inj.Run()
	if err != nil {
		t.Fatal(err)
	}

	var f Fn
	err = inj.Inject(&f)
	if err != nil {
		t.Fatal(err)
	}
	if f() != 1 {
		t.Fatal()
	}
}
