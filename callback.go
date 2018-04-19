// Copyright 2017 orijtech, Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	"go.opencensus.io/plugin/ochttp"

	"github.com/sethgrid/pester"
)

type Callback struct {
	sync.RWMutex

	URL     string      `json:"url,omitempty"`
	Payload interface{} `json:"payload,omitempty"`

	RoundTripper http.RoundTripper
}

var defaultClient = exponentialBackoffClient(5)

func exponentialBackoffClient(retries int) httpDoer {
	client := pester.New()
	if retries < 1 {
		retries = 5
	}
	client.KeepLog = true
	client.MaxRetries = retries
	client.Transport = &ochttp.Transport{}

	return client
}

var errEmptyCallbackURL = errors.New("empty callback URL")

func (cb *Callback) Validate() error {
	cbURL := strings.TrimSpace(cb.URL)
	if cbURL == "" {
		return errEmptyCallbackURL
	}
	cb.URL = cbURL
	return nil
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func (cb *Callback) Do(ctx context.Context) (*http.Response, error) {
	if err := cb.Validate(); err != nil {
		return nil, err
	}

	var doer httpDoer = defaultClient
	if rt := cb.RoundTripper; rt != nil {
		doer = &http.Client{Transport: &ochttp.Transport{Base: rt}}
	}

	var body io.Reader = nil
	isJSONEncoded := false
	if payload := cb.Payload; payload != nil {
		buf := new(bytes.Buffer)

		switch typ := payload.(type) {
		case []byte:
			if _, err := buf.Write(typ); err != nil {
				return nil, err
			}
		case string:
			if _, err := io.WriteString(buf, typ); err != nil {
				return nil, err
			}
		default:
			jsonEncoder := json.NewEncoder(buf)
			jsonEncoder.SetEscapeHTML(false)
			if err := jsonEncoder.Encode(payload); err != nil {
				return nil, err
			}
			// One thing we have to watch for is that after
			// invoking Encode, it adds a newline character
			// which we want to strip to keep a uniform final
			// output as if it were encoded with json.Marshal.
			isJSONEncoded = true
		}

		// Now set the body for reading
		body = buf
	}

	req, err := http.NewRequest("POST", cb.URL, body)
	if err != nil {
		return nil, err
	}
	if isJSONEncoded {
		req.Header.Set("Content-Type", "application/json")
	}
	req = req.WithContext(ctx)

	return doer.Do(req)
}
