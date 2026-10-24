package main

import (
	"context"
	"flag"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	doorbadge "github.com/oapi-codegen/oapi-codegen/v2/examples/webhook"
)

var names = []string{
	"Alice", "Bob", "Charlie", "Diana", "Eve",
	"Frank", "Grace", "Hank", "Iris", "Jack",
}

type webhookEntry struct {
	id   uuid.UUID
	url  string
	kind doorbadge.RegisterWebhookParamsKind
}

// BadgeReader implements doorbadge.StrictServerInterface.
type BadgeReader struct {
	initiator *doorbadge.WebhookInitiator

	mu       sync.Mutex
	webhooks map[uuid.UUID]webhookEntry
}

var _ doorbadge.StrictServerInterface = (*BadgeReader)(nil)

func NewBadgeReader() *BadgeReader {
	initiator, err := doorbadge.NewWebhookInitiator()
	if err != nil {
		log.Fatalf("Failed to create webhook initiator: %v", err)
	}
	return &BadgeReader{
		initiator: initiator,
		webhooks:  make(map[uuid.UUID]webhookEntry),
	}
}

func apiError(code int, message string) doorbadge.Error {
	return doorbadge.Error{
		Code:    int32(code),
		Message: message,
	}
}

func (br *BadgeReader) RegisterWebhook(ctx context.Context, request doorbadge.RegisterWebhookRequestObject) (doorbadge.RegisterWebhookResponseObject, error) {
	if request.Body == nil {
		return doorbadge.RegisterWebhookdefaultJSONResponse{
			StatusCode: http.StatusBadRequest,
			Body:       apiError(http.StatusBadRequest, "request body is required"),
		}, nil
	}
	req := *request.Body
	kind := request.Kind

	if kind != doorbadge.EnterEvent && kind != doorbadge.ExitEvent {
		return doorbadge.RegisterWebhookdefaultJSONResponse{
			StatusCode: http.StatusBadRequest,
			Body:       apiError(http.StatusBadRequest, "Invalid webhook kind: "+string(kind)),
		}, nil
	}

	id := uuid.New()
	entry := webhookEntry{id: id, url: req.URL, kind: kind}

	br.mu.Lock()
	br.webhooks[id] = entry
	br.mu.Unlock()

	log.Printf("Registered webhook: id=%s kind=%s url=%s", id, kind, req.URL)

	return doorbadge.RegisterWebhook201JSONResponse{ID: id}, nil
}

func (br *BadgeReader) DeregisterWebhook(ctx context.Context, request doorbadge.DeregisterWebhookRequestObject) (doorbadge.DeregisterWebhookResponseObject, error) {
	id := request.ID

	br.mu.Lock()
	entry, ok := br.webhooks[id]
	delete(br.webhooks, id)
	br.mu.Unlock()

	if !ok {
		return doorbadge.DeregisterWebhookdefaultJSONResponse{
			StatusCode: http.StatusNotFound,
			Body:       apiError(http.StatusNotFound, "Webhook not found: "+id.String()),
		}, nil
	}

	log.Printf("Deregistered webhook: id=%s kind=%s url=%s", id, entry.kind, entry.url)
	return doorbadge.DeregisterWebhook204Response{}, nil
}

// generateEvents picks a random name and event kind every second and notifies webhooks.
func (br *BadgeReader) generateEvents(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			name := names[rand.IntN(len(names))]
			kind := doorbadge.EnterEvent
			if rand.IntN(2) == 0 {
				kind = doorbadge.ExitEvent
			}

			person := doorbadge.Person{Name: name}

			br.mu.Lock()
			targets := make([]webhookEntry, 0)
			for _, entry := range br.webhooks {
				if entry.kind == kind {
					targets = append(targets, entry)
				}
			}
			br.mu.Unlock()

			if len(targets) == 0 {
				continue
			}

			log.Printf("Event: %s %s (%d webhooks)", kind, name, len(targets))

			for _, target := range targets {
				var resp *http.Response
				var err error

				switch kind {
				case doorbadge.EnterEvent:
					resp, err = br.initiator.EnterEvent(ctx, target.url, person)
				case doorbadge.ExitEvent:
					resp, err = br.initiator.ExitEvent(ctx, target.url, person)
				}

				if err != nil {
					log.Printf("Webhook %s failed: %v — removing", target.id, err)
					br.mu.Lock()
					delete(br.webhooks, target.id)
					br.mu.Unlock()
					continue
				}
				_ = resp.Body.Close()

				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					log.Printf("Webhook %s returned %d — removing", target.id, resp.StatusCode)
					br.mu.Lock()
					delete(br.webhooks, target.id)
					br.mu.Unlock()
				}
			}
		}
	}
}

func main() {
	port := flag.String("port", "8080", "Port for HTTP server")
	flag.Parse()

	reader := NewBadgeReader()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go reader.generateEvents(ctx)

	mux := http.NewServeMux()
	doorbadge.HandlerFromMux(doorbadge.NewStrictHandler(reader, nil), mux)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		mux.ServeHTTP(w, r)
	})

	addr := net.JoinHostPort("0.0.0.0", *port)
	log.Printf("Door Badge Reader server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}
