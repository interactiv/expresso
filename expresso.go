// Copyright 2015 <mparaiso@online.fr>
// License MIT

package expresso


import (
	"net/http"
)

type handlerFunc interface{}

type Route{
    Method string
    Pattern string
    HandlerFunc interface{}
    Params []string
}

func App()*Expresso{
    return &Expresso{}
}

type Expresso struct{
    routes []Route
}

func (e *Expresso) ServeHTTP(responseWriter http.ResponseWriter,request *http.Request) {
	responseWriter.WriteHeader(http.StatusOK)
}

func (e *Expresso) Get(route string,handlerFunc handlerFunc){
    route
}