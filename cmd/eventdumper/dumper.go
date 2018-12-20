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
	"context"
	"log"
	"net/http"

	"github.com/knative/pkg/cloudevents"
)

func myFunc(ctx context.Context, e string) error {
	// Extract only the Cloud Context from the context because that's
	// all we care about for this example and the entire context is toooooo much...
	ec := cloudevents.FromContext(ctx)
	if ec != nil {
		log.Printf("Received Cloud Event Context as: %+v", *ec)
	} else {
		log.Printf("No Cloud Event Context found")
	}
	log.Printf("Received event data as: %+v", e)
	return nil
}

func main() {
	m := cloudevents.NewMux()
	err := m.Handle("GoogleCloudScheduler", myFunc)
	if err != nil {
		log.Fatalf("Failed to create handler %s", err)
	}
	http.ListenAndServe(":8080", m)
}
