package proxy

import (
	"encoding/json"
	"errors"
	uuid "github.com/satori/go.uuid"
	"io"
	"net/http"
	"net/url"
	"sync"
)

// flow http request
type Request struct {
	Method string
	URL    *url.URL
	Proto  string
	Header http.Header
	Body   []byte

	raw    *http.Request
	RawStr string
}

func newRequest(req *http.Request) *Request {
	return &Request{
		Method: req.Method,
		URL:    req.URL,
		Proto:  req.Proto,
		Header: req.Header,
		raw:    req,
	}
}

func (r *Request) Raw() *http.Request {
	return r.raw
}

func (req *Request) MarshalJSON() ([]byte, error) {
	r := make(map[string]interface{})
	r["method"] = req.Method
	r["url"] = req.URL.String()
	r["proto"] = req.Proto
	r["header"] = req.Header
	return json.Marshal(r)
}

func (req *Request) UnmarshalJSON(data []byte) error {
	r := make(map[string]interface{})
	err := json.Unmarshal(data, &r)
	if err != nil {
		return err
	}

	rawurl, ok := r["url"].(string)
	if !ok {
		return errors.New("url parse error")
	}
	u, err := url.Parse(rawurl)
	if err != nil {
		return err
	}

	rawheader, ok := r["header"].(map[string]interface{})
	if !ok {
		return errors.New("rawheader parse error")
	}

	header := make(map[string][]string)
	for k, v := range rawheader {
		vals, ok := v.([]interface{})
		if !ok {
			return errors.New("header parse error")
		}

		svals := make([]string, 0)
		for _, val := range vals {
			sval, ok := val.(string)
			if !ok {
				return errors.New("header parse error")
			}
			svals = append(svals, sval)
		}
		header[k] = svals
	}

	*req = Request{
		Method: r["method"].(string),
		URL:    u,
		Proto:  r["proto"].(string),
		Header: header,
	}
	return nil
}

// flow http response
type Response struct {
	StatusCode int         `json:"statusCode"`
	Header     http.Header `json:"header"`
	Body       []byte      `json:"-"`
	BodyReader io.Reader
	Raw        *http.Response
	close      bool // connection close

	decodedBody []byte
	decoded     bool // decoded reports whether the response was sent compressed but was decoded to decodedBody.
	decodedErr  error
}

// flow
type Flow struct {
	UUID        uuid.UUID
	Id          int
	ConnContext *ConnContext
	Request     *Request
	Response    *Response
	// https://docs.mitmproxy.org/stable/overview-features/#streaming
	// 如果为 true，则不缓冲 Request.Body 和 Response.Body，且不进入之后的 Addon.Request 和 Addon.Response
	Stream            bool
	UseSeparateClient bool // use separate http client to send http request
	done              chan struct{}
}

// todo  yhy
var Id int
var muId sync.Mutex

func IdAdd() int {
	muId.Lock()
	defer muId.Unlock()
	Id += 1
	return Id
}

func newFlow() *Flow {
	IdAdd()
	return &Flow{
		Id:   Id,
		UUID: uuid.NewV4(),
		done: make(chan struct{}),
	}
}

func (f *Flow) Done() <-chan struct{} {
	return f.done
}

func (f *Flow) finish() {
	close(f.done)
}

func (f *Flow) MarshalJSON() ([]byte, error) {
	j := make(map[string]interface{})
	j["id"] = f.Id
	j["uuid"] = f.UUID
	j["request"] = f.Request
	j["response"] = f.Response
	return json.Marshal(j)
}