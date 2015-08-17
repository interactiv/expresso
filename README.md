#Monorail

[![Build Status](https://travis-ci.org/interactiv/monorail.svg?branch=master)](https://travis-ci.org/interactiv/monorail) [![Circle CI](https://circleci.com/gh/interactiv/monorail.svg?style=svg)](https://circleci.com/gh/interactiv/monorail) [![GoDoc](https://godoc.org/github.com/interactiv/monorail?status.svg)](https://godoc.org/github.com/interactiv/monorail) [![Coverage](http://gocover.io/_badge/github.com/interactiv/monorail?0)](http://gocover.io/github.com/interactiv/monorail)

awesome nano webframework for Go
	
Author:  mparaiso <mparaiso@online.fr>

Year: 2015

License: GPL-3.0

version: 0.4

###Quick start:

	
	package main
	
	import "github.com/interactiv/monorail"
	import "net/http"
	import "log"
	import "time"
	
	func main(){
		
		addr:=":3000"
		
		app:=monorail.New()
		
		// creates a middleware that will be called on each request matching /(.*) path
		app.Use("/",func(next monorail.Next){
			t0:=time.Now()
			next()
			log.Println("lapse: ",time.Now().Sub(t0))
		})
		
		// creates a route handler with a route variable
		app.Get("/greet/:name",func(ctx *monorail.Context,rw http.ReponseWriter){
			rw.Write([]byte("Hello "+ctx.RequestVars["name"]))
		})
		
		log.Println("Listening on ",addr)
		log.Fatal(http.ListenAndServe(addr,app))
		
	}
