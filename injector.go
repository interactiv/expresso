package micro

import	(
	"reflect" 
	"fmt"
	"runtime/debug"
)

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