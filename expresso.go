// Copyright 2015 <mparaiso@online.fr>
// License MIT

package expresso

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
)

var (
	// Pattern represents a route param regexp pattern
	Pattern = "(?:\\:)(\\w+)"
	// DefaultPatternMatcher represents the default pattern that a route param matches
	DefaultParamPattern = "(\\w+)"
)

/**********************************/
/*             ROUTE              */
/**********************************/

//Route represents a route in the router
type Route struct {
	methods     []string
	pattern     *regexp.Regexp
	handlerFunc interface{}
	params      []string
	frozen      bool
	converters  map[string]interface{}
}

// Params return route variable names.
// For instance if a route has the following pattern:
//    /catalog/:category/:productId
// it will return []string{"category","productId"}
func (r *Route) Params() []string { return r.params }
func (r *Route) HandlerFunc() interface{} {
	return r.handlerFunc
}
func (r *Route) SetHandlerFunc(handlerFunc interface{}) {
	handlerValue := reflect.ValueOf(handlerFunc)
	if handlerValue.Kind() != reflect.Func {
		panic("handlerFunc must a function")
	}
	r.handlerFunc = handlerFunc
}
func (r Route) MethodMatch(method string) bool {
	match := false
	for _, m := range r.Methods() {
		if strings.TrimSpace(strings.ToUpper(method)) == m || m == "*" {
			match = true
			break
		}
	}
	return match
}
func (r *Route) Freeze() {
	if r.IsFrozen() == false {
		r.frozen = true
	}
}

// IsFrozen return the frozen state of a route.
// A Frozen route cannot be modified.
func (r Route) IsFrozen() bool {
	return r.frozen
}

// Methods gets methods handled by the route
func (r *Route) Methods() []string {
	return r.methods

}

// SetMethods sets the methods handled by the route.
//
// Example:
//
//    route.SetMethods([]string{"GET","POST"})
// []string{"*"} means the route handles all methods.
func (r *Route) SetMethods(methods []string) {
	if r.IsFrozen() == true {
		return
	}
	r.methods = methods
}

func (r *Route) Convert(param string, converterFunc interface{}) *Route {
	if r.IsFrozen() == false {
		if IsCallable(converterFunc) == false {
			panic(fmt.Sprintf("%v is not callable", converterFunc))
		}
		r.converters[param] = converterFunc
	}
	return r
}

/**********************************/
/*            CONTEXT             */
/**********************************/

// Context represents a request context in an expresso application
type Context struct {
	Params map[string]interface{}
}

/**********************************/
/*               APP              */
/**********************************/

// App creates an expresso application
func App() *Expresso {
	return &Expresso{}
}

type registry map[string]interface{}

type Expresso struct {
	debug  bool
	Routes []*Route
}

func (e *Expresso) SetDebug(debug bool) {
	e.debug = debug
}

func (e Expresso) Debug() bool {
	return e.debug
}

func (e *Expresso) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	var match *Route

	for _, route := range e.Routes {
		if route.pattern.MatchString(request.URL.Path) && route.MethodMatch(request.Method) {
			match = route
			break
		}
	}
	if match == nil {
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	}
	context := &Context{Params: map[string]interface{}{}}
	for i, matchedParam := range match.pattern.FindStringSubmatch(request.URL.Path)[1:] {
		context.Params[match.params[i]] = matchedParam
	}

	handlerValue := reflect.ValueOf(match.HandlerFunc())
	arguments := make([]reflect.Value, handlerValue.Type().NumIn())

	for i := 0; i < handlerValue.Type().NumIn(); i++ {

		switch handlerValue.Type().In(i) {
		case reflect.TypeOf(request):
			arguments[i] = reflect.ValueOf(request)
		case reflect.TypeOf(context):
			arguments[i] = reflect.ValueOf(context)
		default:
			// try to inject a ResponseWriter
			if handlerValue.Type().In(i).Implements(reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()) {
				arguments[i] = reflect.ValueOf(responseWriter)
			} else {
				panic(fmt.Sprintf("cannot find argument of type %+v to inject", handlerValue.Type().In(i).String()))
			}
		}
	}

	handlerValue.Call(arguments)
}

func (e *Expresso) Get(path string, handlerFunc interface{}) *Route {
	route := e.Match(path, handlerFunc)
	route.SetMethods([]string{"GET", "HEAD"})
	return route
}

func (e *Expresso) Post(path string, handlerFunc interface{}) *Route {
	route := e.Match(path, handlerFunc)
	route.SetMethods([]string{"POST"})
	return route
}

func (e *Expresso) Put(path string, handlerFunc interface{}) *Route {
	route := e.Match(path, handlerFunc)
	route.SetMethods([]string{"PUT"})
	return route
}

func (e *Expresso) Delete(path string, handlerFunc interface{}) *Route {
	route := e.Match(path, handlerFunc)
	route.SetMethods([]string{"DELETE"})
	return route
}
func (e *Expresso) Match(path string, handlerFunc interface{}) *Route {
	route := &Route{
		methods:    []string{"*"},
		params:     []string{},
		converters: map[string]interface{}{},
	}
	route.SetHandlerFunc(handlerFunc)
	// extract route variables
	routeVarsRegexp := regexp.MustCompile(Pattern)
	matches := routeVarsRegexp.FindAllStringSubmatch(path, -1)
	if matches != nil && len(matches) > 0 {
		for _, match := range matches {
			for _, subMatch := range match[1:] {
				route.params = append(route.params, subMatch)
			}
		}
	}
	route.pattern = regexp.MustCompile(routeVarsRegexp.ReplaceAllString(path, DefaultParamPattern))
	e.Routes = append(e.Routes, route)
	return route
}

/**********************************/
/*               UTIL             */
/**********************************/

func IsCallable(value interface{}) bool {
	return reflect.ValueOf(value).Kind() == reflect.Func
}
