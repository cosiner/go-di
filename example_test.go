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
func (Providers) ProvideDB() (DB, error) {
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
