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

package callback_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"

	"github.com/orijtech/callback"
)

type customRT struct {
	log *bytes.Buffer
	sync.RWMutex
	maxTries int
	curTries int
}

var _ http.RoundTripper = (*customRT)(nil)

func (cr *customRT) RoundTrip(req *http.Request) (*http.Response, error) {
	blob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	cr.RLock()
	curTries, maxTries := cr.curTries, cr.maxTries
	cr.RUnlock()
	if curTries < maxTries {
		fmt.Fprintf(cr.log, "Count: %d\n", cr.curTries)
		cr.Lock()
		cr.curTries += 1
		cr.Unlock()

		// Repackage the body
		prc, pwc := io.Pipe()
		go func() {
			pwc.Write(blob)
			_ = pwc.Close()
		}()
		req.Body = prc
		return cr.RoundTrip(req)
	}

	cr.Lock()
	fmt.Fprintf(cr.log, "Final Write\n")
	cr.Unlock()

	prc, pwc := io.Pipe()
	go func() {
		pwc.Write(blob)
		_ = pwc.Close()
	}()

	resp := &http.Response{
		Body:          prc,
		StatusCode:    200,
		Status:        "200 OK",
		ContentLength: int64(len(blob)),
		Header:        make(http.Header),
	}
	resp.Header.Set("X-Log", string(cr.log.Bytes()))

	return resp, nil
}

func TestCallback(t *testing.T) {
	tests := []struct {
		payload   interface{}
		url       string
		wantBody  []byte
		wantError bool
		wantLog   string
		retries   int
	}{
		0: {
			payload: map[string]interface{}{
				"a": 12,
				"b": map[string]string{"aa": "bb"},
			},
			url:      "https://example.com",
			wantBody: []byte(`{"a":12,"b":{"aa":"bb"}}`),
			wantLog:  "Final Write\n",
		},
		1: {
			payload:  `this is the payload`,
			url:      "https://orijtech.com",
			wantBody: []byte(`this is the payload`),
			retries:  3,
			wantLog:  "Count: 0\nCount: 1\nCount: 2\nFinal Write\n",
		},
		2: {
			payload:  `""""""""""this is the payload""""""""""`,
			url:      "https://orijtech.com",
			wantBody: []byte(`""""""""""this is the payload""""""""""`),
			retries:  2,
			wantLog:  "Count: 0\nCount: 1\nFinal Write\n",
		},
	}

	stripNewline := func(b []byte) []byte {
		return bytes.TrimRight(b, "\n")
	}

	for i, tt := range tests {
		ct := new(customRT)
		ct.log = new(bytes.Buffer)
		ct.maxTries = tt.retries

		cb := &callback.Callback{
			URL:          tt.url,
			Payload:      tt.payload,
			RoundTripper: ct,
		}

		resp, err := cb.Do(context.Background())
		if err != nil {
			if !tt.wantError {
				t.Errorf("#%d: err: %v", i, err)
			}
			continue
		}
		gotLog := resp.Header.Get("X-Log")
		if tt.wantLog != gotLog {
			t.Errorf("#%d:\n\tgotLog: %q\n\twantLog:%q", i, gotLog, tt.wantLog)
		}

		var blob []byte
		if resp.Body != nil {
			blob, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("#%d ReadAll: %v", i, err)
				continue
			}
		}

		got, want := stripNewline(blob), stripNewline(tt.wantBody)
		if !bytes.Equal(got, want) {
			t.Errorf("#%d:\n\tgot= %q\n\twant=%q", i, got, want)
		}
	}
}
