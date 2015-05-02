// Copyrights 2015 mparaiso <mparaiso@online.fr>
// License MIT

package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/interactiv/expresso"
)

//somewhere in the file the following types are declared :

type User struct {
	Name string
	Id   string
}

type Users struct{}

func (_ Users) GetById(id string) *User {
	users := map[string]*User{
		"100": &User{Name: "John", Id: "100"},
		"200": &User{Name: "Jane", Id: "200"},
	}
	return users[id]
}

func main() {
	var users Users
	/* creates a new expresso app*/
	app := expresso.New()

	/* creates a middleware that will be called with each request*/
	app.Use("/", func(next expresso.Next) {
		t0 := time.Now()
		next()
		t1 := time.Now()
		fmt.Println("lapse: ", t1.Sub(t0))
	})
	/*
	   creates a route with a request variable called name
	   handler's arguments are automatically injected, so the framework
	   is fully compatible with the default http.HandlerFunc type.
	*/
	app.Get("/greet/:name?", func(ctx *expresso.Context, rw http.ResponseWriter) {
		rw.Write([]byte("Hello " + ctx.RequestVars["name"].(string)))
	}).
		// Assert the name variable is made of alpha characters
		Assert("name", "[A-Z a-z]+")
	/*
	   create a new route collection
	*/
	adminRoutes := expresso.NewRouteCollection()

	adminRoutes.Use("/", func(rw http.ResponseWriter, r *http.Request, next expresso.Next) {
		if r.URL.Query().Get("password") != "secret" {
			http.Redirect(rw, r, "/", http.StatusForbidden)
			return
		}
		next()
	})

	/*
	   We use dependency injection to make our app aware of the users value
	   Everytime we will inject the Users type, it will resolve the users
	   value
	*/
	app.Injector().Register(users)

	// The Convert method converts a request variable with the help
	// of a converter function,allowing to use a custom type
	// directly in a request handler,arguments are injected
	// with the help of the Injector, the user request
	// variable is passed as a string
	adminRoutes.All("/:user", func(ctx *expresso.Context) {
		// sends a JSON response to the client
		ctx.WriteJSON(ctx.RequestVars["user"].(*User))
	}).Convert("user", func(user string, users Users) *User {
		return users.GetById(user)
	}).Assert("user", "\\d+")

	//register subroute to the main route with prefix /admin
	app.Mount("/admin", adminRoutes)

	//start the webserver on port 80
	http.ListenAndServe(":80", app)

}
