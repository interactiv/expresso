package expresso_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"testing"

	"github.com/interactiv/expect"
	"github.com/interactiv/expresso"
)

/********************************/
/*             TESTS            */
/********************************/

const formContentType = "application/x-www-form-urlencoded"

func TestHelloWord(t *testing.T) {

	var (
		req *http.Request
		err error
	)
	app := expresso.New()
	app.Get("/hello/:name", func(ctx *expresso.Context, rw http.ResponseWriter) {
		fmt.Fprintf(rw, "Hello %s", ctx.RequestVars["name"])
	})
	w := httptest.NewRecorder()
	if req, err = http.NewRequest("GET", "http://foobar.com/hello/foo", nil); err != nil {
		t.Fatal(err)
	}
	app.ServeHTTP(w, req)
	expect.Expect(w.Code, t).ToBe(200)
	expect.Expect(w.Body.String(), t).ToBe("Hello foo")
}

func TestPost(t *testing.T) {

	app := expresso.New()
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

	app := expresso.New()
	id := "10"
	e := expect.New(t)
	app.Put("/blog/:id", func(ctx *expresso.Context) {
		e.Expect(ctx.RequestVars["id"]).ToEqual(id)
	})
	req, err := http.NewRequest("PUT", fmt.Sprintf("http://foobar.com/blog/%s", id), nil)
	e.Expect(err).ToBeNil()
	res := httptest.NewRecorder()
	app.ServeHTTP(res, req)
}

func TestDelete(t *testing.T) {

	app := expresso.New()
	category := "food"
	id := "200"
	e := expect.New(t)
	app.Delete("/category/:category/product/:id", func(ctx *expresso.Context) {
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
	app := expresso.New()
	app.Match("/foo", func(rw http.ResponseWriter) {
		rw.WriteHeader(http.StatusOK)
	}).SetMethods([]string{"GET", "POST"})
	app.Match("/bar", func(rw http.ResponseWriter) {
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

func TestConvert(t *testing.T) {

	e := expect.New(t)
	app := expresso.New()
	app.Get("/person/:person", func(ctx *expresso.Context, rw http.ResponseWriter) {
		var person *Person
		person = ctx.RequestVars["person"].(*Person)
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
	app := expresso.New()
	e := expect.New(t)
	app.Get("/movies/:id", func(ctx *expresso.Context) {
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
	e.Expect(expresso.IsCallable(f)).ToBeTrue()
	foo := new(Foo)
	e.Expect(expresso.IsCallable(foo.Call)).ToBeTrue()
}

func TestInjector(t *testing.T) {
	e := expect.New(t)
	injector := expresso.NewInjector()
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
	r := expresso.NewRoute("/library/books")
	r.SetMethods([]string{"GET", "POST"})
	r.Freeze()
	expect.Expect(r.Name(), t).ToBe("_library_books__GET_POST_")
	r = expresso.NewRoute("/post/new")
	r.SetName("new_post")
	r.Freeze()
	expect.Expect(r.Name(), t).ToBe("new_post")

}

func TestExpressoError404(t *testing.T) {
	const errorMessage = "Route %v Not Found"
	const testRoute = "/foo/bar"
	app := expresso.New()
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

func TestExpressoError500(t *testing.T) {
	e := expect.New(t)
	app := expresso.New()
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

func TestExpressoError401(t *testing.T) {
	const (
		notAuthorizedMessage = "Not Authorized"
		notAuthorizedRoute   = "/notauthorized"
	)
	e := expect.New(t)
	app := expresso.New()
	app.Get(notAuthorizedRoute, func(rw http.ResponseWriter) {
		rw.WriteHeader(http.StatusUnauthorized)
		//rw.Write([]byte(notAuthorizedMessage))
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
	body, err := ioutil.ReadAll(res.Body)
	e.Expect(err).ToBe(nil)
	e.Expect(string(body)).ToEqual(notAuthorizedMessage)
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
