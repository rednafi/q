package httpclient

import (
   "bytes"
   "io"
   "net/http"
   "testing"
)

// fakeClient implements HTTPClient for testing.
type fakeClient struct {
   resp *http.Response
}

func (f *fakeClient) Do(req *http.Request) (*http.Response, error) {
   return f.resp, nil
}

func TestSetClientAndDo(t *testing.T) {
   orig := client
   defer SetClient(orig)
   body := io.NopCloser(bytes.NewBufferString("hello"))
   fc := &fakeClient{resp: &http.Response{StatusCode: 200, Body: body}}
   SetClient(fc)
   req, err := http.NewRequest("GET", "http://example.com", nil)
   if err != nil {
       t.Fatalf("NewRequest error: %v", err)
   }
   resp, err := Do(req)
   if err != nil {
       t.Fatalf("Do error: %v", err)
   }
   if resp.StatusCode != 200 {
       t.Errorf("expected status 200, got %d", resp.StatusCode)
   }
   data, err := io.ReadAll(resp.Body)
   if err != nil {
       t.Fatalf("ReadAll error: %v", err)
   }
   if string(data) != "hello" {
       t.Errorf("expected body 'hello', got %q", string(data))
   }
}