#Micro

[![Build Status](https://travis-ci.org/interactiv/micro.svg?branch=master)](https://travis-ci.org/interactiv/micro) [![Circle CI](https://circleci.com/gh/interactiv/micro.svg?style=svg)](https://circleci.com/gh/interactiv/micro) [![GoDoc](https://godoc.org/github.com/interactiv/micro?status.svg)](https://godoc.org/github.com/interactiv/micro) [![Coverage](http://gocover.io/_badge/github.com/interactiv/micro?0)](http://gocover.io/github.com/interactiv/micro)

micro is a web micro framework written in go. It aims at simplyfing writting web apps in go.
micro has no external dependencies other than Go standard library.
	
Author:  mparaiso <mparaiso@online.fr>

Year: 2015

License: GPL-3.0

version: 0.4

###Quick start:

	
	package main
	
	import "github.com/interactiv/micro"
	import "net/http"
	import "log"
	import "time"
	
	func main(){
		
		addr:=":3000"
		
		app:=micro.New()
		
		// creates a middleware that will be called on each request matching /(.*) path
		app.Use("/",func(next micro.Next){
			t0:=time.Now()
			next()
			log.Println("lapse: ",time.Now().Sub(t0))
		})
		
		// creates a route handler with a route variable
		app.Get("/greet/:name",func(ctx *micro.Context,rw http.ReponseWriter){
			rw.Write([]byte("Hello "+ctx.RequestVars["name"]))
		})
		
		log.Println("Listening on ",addr)
		log.Fatal(http.ListenAndServe(addr,app))
		
	}
