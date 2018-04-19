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
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/orijtech/callback"
)

func Example() {
	cb := &callback.Callback{
		URL:     "https://example.com/payload",
		Payload: fmt.Sprintf(`{"time_now": %q}`, time.Now().Round(time.Second)),
	}
	resp, err := cb.Do(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Response: %+v\n", resp)
}

func Example_WithServer() {
	log.SetFlags(0)

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		payload, _ := ioutil.ReadAll(r.Body)
		log.Printf("got a callback from %q\npayload: %s", r.URL, payload)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		cb := &callback.Callback{
			URL: "http://localhost:8888/callback",
			Payload: map[string]interface{}{
				"origin": r.Host,
				"time":   time.Now().Unix(),
			},
		}
		_, err := cb.Do(context.Background())

		if err == nil {
			w.Write([]byte("OK"))
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	})

	if err := http.ListenAndServe(":8888", nil); err != nil {
		log.Fatal(err)
	}
}
