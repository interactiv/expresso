#Expresso

[![Build Status](https://travis-ci.org/interactiv/expresso.svg?branch=master)](https://travis-ci.org/interactiv/expresso) [![Circle CI](https://circleci.com/gh/interactiv/expresso.svg?style=svg)](https://circleci.com/gh/interactiv/expresso) [![GoDoc](https://godoc.org/github.com/interactiv/expresso?status.svg)](https://godoc.org/github.com/interactiv/expresso) [![Coverage](http://gocover.io/_badge/github.com/interactiv/expresso?0)](http://gocover.io/github.com/interactiv/expresso)

The most awesome nano webframework for Go
	
Author:  mparaiso <mparaiso@online.fr>

Year: 2015

License: MIT

version: 0.3

###Quick start:

	
	package main
	
	import "github.com/interactiv/expresso"
	import "net/http"
	import "log"
	import "time"
	
	func main(){
		
		addr:=":3000"
		
		app:=expresso.New()
		
		// creates a middleware that will be called on each request matching /(.*) path
		app.Use("/",func(next expresso.Next){
			t0:=time.Now()
			next()
			log.Println("lapse: ",time.Now().Sub(t0))
		})
		
		// creates are route handler with a route variable
		app.Get("/greet/:name",func(ctx *expresso.Context,rw http.ReponseWriter){
			rw.Write([]byte("Hello "+ctx.RequestVars["name"]))
		})
		
		log.Println("Listening on ",addr)
		log.Fatal(http.ListenAndServe(addr,app))
		
	}
