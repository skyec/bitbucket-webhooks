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

// A WebHookHandler defined the function signature for event handler callbacks.
// Implementors use type assertions to access the associated event type. The
// type in event is always a pointer to the event struct.
type WebHookHandler func(headers Headers, event interface{}) error

// WebHooks is a http.Handler that parses BitBucket webhook events, mapping them
// to the appropriate event type and calling event handlers.
type Webhook struct {

	// LogOnError is a callback called to log errors
	LogOnError func(format string, a ...interface{})

	handlers map[string]WebHookHandler
}

// New constructs a new WebHooks struct. The zero value of this struct can
// serve web hook requests but will respond with 400 Bad Request status code
// for any event that doesn't have a registered handler.
func NewWebhook() *Webhook {
	return &Webhook{
		handlers: map[string]WebHookHandler{},
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

// ServeHTTP implements the http.Handler interface, extracts the request headers,
// maps the event key to the payload type, parses the JSON payload and calls
// the registered handler.
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

// Handle is called to register a webhook handler for the expected eventKey.
func (wh *Webhook) Handle(eventKey string, handler WebHookHandler) {
	wh.handlers[eventKey] = handler
}

func (wh *Webhook) badRequest(w http.ResponseWriter, r *http.Request, msg string, p ...interface{}) {
	fmsg := fmt.Sprintf(msg, p...)
	if wh.LogOnError != nil {
		wh.LogOnError(fmsg)
	}
	http.Error(w, fmsg, http.StatusBadRequest)
}
