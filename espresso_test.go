package expresso_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/interactiv/expect"
	"github.com/interactiv/expresso"
)

const formContentType = "application/x-www-form-urlencoded"

func TestHelloWord(t *testing.T) {

	var (
		req *http.Request
		err error
	)
	app := expresso.App()
	app.Get("/hello/:name", func(ctx *expresso.Context, rw http.ResponseWriter) {
		fmt.Fprintf(rw, "Hello %s", ctx.Request.Params["name"])
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

	app := expresso.App()
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

	app := expresso.App()
	id := "10"
	e := expect.New(t)
	app.Put("/blog/:id", func(ctx *expresso.Context) {
		e.Expect(ctx.Request.Params["id"]).ToEqual(id)
	})
	req, err := http.NewRequest("PUT", fmt.Sprintf("http://foobar.com/blog/%s", id), nil)
	e.Expect(err).ToBeNil()
	res := httptest.NewRecorder()
	app.ServeHTTP(res, req)
}

func TestDelete(t *testing.T) {

	app := expresso.App()
	category := "food"
	id := "200"
	e := expect.New(t)
	app.Delete("/category/:category/product/:id", func(ctx *expresso.Context) {
		e.Expect(ctx.Request.Params["category"]).ToEqual(category)
		e.Expect(ctx.Request.Params["id"]).ToEqual(id)
	})
	res := httptest.NewRecorder()
	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://example.com/category/%s/product/%s?foo=bar", category, id), nil)
	e.Expect(err).ToBeNil()
	app.ServeHTTP(res, req)
	e.Expect(res.Code).ToEqual(200)
}

func TestMatch(t *testing.T) {
	e := expect.New(t)
	app := expresso.App()
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
	f, err := injector.Get(reflect.TypeOf((*Foo)(nil)))
	e.Expect(err).ToBeNil()
	e.Expect(f).Not().ToBeNil()
	t.Log(f, reflect.TypeOf(f))
	f1, err := injector.Get(reflect.TypeOf((*Caller)(nil)))
	e.Expect(err).ToBeNil()
	e.Expect(f1).Not().ToBeNil()
	t.Log(f1, reflect.TypeOf(f1))
	// Test apply
	var res []interface{}
	res, err = injector.Apply(func(c Caller) string {
		return c.Call()
	})
	e.Expect(err).ToBeNil()
	e.Expect(res[0]).ToEqual("called")
	t.Log(res)

}

type Foo struct {
	Bar string
}

func (f Foo) Call() string {
	return "called"
}

type Caller interface {
	Call() string
}
