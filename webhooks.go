// This bitbucket package is for creating Bitbucket webhooks.
// See the Bitbucket docs for the events and payloads: https://confluence.atlassian.com/bitbucket/manage-webhooks-735643732.html.
// This package provieds all the types needed to parse the payloads and a
// HTTPHandler that parses the webhook requests and calls registered event callbacks.
//
// Most JSON types were autogenerated using http://mholt.github.io/json-to-go/
// then modified by hand to remove duplication.
package bitbucket

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
)

// Headers is a map that contains the event payload headers set by BitBucket.
// See: https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html#EventPayloads-HTTPHeaders
type Headers map[string]string

// A WebhookHandler defined the function signature for event handler callbacks.
// Use type assertions to access the associated event type. The type of event is
// always a pointer to the event struct. WebhookHandlers normally return nil
// but can return an error which triggers a 400 Bad Request response.
type WebhookHandler func(headers Headers, event interface{}) error

// Webhook is a http.Handler that parses BitBucket webhook events, mapping them
// to the appropriate event type and calling event handlers.
type Webhook struct {

	// LogOnError is an optional callback called when logging errors
	LogOnError func(format string, a ...interface{})

	handlers map[string]WebhookHandler
}

// NewWebhook constructs a new Webhook.
func NewWebhook() *Webhook {
	return &Webhook{
		handlers: map[string]WebhookHandler{},
	}
}

// map of webhook events to the payload type
var eventTypeMap = map[string]interface{}{
	"repo:push":                    RepoPushEvent{},
	"repo:fork":                    RepoForkEvent{},
	"repo:commit_comment_created":  RepoCommitCommentCreatedEvent{},
	"repo:commit_status_created":   RepoCommitStatusCreatedEvent{},
	"repo:commit_status_updated":   RepoCommitStatusUpdatedEvent{},
	"issue:created":                IssueCreatedEvent{},
	"issue:updated":                IssueUpdatedEvent{},
	"issue:comment_created":        IssueCommentCreatedEvent{},
	"pullrequest:created":          PullRequestCreatedEvent{},
	"pullrequest:updated":          PullRequestUpdatedEvent{},
	"pullrequest:approved":         PullRequestApprovedEvent{},
	"pullrequest:unapproved":       PullRequestApprovalRemovedEvent{},
	"pullrequest:fulfilled":        PullRequestMergedEvent{},
	"pullrequest:rejected":         PullRequestDeclinedEvent{},
	"pullrequest:comment_created":  PullRequestCommentCreatedEvent{},
	"pullrequest:comment_updated":  PullRequestCommentUpdatedEvent{},
	"pull_request:comment_deleted": PullRequestCommentDeletedEvent{},
}

// ServeHTTP implements the http.Handler interface. It extracts the request
// headers, maps the event key to the correct payload event type, parses the
// JSON payload and calls the registered WebHookHandler passing the headers and
// eventy type. A 400 Bad Request response is sent for any request made to
// an event that doesn't have a registered handler.
func (wh *Webhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	headers := Headers{}
	for _, header := range []string{"X-Event-Key"} {
		headers[header] = r.Header.Get(header)
	}

	eventKey := headers["X-Event-Key"]
	if eventKey == "" {
		wh.badRequest(w, r, "Missing X-Event-Key")
		return
	}

	handler, ok := wh.handlers[eventKey]
	if !ok {
		wh.badRequest(w, r, "No handler for the event key: %s", eventKey)
		return
	}

	t, ok := eventTypeMap[eventKey]
	if !ok {
		wh.badRequest(w, r, "Unsupported event key type: %s", eventKey)
		return
	}

	event := reflect.New(reflect.TypeOf(t)).Elem().Addr().Interface()
	err := json.NewDecoder(r.Body).Decode(event)
	if err != nil {
		log.Printf("Error parsing the body: %s", err)
		http.Error(w, "Read error: "+err.Error(), http.StatusBadRequest)
		return
	}

	err = handler(headers, event)
	if err != nil {
		wh.badRequest(w, r, "Error handling the event: %s", err)
		return
	}

}

// Handle is called to register a webhook handler for the expected eventKey. See
// the Bitbucket docs for all the possible event keys.
func (wh *Webhook) Handle(eventKey string, handler WebhookHandler) {
	wh.handlers[eventKey] = handler
}

func (wh *Webhook) badRequest(w http.ResponseWriter, r *http.Request, msg string, p ...interface{}) {
	fmsg := fmt.Sprintf(msg, p...)
	if wh.LogOnError != nil {
		wh.LogOnError(fmsg)
	}
	http.Error(w, fmsg, http.StatusBadRequest)
}
