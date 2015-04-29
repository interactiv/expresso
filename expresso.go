// Copyright 2015 <mparaiso@online.fr>
// License MIT

package expresso

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"runtime/debug"
	"strings"
)

var (
	// Pattern represents a route param regexp pattern
	Pattern = "(?:\\:)(\\w+)|(\\(.+\\)?)"
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
	RequestMatcher *RequestMatcher
	booted         bool
	injector       *Injector
	errorHandlers  map[int]HandlerFunction
}

// New creates an expresso application
func New() *Expresso {
	expresso := &Expresso{
		RouteCollection: &RouteCollection{Routes: []*Route{}},
		injector:        NewInjector(),
		errorHandlers:   map[int]HandlerFunction{},
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

// ServeHTTP boots expresso server and handles http requests.
//
// Can Panic!
func (e *Expresso) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	var (
		matches                []*Route
		next                   func()
		context                *Context
		injector               *Injector
		responseWriterWithCode *ResponseWriterWithCode
		stack                  *StackWithInjector
	)
	defer func() {
		if err := recover(); err != nil {
			//log.Println(err)
			os.Stderr.WriteString(fmt.Sprint(err))
			injector.Register(err)
			injector.MustApply(e.errorHandlers[500])
		}
	}()
	// wrap responseWriter so we can access the status code
	responseWriterWithCode = &ResponseWriterWithCode{
		ResponseWriter: responseWriter,
	}
	// sets context and injector
	context = NewContext(request)
	injector = NewInjector(request, responseWriterWithCode, context)
	injector.Register(injector)
	injector.SetParent(e.Injector())
	if e.errorHandlers[500] == nil {
		e.Error(500, InternalServerErrorHandler)
	}
	if e.errorHandlers[404] == nil {
		e.Error(404, NotFoundErrorHandler)
	}
	if e.RequestMatcher == nil {
		e.RequestMatcher = NewRequestMatcher(e.RouteCollection)
	}
	if !e.Booted() {
		e.Boot()
	}
	// find all routes matching the request in the route collection
	matches = e.RequestMatcher.MatchAll(request)
	// no match, call 404
	if len(matches) == 0 {
		injector.MustApply(e.errorHandlers[404])
		return
	}
	// For the first matched route, call all its handlers
	// if an handler in a route calls expresso.Next next() , execute the next handler
	// When all handlers of a route have been called
	// if there are still some matched routes and the last handler of the previous route calls next
	// then repeat the process for the next matched route
	next = func() {
		if len(matches) == 0 {
			return
		}
		match := matches[0]
		matches = matches[1:]
		// If there are some request variables, populate the context with them
		for i, matchedParam := range match.pattern.FindStringSubmatch(request.URL.Path)[1:] {
			context.RequestVars[match.params[i]] = matchedParam
		}
		// Apply request parameter converters
		for key, value := range context.RequestVars {
			if match.converters[key] != nil {
				converterInjector := NewInjector(value)
				converterInjector.SetParent(injector)
				res := converterInjector.MustApply(match.converters[key])
				if len(res) > 0 {
					context.RequestVars[key] = res[0]
				}
			}
		}
		stack = NewStackWithInjector(injector, match.Handlers()...)
		stack.SetNext(next)
		stack.ServeHTTP(responseWriterWithCode, request)
	}
	next()
	//try to get status code from response,if error, try to execute
	// error handler
	code := responseWriterWithCode.Code
	if code > 399 && e.errorHandlers[code] != nil {
		injector.MustApply(e.errorHandlers[code])
	}
}

// Error sets an error handler given an error code.
// Arguments of that handler function are resolved by expresso's injector.
//
// Can Panic! if the error code is lower than 400.
func (e *Expresso) Error(errorCode int, handlerFunc HandlerFunction) {
	if e.Booted() {
		return
	}
	if errorCode < 400 {
		panic(fmt.Sprintf("errorCode should be greater or equal to 400, got %d", errorCode))
	}
	e.errorHandlers[errorCode] = handlerFunc
}

// Injector return the injector
func (e *Expresso) Injector() *Injector {
	return e.injector
}

/**********************************/
/*     DEFAULT ERROR HANDLERS     */
/**********************************/

// InternalServerErrorHandler executes the default 500 handler
func InternalServerErrorHandler(err error, rw http.ResponseWriter) {
	http.Error(rw, fmt.Sprintf("%v\r\n%s", err, debug.Stack()), http.StatusInternalServerError)
}

// NotFoundErrorHandler executes the default 404 handler
func NotFoundErrorHandler(rw http.ResponseWriter, r *http.Request) {
	http.NotFound(rw, r)
}

/**********************************/
/*            CONTEXT             */
/**********************************/

// Context represents a request context in an expresso application
type Context struct {
	Request struct {
		*http.Request
	}
	RequestVars map[string]interface{}
	Vars        map[string]interface{}
}

// NewContext returns a new Context
func NewContext(request *http.Request) *Context {
	ctx := &Context{
		RequestVars: map[string]interface{}{},
		Vars:        map[string]interface{}{},
	}
	ctx.Request.Request = request
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
	handlerFunc []HandlerFunction
	params      []string
	frozen      bool
	converters  map[string]interface{}
	assertions  map[string]string
	// name is the route's name
	name string
}

// NewRoute creates a new route with a path that handles all methods
func NewRoute(path string) *Route {
	return &Route{
		methods:     []string{"*"},
		params:      []string{},
		converters:  map[string]interface{}{},
		assertions:  map[string]string{},
		path:        path,
		handlerFunc: []HandlerFunction{},
	}
}

// SetName sets the route name
func (r *Route) SetName(name string) *Route {
	if r.IsFrozen() {
		return r
	}
	r.name = name
	return r
}

// Name returns the route's name
func (r *Route) Name() string {
	return r.name
}

// Params return route variable names.
// For instance if a route has the following pattern:
//    /catalog/:category/:productId
// it will return []string{"category","productId"}
func (r *Route) Params() []string { return r.params }

// HandlerFunc returns the current route handler function
func (r *Route) Handlers() []HandlerFunction {
	return r.handlerFunc
}

// HandlerFunction represent a route handler
type HandlerFunction interface{}

// SetHandlerFunc sets the route handler function.
//
// Can Panic!
func (r *Route) SetHandlers(handlerFunc ...HandlerFunction) {
	if r.IsFrozen() {
		return
	}
	for _, function := range handlerFunc {
		MustBeCallable(function)
	}
	r.handlerFunc = handlerFunc
}

// MethodMatch returns true if that method is handled by the route
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
func (r *Route) Freeze() *Route {
	if r.IsFrozen() {
		return r
	}
	// extract route variables
	routeVarsRegexp := regexp.MustCompile(Pattern)
	matches := routeVarsRegexp.FindAllStringSubmatch(r.path, -1)
	if matches != nil && len(matches) > 0 {
		for i, match := range matches {
			if match[0][0] == ':' {
				// looks like a :param use param without :
				r.params = append(r.params, match[1])
			} else {
				// looks like a valid regexp group, use the param position instead as key
				r.params = append(r.params, fmt.Sprintf("%d", i))
			}
		}
	}
	// replace route variables either with the default variable pattern or an assertion corresponding to the route variable
	stringPattern := routeVarsRegexp.ReplaceAllStringFunc(r.path, func(match string) string {
		// if an assertion is found, replace with the assertion
		params := regexp.MustCompile("\\w+").FindAllString(match, -1)
		if len(params) > 0 {
			if r.assertions[params[0]] != "" {
				return r.assertions[params[0]]
			}
		}
		//if match looks like a valid regexp group, return match untouched
		if match[0] == '(' && match[len(match)-1] == ')' {
			return match
		}
		return DefaultParamPattern
	})
	// add ^ and $ and optional /? to string pattern
	stringPattern = "^" + stringPattern + "/?$"
	r.pattern = regexp.MustCompile(stringPattern)
	if r.name == "" {
		r.name = regexp.MustCompile("\\W+").ReplaceAllString(r.path+"_"+fmt.Sprint(r.methods), "_")
	}
	r.frozen = true
	return r
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

// Convert converts a string value using a converter function. Arguments of
// the converter function will be injected according to their type. The initial value
// is injected as a string.
//
// Can Panic!
func (r *Route) Convert(param string, converterFunc conversionFunction) *Route {
	if !r.IsFrozen() {
		MustBeCallable(converterFunc)
		r.converters[param] = converterFunc
	}
	return r
}

// Assert asserts that a route variable respects a given regexp pattern.
//
// WILL Panic! if the pattern is not valid regexp pattern
func (r *Route) Assert(parameterName string, pattern string) *Route {
	if r.IsFrozen() {
		return r
	}
	// if the pattern is not a valid regexp pattern string, panic
	regexp.MustCompile("(" + pattern + ")")
	r.assertions[parameterName] = "(" + pattern + ")"
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
func (rc *RouteCollection) Get(path string, handlerFunc ...HandlerFunction) *Route {
	route := rc.All(path, handlerFunc...)
	route.SetMethods([]string{"GET", "HEAD"})
	return route
}

// Post creates a POST route
func (rc *RouteCollection) Post(path string, handlerFunc ...HandlerFunction) *Route {
	route := rc.All(path, handlerFunc...)
	route.SetMethods([]string{"POST"})
	return route
}

// Put creates a PUT route
func (rc *RouteCollection) Put(path string, handlerFunc ...HandlerFunction) *Route {
	route := rc.All(path, handlerFunc...)
	route.SetMethods([]string{"PUT"})
	return route
}

// Delete creates a DELETE route
func (rc *RouteCollection) Delete(path string, handlerFunc ...HandlerFunction) *Route {
	route := rc.All(path, handlerFunc...)
	route.SetMethods([]string{"DELETE"})
	return route
}

// Match creates a route that matches all methods
func (rc *RouteCollection) All(path string, handlerFunc ...HandlerFunction) *Route {
	if rc.IsFrozen() {
		panic(fmt.Sprintf("RouteCollection %v is frozen, no route can be added.", rc))
	}
	route := NewRoute(path)
	route.SetHandlers(handlerFunc...)
	rc.Routes = append(rc.Routes, route)
	return route
}

/**********************************/
/*         REQUEST MATCHER        */
/**********************************/
type RequestMatcher struct {
	routeCollection *RouteCollection
}

func NewRequestMatcher(routeCollection *RouteCollection) *RequestMatcher {
	return &RequestMatcher{routeCollection}
}
func (rm *RequestMatcher) Match(request *http.Request) *Route {
	// try to match current request url with a route
	if len(rm.routeCollection.Routes) > 0 {
		for _, route := range rm.routeCollection.Routes {
			if route.pattern.MatchString(request.URL.Path) && route.MethodMatch(request.Method) {
				return route
				break
			}
		}
	}
	return nil
}

func (rm *RequestMatcher) MatchAll(request *http.Request) (matches []*Route) {
	if len(rm.routeCollection.Routes) > 0 {
		for _, route := range rm.routeCollection.Routes {
			if route.pattern.MatchString(request.URL.Path) && route.MethodMatch(request.Method) {
				matches = append(matches, route)
			}
		}
	}
	return
}

/**********************************/
/*            INJECTOR            */
/**********************************/

// Injector is a dependency injection container
// Based on types.
type Injector struct {
	services map[reflect.Type]interface{}
	parent   *Injector
}

// NewInjector returns an new Injector
func NewInjector(services ...interface{}) *Injector {
	injector := &Injector{services: map[reflect.Type]interface{}{}}
	for _, service := range services {
		injector.Register(service)
	}
	return injector
}

// Register registers a new service to the injector
func (i *Injector) Register(service interface{}) {
	i.services[reflect.ValueOf(service).Type()] = service
}

// RegisterWithType registers a new service to the injector with a given type
func (i *Injector) RegisterWithType(service interface{}, Type interface{}) {
	if !reflect.TypeOf(service).ConvertibleTo(reflect.TypeOf(Type)) {
		panic(fmt.Sprint(service, " is not convertible to ", Type))
	}
	i.services[reflect.TypeOf(Type)] = service
}

// Resolve fetch the value according to a registered type
func (i *Injector) Resolve(someType reflect.Type) (interface{}, error) {
	var (
		err     error
		service interface{}
	)
	for typeService, service := range i.services {
		if typeService == someType {
			return service, nil
		} else if someType.Kind() == reflect.Interface && typeService.Implements(someType) {
			return service, nil
		} else if someType.Kind() == reflect.Ptr && someType.Elem().Kind() == reflect.Interface && typeService.Implements(someType.Elem()) {
			return service, nil
		}
	}
	if service == nil && i.parent != nil && i.parent != i {
		service, err = i.parent.Resolve(someType)
	}
	if service == nil {
		err = fmt.Errorf("service with type %v cannot be injected : not found", someType)
	}
	return service, err
}

// Apply applies resolved values to the given function
func (i *Injector) Apply(function interface{}) ([]interface{}, error) {
	var err error
	if !IsCallable(function) {
		return nil, fmt.Errorf("%v is not a function or a method\r\n%s", function, debug.Stack())
	}
	arguments := []reflect.Value{}
	callableValue := reflect.ValueOf(function)
	for j := 0; j < callableValue.Type().NumIn(); j++ {
		argument, err := i.Resolve(callableValue.Type().In(j))
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

// MustApply is the "can panic" version of MustApply
func (i *Injector) MustApply(function interface{}) (results []interface{}) {
	results, err := i.Apply(function)
	if err != nil {
		panic(err)
	}
	return
}

// SetParent sets the injector's parent
func (i *Injector) SetParent(parent *Injector) {
	i.parent = parent
}

// Parent gets the injector's parent
func (i Injector) Parent() *Injector {
	return i.parent
}

/**********************************/
/*              UTILS             */
/**********************************/

// IsCallable returns true if the value can
// be called like a function or a method.
func IsCallable(value interface{}) bool {
	if reflect.ValueOf(value).Kind() == reflect.Ptr {
		return reflect.ValueOf(value).Elem().Kind() == reflect.Func
	}
	return reflect.ValueOf(value).Kind() == reflect.Func
}

// MustBeCallable is the "panicable" version of IsCallable
//
// Can Panic!
func MustBeCallable(potentialFunction interface{}) {
	if !IsCallable(potentialFunction) {
		panic(fmt.Sprintf("%+v must be callable", potentialFunction))
	}
}

/**********************************/
/*             TYPEDEFS           */
/**********************************/

// ResponseWriterWithCode exposes the status of a response.
type ResponseWriterWithCode struct {
	http.ResponseWriter
	Code int
}

// WriteHeader sends an HTTP response header with status code.
func (r *ResponseWriterWithCode) WriteHeader(code int) {
	r.Code = code
	r.ResponseWriter.WriteHeader(code)
}

// Next represents a function
type Next func()

/**********************************/
/*        MIDDLEWARE STACK        */
/**********************************/

type HandlerFuncWithNext func(http.ResponseWriter, *http.Request, http.HandlerFunc)

// Stack is a middleware stack
type Stack struct {
	handlers []HandlerFuncWithNext
}

// NewStack returns a new Stack
func NewStack(handlers ...HandlerFuncWithNext) *Stack {
	return &Stack{handlers}
}

// NewStackFunc returns a HandlerFunc ready to be used with http;HandleFunc
func NewStackFunc(handlers ...HandlerFuncWithNext) http.HandlerFunc {
	return NewStack(handlers...).ServeHTTP
}

// ServeHTTP serves http requests
func (s *Stack) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	var handlers = s.handlers
	var next http.HandlerFunc
	next = func(rw http.ResponseWriter, r *http.Request) {
		if len(handlers) <= 0 {
			return
		}
		handler := handlers[0]
		handlers = handlers[1:]
		handler(rw, r, next)
	}
	next(rw, r)
}

// StackWithInjector is a middleware stack with a dependency injection container
type StackWithInjector struct {
	handlers []HandlerFunction
	injector *Injector
	next     Next
}

// NewStackWithInjector returns a new StackWithInjector
func NewStackWithInjector(injector *Injector, handlers ...HandlerFunction) *StackWithInjector {
	stack := &StackWithInjector{}
	stack.handlers = handlers
	stack.injector = NewInjector()
	stack.injector.SetParent(injector)
	return stack
}

func (s *StackWithInjector) SetNext(function Next) {
	s.next = function
}
func (s *StackWithInjector) HasNext() bool {

	return s.next != nil
}

// ServeHTTP serves http request
func (s *StackWithInjector) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	var handlers = s.handlers
	var next Next
	s.injector.Register(rw)
	s.injector.Register(r)
	next = func() {
		if len(handlers) <= 0 {
			if s.HasNext() {
				s.next()
			}
			return
		}
		handler := handlers[0]
		handlers = handlers[1:]
		MustBeCallable(handler)
		s.injector.MustApply(handler)
	}
	s.injector.Register(next)
	next()
}
