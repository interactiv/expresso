package routing

type Route struct {
	path         string
	host         string
	schemes      []string
	methods      []string
	defaults     map[string]interface{}
	requirements map[string]interface{}
	options      []string
	compiled     interface{}
	condition    string
}

/*
 * Getters and setters for struct type Route
 */

// GetPath returns a string
func (route Route) Path() string {
	return route.path
}

// SetPath sets *Route.route and returns *Route
func (route *Route) SetPath(path string) *Route {
	route.path = path
	return route
}

// GetHost returns a string
func (route Route) Host() string {
	return route.host
}

// SetHost sets *Route.route and returns *Route
func (route *Route) SetHost(host string) *Route {
	route.host = host
	return route
}

// GetSchemes returns a []string
func (route Route) Schemes() []string {
	return route.schemes
}

// SetSchemes sets *Route.route and returns *Route
func (route *Route) SetSchemes(schemes []string) *Route {
	route.schemes = schemes
	return route
}

// GetMethods returns a []string
func (route Route) Methods() []string {
	return route.methods
}

// SetMethods sets *Route.route and returns *Route
func (route *Route) SetMethods(methods []string) *Route {
	route.methods = methods
	return route
}

// GetDefaults returns a map[string]interface{}
func (route Route) Defaults() map[string]interface{} {
	return route.defaults
}

// SetDefaults sets *Route.route and returns *Route
func (route *Route) SetDefaults(defaults map[string]interface{}) *Route {
	route.defaults = defaults
	return route
}

// GetRequirements returns a map[string]interface{}
func (route Route) Requirements() map[string]interface{} {
	return route.requirements
}

// SetRequirements sets *Route.route and returns *Route
func (route *Route) SetRequirements(requirements map[string]interface{}) *Route {
	route.requirements = requirements
	return route
}

// GetOptions returns a []string
func (route Route) Options() []string {
	return route.options
}

// SetOptions sets *Route.route and returns *Route
func (route *Route) SetOptions(options []string) *Route {
	route.options = options
	return route
}

// GetCompiled returns a interface{}
func (route Route) Compiled() interface{} {
	return route.compiled
}

// SetCompiled sets *Route.route and returns *Route
func (route *Route) SetCompiled(compiled interface{}) *Route {
	route.compiled = compiled
	return route
}

// GetCondition returns a string
func (route Route) Condition() string {
	return route.condition
}

// SetCondition sets *Route.route and returns *Route
func (route *Route) SetCondition(condition string) *Route {
	route.condition = condition
	return route
}
