package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPost(t *testing.T) {
	var body ProduceRequest
	body.Record = Record{
		Offset: 0,
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/", bytes.NewBuffer(b))
	rec := httptest.NewRecorder()

	svr := NewHTTPServer(":8080")
	svr.Handler.ServeHTTP(rec, req)
	res := rec.Result()

	if res.StatusCode != http.StatusOK {
		t.Fatal("invalid status code")
	}
}
