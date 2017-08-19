package di

import (
	"fmt"
	"log"
)

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

func ExampleOpt() {
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
