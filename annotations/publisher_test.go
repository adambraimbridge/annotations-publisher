package annotations

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/husobee/vestigo"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
)

func TestPublish(t *testing.T) {
	uuid := uuid.New()
	server := startMockServer(t, uuid, true, true)
	defer server.Close()

	publisher := NewPublisher("originSystemID", server.URL+"/notify", "user:pass", server.URL+"/__gtg")

	err := publisher.Publish(uuid, make(map[string]interface{}))
	assert.NoError(t, err)
}

func TestPublishFailsToMarshalBodyToJSON(t *testing.T) {
	publisher := NewPublisher("originSystemID", "/notify", "user:pass", "/__gtg")

	body := make(map[string]interface{})
	body["dodgy!"] = func() {}
	err := publisher.Publish("a-valid-uuid", body)
	assert.EqualError(t, err, "json: unsupported type: func()")
}

func TestPublishFailsInvalidURL(t *testing.T) {
	publisher := NewPublisher("originSystemID", ":#", "user:pass", "/__gtg")

	body := make(map[string]interface{})
	err := publisher.Publish("a-valid-uuid", body)
	assert.EqualError(t, err, "parse :: missing protocol scheme")
}

func TestPublishRequestFailsServerUnavailable(t *testing.T) {
	publisher := NewPublisher("originSystemID", "/publish", "user:pass", "/__gtg")

	body := make(map[string]interface{})
	err := publisher.Publish("a-valid-uuid", body)
	assert.EqualError(t, err, "Post /publish: unsupported protocol scheme \"\"")
}

func TestPublishRequestUnsuccessful(t *testing.T) {
	uuid := uuid.New()
	server := startMockServer(t, uuid, false, true)
	defer server.Close()

	publisher := NewPublisher("originSystemID", server.URL+"/notify", "user:pass", server.URL+"/__gtg")

	body := make(map[string]interface{})
	err := publisher.Publish(uuid, body)
	assert.EqualError(t, err, fmt.Sprintf("Publish to %v/notify returned a 503 status code", server.URL))
}

func TestPublisherEndpoint(t *testing.T) {
	publisher := NewPublisher("originSystemID", "/publish", "user:pass", "/__gtg")
	assert.Equal(t, "/publish", publisher.Endpoint())
}

func TestPublisherAuthIsInvalid(t *testing.T) {
	publisher := NewPublisher("originSystemID", "/publish", "user", "/__gtg")

	body := make(map[string]interface{})
	err := publisher.Publish("a-valid-uuid", body)
	assert.EqualError(t, err, "Invalid auth configured")

	// Now check for too many ':'s
	publisher = NewPublisher("originSystemID", "/publish", "user:pass:anotherPass", "/__gtg")

	err = publisher.Publish("a-valid-uuid", body)
	assert.EqualError(t, err, "Invalid auth configured")
}

func TestPublisherGTG(t *testing.T) {
	server := startMockServer(t, "", true, true)
	defer server.Close()

	publisher := NewPublisher("originSystemID", "publishEndpoint", "user:pass", server.URL+"/__gtg")
	err := publisher.GTG()
	assert.NoError(t, err)
}

func TestPublisherGTGFails(t *testing.T) {
	server := startMockServer(t, "", true, false)
	defer server.Close()

	publisher := NewPublisher("originSystemID", "publishEndpoint", "user:pass", server.URL+"/__gtg")
	err := publisher.GTG()
	assert.EqualError(t, err, fmt.Sprintf("GTG %v returned a %v status code", server.URL+"/__gtg", http.StatusServiceUnavailable))
}

func TestPublisherGTGDoRequestFails(t *testing.T) {
	publisher := NewPublisher("originSystemID", "publishEndpoint", "user:pass", "/__gtg")
	err := publisher.GTG()
	assert.EqualError(t, err, "Get /__gtg: unsupported protocol scheme \"\"")
}

func TestPublisherGTGInvalidURL(t *testing.T) {
	publisher := NewPublisher("originSystemID", "publishEndpoint", "user:pass", ":#")
	err := publisher.GTG()
	assert.EqualError(t, err, "parse :: missing protocol scheme")
}

func startMockServer(t *testing.T, uuid string, publishOk bool, gtgOk bool) *httptest.Server {
	r := vestigo.NewRouter()
	r.Get("/__gtg", func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		assert.Equal(t, "PAC annotations-publisher", userAgent)

		if !gtgOk {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})

	r.Post("/notify", func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		assert.Equal(t, "PAC annotations-publisher", userAgent)

		contentType := r.Header.Get("Content-Type")
		assert.Equal(t, "application/json", contentType)

		originSystemID := r.Header.Get("X-Origin-System-Id")
		assert.Equal(t, "originSystemID", originSystemID)

		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "user", user)
		assert.Equal(t, "pass", pass)

		dec := json.NewDecoder(r.Body)
		data := make(map[string]interface{})
		err := dec.Decode(&data)
		assert.NoError(t, err)

		bodyUUID, ok := data["uuid"]
		assert.True(t, ok)
		assert.Equal(t, uuid, bodyUUID.(string))

		if !publishOk {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
	return httptest.NewServer(r)
}
