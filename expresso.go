// Copyright 2015 <mparaiso@online.fr>
// License MIT

package expresso

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strings"
)

var (
	// Pattern represents a route param regexp pattern
	Pattern = "(?:\\:)(\\w+)"
	// DefaultParamPattern represents the default pattern that a route param matches
	DefaultParamPattern = "(\\w+)"
)

/**********************************/
/*               APP              */
/**********************************/

// Expresso represents an expresso application
type Expresso struct {
	debug bool
	*RouteCollection
	booted   bool
	injector *Injector
}

// App creates an expresso application
func New() *Expresso {
	expresso := &Expresso{
		RouteCollection: &RouteCollection{Routes: []*Route{}},
		injector:        NewInjector(),
	}
	expresso.injector.Register(expresso)
	return expresso
}

// Boot boots the application
func (e *Expresso) Boot() {
	if !e.Booted() {
		e.RouteCollection.Freeze()
		e.booted = true
	}
}

// Booted returns true if the Boot function has been called
func (e Expresso) Booted() bool {
	return e.booted
}

// ServeHTTP boots expresso server and handles http requests
func (e *Expresso) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	var match *Route
	if !e.Booted() {
		e.Boot()
	}
	for _, route := range e.RouteCollection.Routes {
		if route.pattern.MatchString(request.URL.Path) && route.MethodMatch(request.Method) {
			match = route
			break
		}
	}
	if match == nil {
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	}

	context := NewContext(request)
	injector := NewInjector(request, responseWriter, context)
	injector.SetParent(e.Injector())
	defer func() { context = nil; injector = nil }()

	for i, matchedParam := range match.pattern.FindStringSubmatch(request.URL.Path)[1:] {
		context.Request.Params[match.params[i]] = matchedParam
	}
	// Apply request parameter converters
	for key, value := range context.Request.Params {
		if match.converters[key] != nil {
			converterInjector := NewInjector(value)
			converterInjector.SetParent(injector)
			res, err := converterInjector.Apply(match.converters[key])
			if err != nil {
				panic(err)
			} else if len(res) > 0 {
				context.Request.Params[key] = res[0]
			}
		}

	}
	_, err := injector.Apply(match.HandlerFunc())
	if err != nil {
		log.Println(err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
	}
}

func (e *Expresso) Injector() *Injector {
	return e.injector
}

/**********************************/
/*            CONTEXT             */
/**********************************/

// Context represents a request context in an expresso application
type Context struct {
	Request struct {
		*http.Request
		Params map[string]interface{}
	}
}

func NewContext(request *http.Request) *Context {
	ctx := &Context{}
	ctx.Request.Request = request
	ctx.Request.Params = map[string]interface{}{}
	return ctx
}

/**********************************/
/*             ROUTE              */
/**********************************/

//Route represents a route in the router
type Route struct {
	// methods handled by the route
	methods []string
	// pattern is the pattern with which the request will be matched against
	pattern *regexp.Regexp
	// path is the path as string
	path        string
	handlerFunc interface{}
	params      []string
	frozen      bool
	converters  map[string]interface{}
}

// NewRoute creates a new route with a path that handles all methods
func NewRoute(path string) *Route {
	return &Route{
		methods:    []string{"*"},
		params:     []string{},
		converters: map[string]interface{}{},
		path:       path,
	}
}

// Params return route variable names.
// For instance if a route has the following pattern:
//    /catalog/:category/:productId
// it will return []string{"category","productId"}
func (r *Route) Params() []string { return r.params }

// HandlerFunc returns the current route handler function
func (r *Route) HandlerFunc() interface{} {
	return r.handlerFunc
}

type handlerFunction interface{}

// SetHandlerFunc sets the route handler function
func (r *Route) SetHandlerFunc(handlerFunc handlerFunction) {
	if r.IsFrozen() {
		return
	}
	if !IsCallable(handlerFunc) {
		panic(fmt.Sprintf("%v must be callable", handlerFunc))
		return
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

// Freeze freezes a route , which will make it read only
func (r *Route) Freeze() {
	if r.IsFrozen() {
		return
	}
	// extract route variables
	routeVarsRegexp := regexp.MustCompile(Pattern)
	matches := routeVarsRegexp.FindAllStringSubmatch(r.path, -1)
	if matches != nil && len(matches) > 0 {
		for _, match := range matches {
			for _, subMatch := range match[1:] {
				r.params = append(r.params, subMatch)
			}
		}
	}
	r.pattern = regexp.MustCompile(routeVarsRegexp.ReplaceAllString(r.path, DefaultParamPattern))
	r.frozen = true
}

// IsFrozen return the frozen state of a route.
// A Frozen route cannot be modified.
func (r *Route) IsFrozen() bool {
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

type conversionFunction interface{}

func (r *Route) Convert(param string, converterFunc conversionFunction) *Route {
	if !r.IsFrozen() {
		if !IsCallable(converterFunc) {
			panic(fmt.Sprintf("%v is not callable", converterFunc))
		}
		r.converters[param] = converterFunc
	}
	return r
}

/**********************************/
/*   ROUTE COLLECTION             */
/**********************************/

// RouteCollection is a collection of routes
type RouteCollection struct {
	Routes []*Route
	frozen bool
}

// Freeze freezes a route collection
func (rc *RouteCollection) Freeze() {
	if rc.IsFrozen() == false {
		rc.frozen = true
		for _, route := range rc.Routes {
			route.Freeze()
		}
	}
}

// IsFrozen returns true if the route collection is frozen
func (rc RouteCollection) IsFrozen() bool {
	return rc.frozen
}

// Get creates a GET route
func (rc *RouteCollection) Get(path string, handlerFunc interface{}) *Route {
	route := rc.Match(path, handlerFunc)
	route.SetMethods([]string{"GET", "HEAD"})
	return route
}

// Post creates a POST route
func (rc *RouteCollection) Post(path string, handlerFunc interface{}) *Route {
	route := rc.Match(path, handlerFunc)
	route.SetMethods([]string{"POST"})
	return route
}

// Put creates a PUT route
func (rc *RouteCollection) Put(path string, handlerFunc interface{}) *Route {
	route := rc.Match(path, handlerFunc)
	route.SetMethods([]string{"PUT"})
	return route
}

// Delete creates a DELETE route
func (rc *RouteCollection) Delete(path string, handlerFunc interface{}) *Route {
	route := rc.Match(path, handlerFunc)
	route.SetMethods([]string{"DELETE"})
	return route
}

// Match creates a route that matches all methods
func (rc *RouteCollection) Match(path string, handlerFunc interface{}) *Route {
	route := NewRoute(path)
	route.SetHandlerFunc(handlerFunc)
	rc.Routes = append(rc.Routes, route)
	return route
}

/**********************************/
/*            INJECTOR            */
/**********************************/

type Injector struct {
	services map[reflect.Type]interface{}
	parent   *Injector
}

func NewInjector(service ...interface{}) *Injector {
	injector := &Injector{services: map[reflect.Type]interface{}{}}
	for _, service_ := range service {
		injector.Register(service_)
	}
	return injector
}

// Register registers a new service to the injector
func (i *Injector) Register(service interface{}) {
	i.services[reflect.ValueOf(service).Type()] = service
}

func (i *Injector) Get(type_ reflect.Type) (interface{}, error) {
	var (
		err     error
		service interface{}
	)
	for typeService, service := range i.services {
		if typeService == type_ {
			return service, nil
		} else if type_.Kind() == reflect.Interface && typeService.Implements(type_) {
			return service, nil
		} else if type_.Kind() == reflect.Ptr && type_.Elem().Kind() == reflect.Interface && typeService.Implements(type_.Elem()) {
			return service, nil
		}
	}
	if service == nil && i.parent != nil && i.parent != i {
		service, err = i.parent.Get(type_)
	}
	if service == nil {
		err = errors.New(fmt.Sprintf("service with type %v cannot be injected : not found", type_))
	}
	return service, err
}

func (injector *Injector) Apply(callable interface{}) ([]interface{}, error) {
	var err error
	if !IsCallable(callable) {
		return nil, errors.New(fmt.Sprintf("%v is not a function or a method", callable))
	}
	arguments := []reflect.Value{}
	callableValue := reflect.ValueOf(callable)
	for i := 0; i < callableValue.Type().NumIn(); i++ {
		argument, err := injector.Get(callableValue.Type().In(i))
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, reflect.ValueOf(argument))
	}
	results := callableValue.Call(arguments)

	out := []interface{}{}
	for _, result := range results {
		out = append(out, result.Interface())
	}
	return out, err
}

func (injector *Injector) SetParent(parent *Injector) {
	injector.parent = parent
}

func (injector Injector) Parent() *Injector {
	return injector.parent
}

/**********************************/
/*              UTILS             */
/**********************************/

// IsCallable returns true if the value can
// be called like a function or a method
func IsCallable(value interface{}) bool {
	return reflect.ValueOf(value).Kind() == reflect.Func
}

/**********************************/
/*             TYPEDEFS           */
/**********************************/

type Convertible string
