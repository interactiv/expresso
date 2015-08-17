//    Monorail version 0.4
//    Monorail is a web framework for the Go language
//    Copyright (C) 2015  mparaiso <mparaiso@online.fr>
//
//    This program is free software: you can redistribute it and/or modify
//    it under the terms of the GNU General Public License as published by
//    the Free Software Foundation, either version 3 of the License, or
//    (at your option) any later version.

//    This program is distributed in the hope that it will be useful,
//    but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//    GNU General Public License for more details.

//    You should have received a copy of the GNU General Public License
//    along with this program.  If not, see <http://www.gnu.org/licenses/>

package monorail_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/interactiv/expect"
	"github.com/interactiv/monorail"
)

/********************************/
/*             TESTS            */
/********************************/

const formContentType = "application/x-www-form-urlencoded"

func TestHelloWord(t *testing.T) {

	app := monorail.New()
	app.Get("/hello/:name", func(ctx *monorail.Context, rw http.ResponseWriter) {
		fmt.Fprintf(rw, "Hello %s", ctx.RequestVars["name"])
	})
	server := httptest.NewServer(app)
	defer server.Close()
	res := monorail.MustWithResult(http.Get(server.URL + "/hello/foo")).(*http.Response)
	defer res.Body.Close()
	expect.Expect(res.StatusCode, t).ToBe(200)
	expect.Expect(string(monorail.MustWithResult(ioutil.ReadAll(res.Body)).([]byte)), t).ToBe("Hello foo")
}

func TestOptionalRequestVariable(t *testing.T) {
	app := monorail.New()
	e := expect.New(t)
	app.Use("/", func(next monorail.Next) { next() })
	app.Get("/:param?", func(ctx *monorail.Context) {
		ctx.WriteString("param: ", ctx.RequestVars["param"])
	})
	app.Get("/:param1?/:param2", func(ctx *monorail.Context) {
		ctx.WriteString(ctx.RequestVars["param1"], ctx.RequestVars["param2"])
	})
	server := httptest.NewServer(app)
	defer server.Close()
	res := monorail.MustWithResult(http.Get(server.URL + "/example")).(*http.Response)
	defer res.Body.Close()
	e.Expect(res.StatusCode).ToBe(200)
	body := string(monorail.MustWithResult(ioutil.ReadAll(res.Body)).([]byte))
	e.Expect(body).ToContain("example")
	res = monorail.MustWithResult(http.Get(server.URL + "/")).(*http.Response)
	defer res.Body.Close()
	e.Expect(res.StatusCode).ToBe(200)
	body = string(monorail.MustWithResult(ioutil.ReadAll(res.Body)).([]byte))
	e.Expect(body).Not().ToContain("example")
	e.Expect(body).ToContain("param:")
	res = monorail.MustWithResult(http.Get((server.URL + "/job/salary"))).(*http.Response)
	defer res.Body.Close()
	e.Expect(res.StatusCode).ToBe(200)
	body = string(monorail.MustWithResult(ioutil.ReadAll(res.Body)).([]byte))
	e.Expect(body).ToContain("job")
	e.Expect(body).ToContain("salary")
	res = monorail.MustWithResult(http.Get(server.URL + "/house/room/door")).(*http.Response)
	defer res.Body.Close()
	e.Expect(res.StatusCode).ToBe(404)
	//body =string(monorail.MustWithResult(ioutil.ReadAll(res.Body)).([]byte))
}

func TestPost(t *testing.T) {

	app := monorail.New()
	app.Get("/feedback", func() {
		t.Fatalf("GET /feedback shouldn't be called on POST /feedback request")
	})
	app.Post("/feedback", func(rw http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		expect.Expect(err, t).ToBeNil()
		message := r.Form.Get("message")
		expect.Expect(message, t).ToBe("message")
	})

	w := httptest.NewRecorder()
	values := url.Values{}
	values.Add("message", "message")
	body := new(bytes.Buffer)
	body.WriteString(values.Encode())
	req, err := http.NewRequest("POST", "http://foo.com/feedback", body)
	req.Header.Set("Content-Type", formContentType)
	expect.Expect(err, t).ToBeNil()
	app.ServeHTTP(w, req)
	expect.Expect(w.Code, t).ToBe(200)
}

func TestPut(t *testing.T) {

	app := monorail.New()
	id := "10"
	e := expect.New(t)
	app.Put("/blog/:id", func(ctx *monorail.Context) {
		e.Expect(ctx.RequestVars["id"]).ToEqual(id)
	})
	req, err := http.NewRequest("PUT", fmt.Sprintf("http://foobar.com/blog/%s", id), nil)
	e.Expect(err).ToBeNil()
	res := httptest.NewRecorder()
	app.ServeHTTP(res, req)
}

func TestDelete(t *testing.T) {

	app := monorail.New()
	category := "food"
	id := "200"
	e := expect.New(t)
	app.Delete("/category/:category/product/:id", func(ctx *monorail.Context) {
		e.Expect(ctx.RequestVars["category"]).ToEqual(category)
		e.Expect(ctx.RequestVars["id"]).ToEqual(id)
	})
	res := httptest.NewRecorder()
	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://example.com/category/%s/product/%s?foo=bar", category, id), nil)
	e.Expect(err).ToBeNil()
	app.ServeHTTP(res, req)
	e.Expect(res.Code).ToEqual(200)
}

func TestMatch(t *testing.T) {
	e := expect.New(t)
	app := monorail.New()
	app.All("/foo", func(rw http.ResponseWriter) {
		rw.WriteHeader(http.StatusOK)
	}).SetMethods([]string{"GET", "POST"})
	app.All("/bar", func(rw http.ResponseWriter) {
		rw.WriteHeader(http.StatusOK)
	}).SetMethods([]string{"*"})
	server := httptest.NewServer(app)
	defer server.Close()
	res, err := http.Get(server.URL + "/foo")
	e.Expect(err).ToBeNil()
	e.Expect(res.StatusCode).ToEqual(200)
	res, err = http.Post(server.URL+"/foo", formContentType, nil)
	defer res.Body.Close()
	e.Expect(err).ToBeNil()
	e.Expect(res.StatusCode).ToBe(200)
	req, err := http.NewRequest("PUT", server.URL+"/foo", nil)
	e.Expect(err).ToBeNil()
	res, err = http.DefaultClient.Do(req)
	defer res.Body.Close()
	e.Expect(err).ToBeNil()
	e.Expect(res.StatusCode).ToBe(http.StatusNotFound)
	req, err = http.NewRequest("OPTIONS", server.URL+"/bar", nil)
	e.Expect(err).ToBeNil()
	res, err = http.DefaultClient.Do(req)
	e.Expect(err).ToBeNil()
	defer res.Body.Close()
	e.Expect(res.StatusCode).ToEqual(http.StatusOK)
}

func TestUse(t *testing.T) {
	app := monorail.New()
	e := expect.New(t)
	app.Use("/", func(rw http.ResponseWriter, next monorail.Next) {
		rw.Write([]byte("Use"))
		next()
	})
	app.Get("/example", func(rw http.ResponseWriter) {
		rw.Write([]byte("example"))
	})
	server := httptest.NewServer(app)
	defer server.Close()
	res, err := http.Get(server.URL + "/example")
	defer res.Body.Close()
	e.Expect(err).ToBeNil()
	e.Expect(res.StatusCode).ToBe(200)
	body, err := ioutil.ReadAll(res.Body)
	e.Expect(err).ToBeNil()
	e.Expect(string(body)).ToContain("Use")
}

func TestConvert(t *testing.T) {

	e := expect.New(t)
	app := monorail.New()
	app.Get("/person/:person", func(ctx *monorail.Context, rw http.ResponseWriter) {
		var person *Person
		person = ctx.ConvertedRequestVars["person"].(*Person)
		fmt.Fprintf(rw, "%s", person.name)
		e.Expect(person).Not().ToBeNil()
	}).Convert("person", func(person string, r *http.Request) *Person {
		id, err := strconv.Atoi(person)
		if err != nil {
			return nil
		}
		return PersonRepository.Find(id)
	})
	server := httptest.NewServer(app)
	defer server.Close()
	response, err := http.Get(server.URL + "/person/0")
	e.Expect(err).ToBeNil()
	defer response.Body.Close()
	e.Expect(response.StatusCode).ToBe(200)
}

func TestAssert(t *testing.T) {
	app := monorail.New()
	e := expect.New(t)
	app.Get("/movies/:id", func(ctx *monorail.Context) {
		e.Expect(ctx.RequestVars["id"]).ToEqual("0123")
	}).Assert("id", "\\d+")
	server := httptest.NewServer(app)
	defer server.Close()
	res, err := http.Get(server.URL + "/movies/foobar")
	defer res.Body.Close()
	e.Expect(err).ToBeNil()
	e.Expect(res.StatusCode).ToEqual(404)
	res, err = http.Get(server.URL + "/movies/0123")
	defer res.Body.Close()
	e.Expect(err).ToBeNil()
	e.Expect(res.StatusCode).ToEqual(200)
}

func TestIsCallable(t *testing.T) {
	var f = func() {}
	e := expect.New(t)
	e.Expect(monorail.IsCallable(f)).ToBeTrue()
	foo := new(Foo)
	e.Expect(monorail.IsCallable(foo.Call)).ToBeTrue()
}

func TestInjector(t *testing.T) {
	e := expect.New(t)
	injector := monorail.NewInjector()
	injector.Register(&Foo{Bar: "bar"})
	f, err := injector.Resolve(reflect.TypeOf((*Foo)(nil)))
	e.Expect(err).ToBeNil()
	e.Expect(f).Not().ToBeNil()
	f1, err := injector.Resolve(reflect.TypeOf((*Caller)(nil)))
	e.Expect(err).ToBeNil()
	e.Expect(f1).Not().ToBeNil()
	// Test apply
	var res []interface{}
	res, err = injector.Apply(func(c Caller) string {
		return c.Call()
	})
	e.Expect(err).ToBeNil()
	e.Expect(res[0]).ToEqual("called")

}

func TestBind(t *testing.T) {
	r := monorail.NewRoute("/post/new")
	r.SetName("new_post")
	expect.Expect(r.Name(), t).ToBe("new_post")
}

func TestError(t *testing.T) {
	e := expect.New(t)
	e.Expect(func() {
		app := monorail.New()
		app.Error(100, func() {})
	}).ToPanic()
}

func TestmonorailError404(t *testing.T) {
	const errorMessage = "Route %v Not Found"
	const testRoute = "/foo/bar"
	app := monorail.New()
	e := expect.New(t)
	app.Error(404, func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(rw, errorMessage, r.URL.Path)
	})
	server := httptest.NewServer(app)
	defer server.Close()
	res, err := http.Get(server.URL + testRoute)
	defer res.Body.Close()
	e.Expect(err).ToBeNil()
	e.Expect(res.StatusCode).ToBe(404)
	body, err := ioutil.ReadAll(res.Body)
	e.Expect(err).ToBeNil()
	e.Expect(string(body)).ToBe(fmt.Sprintf(errorMessage, testRoute))

}

func TestmonorailError500(t *testing.T) {
	e := expect.New(t)
	app := monorail.New()
	app.Get("/", func(foo *Foo) {})
	app.Error(500, func(rw http.ResponseWriter) {
		http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	})
	server := httptest.NewServer(app)
	defer server.Close()
	res, err := http.Get(server.URL)
	defer res.Body.Close()
	e.Expect(err).ToBeNil()
	e.Expect(res.StatusCode).ToEqual(500)
}

func TestmonorailError401(t *testing.T) {
	const (
		notAuthorizedMessage = "Not Authorized"
		notAuthorizedRoute   = "/notauthorized"
	)
	e := expect.New(t)
	app := monorail.New()
	app.Get(notAuthorizedRoute, func(rw http.ResponseWriter, next monorail.Next) {
		rw.WriteHeader(http.StatusUnauthorized)
		next()
	})
	app.Error(401, func(rw http.ResponseWriter) {
		rw.Write([]byte(notAuthorizedMessage))
	})
	server := httptest.NewServer(app)
	defer server.Close()
	res, err := http.Get(server.URL + notAuthorizedRoute)
	defer res.Body.Close()
	e.Expect(err).ToBeNil()
	e.Expect(res.StatusCode).ToEqual(401)
	body := string(monorail.MustWithResult(ioutil.ReadAll(res.Body)).([]byte))
	e.Expect(body).ToEqual(notAuthorizedMessage)
}

// TestMonorailRouteMatchers test the new route matcher api
func TestMonorailRouteMatchers(t *testing.T) {
	e := expect.New(t)
	requests := map[string]*http.Request{
		"GET":  &http.Request{Method: "GET"},
		"PUT":  &http.Request{Method: "PUT"},
		"HEAD": &http.Request{Method: "HEAD"},
		"POST": &http.Request{Method: "POST"},
	}
	methodMatcher := monorail.NewMethodMatcher()
	for _, request := range requests {
		e.Expect(methodMatcher.Match(request)).ToBeTrue()
	}

	methodMatcher = monorail.NewMethodMatcher("GET")
	e.Expect(methodMatcher.Match(requests["GET"])).ToBeTrue()
	e.Expect(methodMatcher.Match(requests["HEAD"])).ToBeTrue()
}

// TestPrefix makes sure that given a mounted route at /
// if a subroute is /example , then the subroute is accessible at /example and //example
func TestPrefix(t *testing.T) {
	const message = "example"
	e := expect.New(t)
	app := monorail.New()
	routeCollection := monorail.NewRouteCollection()
	routeCollection.All("/"+message, func(rw http.ResponseWriter) {
		rw.Write([]byte(message))
	})
	app.Mount("/", routeCollection)
	server := httptest.NewServer(app)
	defer server.Close()
	response := monorail.MustWithResult(http.Get(server.URL + "/" + message)).(*http.Response)
	defer response.Body.Close()
	e.Expect(response.StatusCode).ToEqual(200)
	body := string(monorail.MustWithResult(ioutil.ReadAll(response.Body)).([]byte))
	e.Expect(body).ToEqual(message)
}

/**********************************/
/*      EVENT EMITTER TESTS       */
/**********************************/
func TestEventEmitter(t *testing.T) {
	var called int
	e := expect.New(t)
	em := monorail.NewEventEmitter()
	listener := func(event string, arguments ...interface{}) bool {
		called = called + 1
		return true
	}
	em.AddListener("event", &listener)
	em.Emit("event")
	e.Expect(called).ToBe(1)
	em.RemoveListener("event", &listener)
	em.Emit("event")
	e.Expect(called).ToBe(1)
	em.AddListener("event", &listener)
	em.RemoveAllListeners("event")
	em.Emit("event")
	e.Expect(called).ToEqual(1)
	e.Expect(em.HasListener("event")).ToBeFalse()
	em.AddListener("event", &listener)
	e.Expect(em.HasListener("event")).ToBeTrue()
}

/**********************************/
/*     ROUTE COLLECTION TESTS     */
/**********************************/

func TestAddRoute(t *testing.T) {
	rc := monorail.NewRouteCollection()
	route := monorail.NewRoute("/")
	rc.AddRoute(route)
	e := expect.New(t)
	e.Expect(len(rc.Routes)).ToBe(1)
}

func TestRouteCollectionMount(t *testing.T) {
	e := expect.New(t)
	app := monorail.New()
	subRoutes := monorail.NewRouteCollection()
	subRoutes.Use("/", func(ctx *monorail.Context, next monorail.Next) {
		ctx.WriteString("Use")
		next()
	})
	subRoutes.All("/", func(ctx *monorail.Context) {
		ctx.WriteString("SubRoutes")
	})
	app.Mount("/subroutes", subRoutes)
	subRoutes2 := monorail.NewRouteCollection()
	subRoutes2.All("/", func(ctx *monorail.Context) {
		ctx.WriteString("SubSubRoutes")
	})
	subRoutes.Mount("/subroutes", subRoutes2)
	server := httptest.NewServer(app)
	defer server.Close()
	res, err := http.Get(server.URL + "/subroutes")
	defer res.Body.Close()
	e.Expect(err).ToBeNil()
	e.Expect(res.StatusCode).ToBe(200)
	body, err := ioutil.ReadAll(res.Body)
	e.Expect(string(body)).ToBe("UseSubRoutes")
	res, err = http.Post(server.URL+"/subroutes/subroutes", "application/x-www-form-urlencoded", nil)
	defer res.Body.Close()
	e.Expect(err).ToBeNil()
	e.Expect(res.StatusCode).ToBe(200)
	body, _ = ioutil.ReadAll(res.Body)
	e.Expect(string(body)).ToEqual("UseSubSubRoutes")
}

/**********************************/
/*         CONTEXT TESTS          */
/**********************************/

func TestContextReadJson(t *testing.T) {
	e := expect.New(t)
	response := httptest.NewRecorder()
	context := monorail.NewContext(response, nil)
	type Account struct {
		Balance float32
	}
	account := &Account{Balance: 1000.0}
	context.WriteJSON(account)
	e.Expect(response.Header().Get("Content-Type")).ToBe("application/json")
	e.Expect(response.Body.String()).ToContain(`{"Balance":1000}`)
	req, _ := http.NewRequest("GET", "example.com", strings.NewReader(`{"Balance":500}`))
	context = monorail.NewContext(nil, req)
	context.ReadJSON(account)
	e.Expect(account.Balance).ToEqual(float32(500))
	response = httptest.NewRecorder()
	context = monorail.NewContext(response, nil)
	context.WriteString("foo", "bar")
	e.Expect(response.Body.String()).ToEqual("foobar")
}

/**********************************/
/*           UTILS TESTS          */
/**********************************/

func TestMustWithResult(t *testing.T) {
	b := func() (*Foo, error) {
		return new(Foo), nil
	}
	_ = monorail.MustWithResult(b()).(*Foo)
}

/********************************/
/*            FIXTURES          */
/********************************/

type Foo struct {
	Bar string
}

func (f Foo) Call() string {
	return "called"
}

type Caller interface {
	Call() string
}

type Person struct {
	id   int
	name string
}

func (p Person) Find(id int) *Person {
	var (
		person           *Person
		personRepository = []*Person{
			&Person{id: 0, name: "James"},
			&Person{id: 1, name: "Frank"},
		}
	)
	for _, p := range personRepository {
		if p.id == id {
			person = p
			break
		}
	}
	return person
}

var (
	PersonRepository Person
)
