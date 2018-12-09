/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/knative/pkg/cloudevents"
)

type Filter struct{}

type event struct {
	Data string `json:"data,omitEmpty"`
}

func (f *Filter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	var body []byte
	ctx, err := cloudevents.Binary.FromRequest(&body, r)

	if err != nil {
		log.Printf("Failed to parse events from the request: %s", err)
		return
	}
	log.Printf("Received Context: %+v", ctx)
	log.Printf("Received body as: %q", string(body))
	e := event{}
	err = json.Unmarshal(body, &e)
	if err != nil {
		log.Printf("Failed to unmarshal event data: %s", err)
		return
	}
	log.Printf("Received event as: %+v", e)
	// TODO: Copy the headers here...
	w.Write(body)
}

func main() {
	http.ListenAndServe(":8080", &Filter{})
}
