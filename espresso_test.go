package expresso_test

import (
	"fmt"
	"github.com/interactiv/expresso"
	"github.com/interactiv/expect"
	"net/http"
    "net/http/httptest"
	"testing"
)

func TestIntroUsage(t *testing.T) {
    var (
        req *http.Request
        err error
        
    )
	app:= expresso.App()
    app.Get("/hello/{name}",func(name string,rw http.ResponseWriter){
        fmt.Fprintf(rw,"Hello %s",name)
    })
    w:=httptest.NewRecorder()
    if req,err=http.NewRequest("GET","http://foobar.com/hello/foo",nil);err!=nil{
        t.Fatal(err)
    }
    app.ServeHTTP(w,req)
    expect.Expect(w.Code,t).ToBe(200)
    expect.Expect(w.Body.String(),t).ToBe("Hello foo")
}


