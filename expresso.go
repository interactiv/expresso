// Copyrights 2015 mparaiso <mparaiso@online.fr>
// License MIT

package expresso

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"runtime/debug"
	"strings"
)

var (
	// Pattern represents a route param regexp pattern
	Pattern = "(?:\\:)(\\w+)(\\?)?|(\\(.+\\)?)"
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
	*EventEmitter
	RequestMatcher *RequestMatcher
	booted         bool
	injector       *Injector
	errorHandlers  map[int]HandlerFunction
}

// New creates an expresso application
func New() *Expresso {
	expresso := &Expresso{
		RouteCollection: NewRouteCollection(),
		EventEmitter:    NewEventEmitter(),
		injector:        NewInjector(),
		errorHandlers:   map[int]HandlerFunction{},
	}
	expresso.injector.Register(expresso)
	return expresso
}

// Boot boots the application
func (e *Expresso) Boot() {
	if !e.Booted() {
		e.RouteCollection.Flush()
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
		next                   Next
		context                *Context
		requestInjector        *Injector
		responseWriterWithCode *ResponseWriterWithCode
	)
	defer func() {
		if err := recover(); err != nil {
			responseWriter.WriteHeader(http.StatusInternalServerError)
			log.Println(err)
			debug.PrintStack()
			requestInjector.MustApply(e.errorHandlers[500])
		}
	}()
	// wrap responseWriter so we can access the status code
	responseWriterWithCode = &ResponseWriterWithCode{
		ResponseWriter: responseWriter,
	}
	// sets context and injector
	context = NewContext(responseWriterWithCode, request)
	requestInjector = NewInjector(request, responseWriterWithCode, context, e.EventEmitter)
	requestInjector.Register(requestInjector)
	requestInjector.SetParent(e.Injector())
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

	// For the first matched route, call all its handlers
	// if an handler in a route calls expresso.Next next() , execute the next handler
	// When all handlers of a route have been called
	// if there are still some matched routes and the last handler of the previous route calls next
	// then repeat the process for the next matched route
	next = func() {
		if e.hasErrorCode(responseWriterWithCode, requestInjector) {
			return
		}
		if len(matches) == 0 {
			requestInjector.MustApply(e.errorHandlers[404])
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
				converterInjector.SetParent(requestInjector)
				res := converterInjector.MustApply(match.converters[key])
				if e.hasErrorCode(responseWriterWithCode, requestInjector) {
					return
				}
				if len(res) > 0 {
					// if only 1 value is returned , assign the value
					if len(res) == 1 {
						context.ConvertedRequestVars[key] = res[0]
						// if multiple values are returned , assign the array of values
					} else {
						context.ConvertedRequestVars[key] = res
					}
				}
			}
		}
		requestInjector.Register(next)
		context.next = next
		requestInjector.MustApply(match.Handler())
	}
	next()

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

// hasErrorCode Return true if a http status greater than 399 has been set
func (e *Expresso) hasErrorCode(rw *ResponseWriterWithCode, injector *Injector) bool {
	if code := rw.Code(); code > 399 {
		if e.errorHandlers[code] != nil && rw.Length() == 0 {
			injector.MustApply(e.errorHandlers[code])
		} else {
			http.Error(rw, http.StatusText(code), code)
		}
		return true
	}
	return false
}

// Injector return the injector
func (e *Expresso) Injector() *Injector {
	return e.injector
}

/**********************************/
/*     DEFAULT ERROR HANDLERS     */
/**********************************/

// InternalServerErrorHandler executes the default 500 handler
func InternalServerErrorHandler(rw http.ResponseWriter) {
	rw.Write([]byte(http.StatusText(http.StatusInternalServerError)))
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
	Request  *http.Request
	Response http.ResponseWriter
	// RequestVars are variables extracted from the request
	RequestVars          map[string]string
	ConvertedRequestVars map[string]interface{}
	//  Vars is a map to store any data during the request response cycle
	Vars map[string]interface{}
	next Next
}

// NewContext returns a new Context
func NewContext(response http.ResponseWriter, request *http.Request) *Context {
	ctx := &Context{
		RequestVars:          map[string]string{},
		ConvertedRequestVars: map[string]interface{}{},
		Vars:                 map[string]interface{}{},
		Request:              request,
		Response:             response,
	}
	return ctx
}

// SetStatus sets the the status code
func (ctx *Context) SetStatus(status int) {

	ctx.Response.WriteHeader(status)

}

// Next calls the next middleware in the middleware chain
func (ctx *Context) Next() {
	ctx.next()
}

// Redirect redirects request
func (ctx *Context) Redirect(path string, code int) {
	http.Redirect(ctx.Response, ctx.Request, path, code)
}

// WriteJSON writes json to response
func (ctx *Context) WriteJSON(v interface{}) error {
	ctx.Response.Header().Add("Content-Type", "application/json")
	return json.NewEncoder(ctx.Response).Encode(v)
}

// WriteXML writes xml to response
func (ctx *Context) WriteXML(v interface{}) error {
	ctx.Response.Header().Add("Content-Type", "text/xml")
	return xml.NewEncoder(ctx.Response).Encode(v)
}

// WriteString writes a string to response
func (ctx *Context) WriteString(v ...interface{}) (int, error) {
	return fmt.Fprint(ctx.Response, v...)
}

// WriteJSONP writes a jsonp response
func (ctx *Context) WriteJSONP(v interface{}, callbackName string) (n int, err error) {
	ctx.Response.Header().Add("Content-Type", "application/x-javascript")
	bytes, err := json.Marshal(v)
	if err != nil {
		return 0, err
	}
	n, err = ctx.WriteString(callbackName+"(", bytes, ")")
	return
}

// ReadJSON reads json from request's Body
func (ctx *Context) ReadJSON(v interface{}) error {
	return json.NewDecoder(ctx.Request.Body).Decode(v)
}

// ReadXML reads xml from request's body
func (ctx *Context) ReadXML(v interface{}) error {
	return xml.NewDecoder(ctx.Request.Body).Decode(v)
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
	handlerFunc HandlerFunction
	params      []string
	frozen      bool
	converters  map[string]interface{}
	assertions  map[string]string
	attributes  map[string]interface{}
	// name is the route's name
	name string
	// wether the route is intended to be a middlware or not
	passthrough bool
}

// NewRoute creates a new route with a path that handles all methods
func NewRoute(path string) *Route {
	return &Route{
		methods:     []string{"*"},
		params:      []string{},
		converters:  map[string]interface{}{},
		assertions:  map[string]string{},
		attributes:  map[string]interface{}{},
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

// Handler returns the current route handler function
func (r *Route) Handler() HandlerFunction {
	return r.handlerFunc
}

// HandlerFunction represent a route handler
type HandlerFunction interface{}

// SetHandler sets the route handler function.
//
// Can Panic!
func (r *Route) SetHandler(handlerFunc HandlerFunction) {
	if r.IsFrozen() {
		return
	}
	MustBeCallable(handlerFunc)
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
		// if an assertion is found, replace with the assertion pattern
		params := regexp.MustCompile("\\w+").FindAllString(match, -1)
		if len(params) > 0 {
			if r.assertions[params[0]] != "" {
				// optional ?
				if strings.HasSuffix(match, "?") {
					return "?" + r.assertions[params[0]] + "?"
				}
				return r.assertions[params[0]]
			}
		}
		//if match looks like a valid regexp group, return match untouched
		if match[0] == '(' && match[len(match)-1] == ')' {
			return match
		}
		//if match ends with ? , match is optional
		if strings.HasSuffix(match, "?") {
			return "?" + DefaultParamPattern + "?"
		}
		return DefaultParamPattern
	})
	// add ^ and $ and optional /? to string pattern
	if strings.HasSuffix(stringPattern, "/") {
		stringPattern = "^" + stringPattern + "?"
	} else {
		stringPattern = "^" + stringPattern + "/?"
	}
	if !r.passthrough {
		stringPattern = stringPattern + "$"
	}
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

// SetAttribute sets a route attribute
func (r *Route) SetAttribute(attr string, value interface{}) *Route {
	r.attributes[attr] = value
	return r
}

// Attribute returns a route attribute
func (r *Route) Attribute(attr string) interface{} {
	return r.attributes[attr]
}

/**********************************/
/*   ROUTE COLLECTION             */
/**********************************/

// RouteCollection is a collection of routes
type RouteCollection struct {
	Routes    []*Route
	prefix    string
	frozen    bool
	Children  []*RouteCollection
	hasParent bool
}

// NewRouteCollection creates a new RouteCollection
func NewRouteCollection() *RouteCollection {
	return &RouteCollection{Routes: []*Route{}, Children: []*RouteCollection{}}
}

// AddRoute adds a route to the route collection
func (rc *RouteCollection) AddRoute(r *Route) *RouteCollection {
	rc.Routes = append(rc.Routes, r)
	return rc
}

func (rc *RouteCollection) mustNotBeFrozen() {
	if rc.frozen {
		log.Panic("You cannot modify a route collection that has been frozen ", rc)
	}
}

func (rc *RouteCollection) setPrefix(prefix string) *RouteCollection {
	rc.mustNotBeFrozen()
	if prefix != "" && prefix[0] != '/' {
		prefix = "/" + prefix
	}
	rc.prefix = prefix
	return rc
}

// Flush freezes a route collection
func (rc *RouteCollection) Flush() {

	if rc.IsFrozen() == true {
		return
	}

	for _, route := range rc.Routes {
		route.path = rc.prefix + route.path
		route.Freeze()
	}

	if len(rc.Children) > 0 {

		for _, routeCollection := range rc.Children {
			routeCollection.setPrefix(rc.prefix + routeCollection.prefix).Flush()
			for _, route := range routeCollection.Routes {
				rc.Routes = append(rc.Routes, route)
			}
			routeCollection.Routes = []*Route{}
		}
	}
	rc.frozen = true
}

// IsFrozen returns true if the route collection is frozen
func (rc RouteCollection) IsFrozen() bool {
	return rc.frozen
}

// Use creates a passthrough route usefull for middlewares
func (rc *RouteCollection) Use(path string, handlerFunction HandlerFunction) *Route {
	route := rc.All(path, handlerFunction)
	route.passthrough = true
	return route
}

// Mount mounts a route collection on a path. All routes in the route collection will be prefixed
// with that path.
func (rc *RouteCollection) Mount(path string, routeCollection *RouteCollection) *RouteCollection {
	if !routeCollection.hasParent {

		rc.Children = append(rc.Children, routeCollection)
		routeCollection.setPrefix(path)
		routeCollection.hasParent = true
	}
	return rc
}

// Get creates a GET route
func (rc *RouteCollection) Get(path string, handlerFunction HandlerFunction) *Route {
	route := rc.All(path, handlerFunction)
	route.SetMethods([]string{"GET", "HEAD"})
	return route
}

// Post creates a POST route
func (rc *RouteCollection) Post(path string, handlerFunction HandlerFunction) *Route {
	route := rc.All(path, handlerFunction)
	route.SetMethods([]string{"POST"})
	return route
}

// Put creates a PUT route
func (rc *RouteCollection) Put(path string, handlerFunction HandlerFunction) *Route {
	route := rc.All(path, handlerFunction)
	route.SetMethods([]string{"PUT"})
	return route
}

// Delete creates a DELETE route
func (rc *RouteCollection) Delete(path string, handlerFunction HandlerFunction) *Route {
	route := rc.All(path, handlerFunction)
	route.SetMethods([]string{"DELETE"})
	return route
}

// All creates a route that matches all methods
func (rc *RouteCollection) All(path string, handlerFunction HandlerFunction) *Route {
	rc.mustNotBeFrozen()
	route := NewRoute(path)
	route.SetHandler(handlerFunction)
	rc.Routes = append(rc.Routes, route)
	return route
}

/**********************************/
/*            MATCHERS            */
/**********************************/

// Matcher is a type something that can match a http.Request
type Matcher interface {
	Match(*http.Request) bool
}

// RequestMatcher match request path to route pattern
type RequestMatcher struct {
	routeCollection *RouteCollection
}

// NewRequestMatcher returns a new RequestMatcher
func NewRequestMatcher(routeCollection *RouteCollection) *RequestMatcher {
	return &RequestMatcher{routeCollection}
}

// Match returns a route that matches a http.Request
func (rm *RequestMatcher) Match(request *http.Request) *Route {
	// try to match current request url with a route
	if len(rm.routeCollection.Routes) > 0 {
		for _, route := range rm.routeCollection.Routes {
			if route.pattern.MatchString(request.URL.Path) && route.MethodMatch(request.Method) {
				return route

			}
		}
	}
	return nil
}

// MatchAll matches all routes matching the request in the route collection
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
/*         EVENT EMITTER          */
/**********************************/

// Listener is an event handler function
type Listener *func(string, ...interface{}) bool

// EventEmitter listens for and emits events
type EventEmitter struct {
	handlers map[string][]Listener
}

// NewEventEmitter returns a new event emitter
func NewEventEmitter() *EventEmitter {
	return &EventEmitter{
		handlers: map[string][]Listener{},
	}
}

// Emit emits an event
func (em *EventEmitter) Emit(event string, arguments ...interface{}) {
	if len(em.handlers) > 0 && em.handlers[event] != nil {
		for _, handler := range em.handlers[event] {
			Continue := (*handler)(event, arguments...)
			if !Continue {
				break
			}
		}
	}
}

// AddListener adds a new listener function pointer
func (em *EventEmitter) AddListener(event string, listener Listener) {
	if em.handlers[event] != nil {
		em.handlers[event] = []Listener{}
	}
	em.handlers[event] = append(em.handlers[event], listener)
}

// RemoveListener removes a listener function pointer
func (em *EventEmitter) RemoveListener(event string, listener Listener) bool {
	var found bool
	if em.handlers[event] != nil {
		for i, handler := range em.handlers[event] {
			if handler == listener {

				head := em.handlers[event][0:i]
				if length := len(em.handlers); i == length-1 {
					em.handlers[event] = head
				} else {
					tail := em.handlers[event][i+1 : length-1]
					em.handlers[event] = append(head, tail...)
				}

				found = true
				break
			}
		}
	}
	return found
}

// RemoveAllListeners remove all listeners given an event and returns the listener slice
func (em *EventEmitter) RemoveAllListeners(event string) []Listener {
	listeners := []Listener{}
	if em.handlers[event] != nil {
		listeners, em.handlers[event] = em.handlers[event], listeners
	}
	return listeners
}

// HasListener returns true if an event has listeners
func (em *EventEmitter) HasListener(event string) bool {
	if em.handlers[event] != nil && len(em.handlers[event]) > 0 {
		return true
	}
	return false
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

// Must will panic if err is not nil
func Must(err error) {
	if err != nil {
		panic(err)
	}
}

// MustWithResult returns a result or panics on error
func MustWithResult(result interface{}, err error) interface{} {
	if err != nil {
		panic(err)
	}
	return result
}

/**********************************/
/*             TYPEDEFS           */
/**********************************/

// ResponseWriterWithCode exposes the status of a response.
type ResponseWriterWithCode struct {
	http.ResponseWriter
	code          int
	writtenLength int
}

// WriteHeader sends an HTTP response header with status code.
func (r *ResponseWriterWithCode) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

// Write writes to the response
func (r *ResponseWriterWithCode) Write(b []byte) (int, error) {
	i, err := r.ResponseWriter.Write(b)
	r.writtenLength = r.writtenLength + len(b)
	return i, err
}

// Code returns the response status code
func (r *ResponseWriterWithCode) Code() int {
	return r.code
}

// Length returns the number of bytes written in the response
func (r *ResponseWriterWithCode) Length() int {
	return r.writtenLength
}

// Next represents a function
type Next func()
