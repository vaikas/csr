/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package receiveadapter

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/google/uuid"
	"github.com/knative/pkg/cloudevents"
)

const (
	EventType   = "GoogleCloudScheduler"
	EventSource = "GCPCloudScheduler"
)

// CloudSchedulerReceiveAdapter converts incoming Cloud Scheduler events to
// CloudEvents and then sends them to the specified Sink
type CloudSchedulerReceiveAdapter struct {
	Sink   string
	Client *http.Client
}

func (ra *CloudSchedulerReceiveAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if reqBytes, err := httputil.DumpRequest(r, true); err == nil {
		log.Printf("Message Dumper received a message: %+v", string(reqBytes))
		w.Write(reqBytes)
		ra.postMessage(reqBytes, extractEventID(r))

	} else {
		log.Printf("Error dumping the request: %+v :: %+v", err, r)
	}
}

func extractEventID(r *http.Request) string {
	eventIDHeaders, ok := r.Header["X-Request-Id"]
	if ok {
		return string(eventIDHeaders[0])
	}
	if uuid, err := uuid.NewRandom(); err == nil {
		return uuid.String()
	}
	return ""
}

func (ra *CloudSchedulerReceiveAdapter) postMessage(payload interface{}, eventID string) error {
	ctx := cloudevents.EventContext{
		CloudEventsVersion: cloudevents.CloudEventsVersion,
		EventType:          EventType,
		EventID:            eventID,
		EventTime:          time.Now(),
		Source:             EventSource,
	}
	req, err := cloudevents.Binary.NewRequest(ra.Sink, payload, ctx)
	if err != nil {
		log.Printf("Failed to marshal the message: %+v : %s", payload, err)
		return err
	}

	log.Printf("Posting to %q", ra.Sink)
	client := ra.Client
	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// TODO: in general, receive adapters may have to be able to retry for error cases.
		log.Printf("response Status: %s", resp.Status)
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("response Body: %s", string(body))
	}
	return nil
}
