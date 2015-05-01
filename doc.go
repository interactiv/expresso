// Copyrights 2015 mparaiso <mparaiso@online.fr>
// License MIT

// Expresso 
// 
// version 0.1
//
// The most awesome nano webframework for Go
//
// Quick start:
//
//    package main
//
//    import (
//    	"fmt"
//    	"net/http"
//    	"time"
//
//    	"github.com/interactiv/expresso"
//    )
//
//    func main() {
//    	/* creates a new expresso app*/
//    	app := expresso.New()
//
//    	/* creates a middleware that will be called with each request*/
//    	app.Use("/", func(next expresso.Next) {
//    		t0 := time.Now()
//    		next()
//    		t1 := time.Now()
//    		fmt.Print("lapse: ", t1.Sub(t0))
//    	})
//    	/*
//    	   creates a route with a request variable called name
//    	   handler's arguments are automatically injected, so the framework
//    	   is fully compatible with the default http.HandlerFunc type.
//    	*/
//    	app.Get("/:name", func(ctx *expresso.Context, rw http.ResponseWriter) {
//    		rw.Write([]byte("Hello" + ctx.RequestVars["name"]))
//    	}).
//    		// Assert the name variable is made of alpha characters
//    		Assert("name", "[A-Z a-z]+")
//    	/*
//    	   create a new route collection
//    	*/
//    	adminRoutes := expresso.NewRouteCollection()
//
//    	adminRoutes.Use("/", func(rw http.ResponseWriter, r *http.Request) {
//    		if r.URL.Query().Get("password") != "secret" {
//    			http.Redirect(rw, r, "/", http.StatusForbidden)
//    			return
//    		}
//    		next()
//    	})
//
//
//somewhere in the file the following types are declared :
//
//    type User struct{
//        Name string
//        Id string
//    }
//    type Users struct{}
//    func (_ Users)GetById(string id)*User{ ... }
//
//    var users Users
//
//    We use dependency injection to make our app aware of the users value
//    Everytime we will inject the Users type, it will resolve the users
//    value
//
//    app.Injector().Register(users)
//
// The Convert method converts a request variable  with the help of a
// converter function,allowing to use a custom type directly in a request
// handler,arguments are injected with the help of the Injector, the user
// request variable is passed as a string
//
//    adminRoutes.All("/:user", func(ctx *expresso.Context) {
//        ctx.WriteString("Hello admin"+ctx.RequestVars["user"].(User).Name)
//    }).Convert("user",func(user string,users Users)User{
//        return users.GetById(user)
//    })
//
// register subroute to the main route with prefix /admin
//
//    app.Mount("/admin", subRoute)
//
// start the webserver on port 80
//
//    http.ListenAndServe(":80", app)
//
//    }
//
package expresso
