# goapp
goapp is a library for [Go](https://golang.org) help run application. 

# Documentation
Documentation can be found at [Godoc](https://godoc.org/github.com/cosiner/goapp)

# Dependence
```Go

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
```

# LICENSE
MIT.
