# go-di
go-di is a library for [Go](https://golang.org) to do dependency injection. 

# Documentation
Documentation can be found at [Godoc](https://godoc.org/github.com/cosiner/go-di)

# Dependence
```Go
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
	err := d.Provide(
		expected.Grades,
		Named("Age", expected.Age),
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
		}) (res struct{ Name string }) {
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
	d.Inject(Decompose(&got), &logger)
	if !reflect.DeepEqual(expected, got) || logger != expected.Logger {
		t.Fatal()
	}
}
```

# LICENSE
MIT.
