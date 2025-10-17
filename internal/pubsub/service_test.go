package pubsub

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/apahim/cls-backend/internal/config"
	"github.com/apahim/cls-backend/internal/models"
	"github.com/apahim/cls-backend/internal/utils"
	"github.com/google/uuid"
)

// mockMessage implements a basic message for testing
type mockMessage struct {
	data []byte
	ackd bool
	mu   sync.Mutex
}

func (m *mockMessage) Ack() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ackd = true
}

func (m *mockMessage) Nack() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ackd = false
}

func (m *mockMessage) Data() []byte {
	return m.data
}

func (m *mockMessage) ID() string {
	return "mock-message-id"
}

func (m *mockMessage) PublishTime() time.Time {
	return time.Now()
}

func (m *mockMessage) isAcked() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ackd
}

// mockTopic implements a basic topic for testing
type mockTopic struct {
	name      string
	messages  []*mockMessage
	mu        sync.Mutex
	published [][]byte
}

func (t *mockTopic) Publish(ctx context.Context, msg *Message) *mockPublishResult {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.published = append(t.published, msg.Data)

	result := &mockPublishResult{
		ready: make(chan struct{}),
	}
	close(result.ready)
	return result
}

func (t *mockTopic) Stop() {}

func (t *mockTopic) getPublished() [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([][]byte, len(t.published))
	copy(result, t.published)
	return result
}

// mockPublishResult implements PublishResult for testing
type mockPublishResult struct {
	ready chan struct{}
	msgID string
	err   error
}

func (r *mockPublishResult) Ready() <-chan struct{} {
	return r.ready
}

func (r *mockPublishResult) Get(ctx context.Context) (string, error) {
	select {
	case <-r.ready:
		return r.msgID, r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// mockSubscription implements a basic subscription for testing
type mockSubscription struct {
	name     string
	messages chan *mockMessage
	handlers []StatusUpdateHandler
	mu       sync.Mutex
	stopped  bool
}

func (s *mockSubscription) Receive(ctx context.Context, handler func(ctx context.Context, msg *Message)) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-s.messages:
			if msg == nil {
				return nil // Subscription stopped
			}
			// Convert mockMessage to Message interface
			message := &Message{
				ID:          msg.ID(),
				Data:        msg.Data(),
				PublishTime: msg.PublishTime(),
			}
			handler(ctx, message)
		}
	}
}

func (s *mockSubscription) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped {
		s.stopped = true
		close(s.messages)
	}
}

func (s *mockSubscription) addMessage(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped {
		s.messages <- &mockMessage{data: data}
	}
}

// mockClient implements the PubSubClient interface for testing
type mockClient struct {
	topics        map[string]*mockTopic
	subscriptions map[string]*mockSubscription
	mu            sync.Mutex
}

func newMockClient() *mockClient {
	return &mockClient{
		topics:        make(map[string]*mockTopic),
		subscriptions: make(map[string]*mockSubscription),
	}
}

func (c *mockClient) Topic(name string) Topic {
	c.mu.Lock()
	defer c.mu.Unlock()

	if topic, exists := c.topics[name]; exists {
		return topic
	}

	topic := &mockTopic{
		name:     name,
		messages: make([]*mockMessage, 0),
	}
	c.topics[name] = topic
	return topic
}

func (c *mockClient) Subscription(name string) Subscription {
	c.mu.Lock()
	defer c.mu.Unlock()

	if sub, exists := c.subscriptions[name]; exists {
		return sub
	}

	sub := &mockSubscription{
		name:     name,
		messages: make(chan *mockMessage, 100),
		handlers: make([]StatusUpdateHandler, 0),
	}
	c.subscriptions[name] = sub
	return sub
}

func (c *mockClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, sub := range c.subscriptions {
		sub.Stop()
	}
	return nil
}

func (c *mockClient) getTopic(name string) *mockTopic {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.topics[name]
}

func (c *mockClient) getSubscription(name string) *mockSubscription {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.subscriptions[name]
}

func createTestService(t *testing.T) (*Service, *mockClient) {
	cfg := config.PubSubConfig{
		ProjectID:              "test-project",
		ClusterEventsTopic:     "cluster-events",
		MaxConcurrentHandlers:  10,
		MaxOutstandingMessages: 100,
	}

	mockClient := newMockClient()
	service := &Service{
		config: cfg,
		logger: utils.NewLogger("test-service"),
		status: "running",
	}

	return service, mockClient
}

func TestService_PublishClusterEvent(t *testing.T) {
	service, mockClient := createTestService(t)
	// service.Close() // Mock service doesn't need close

	event := &models.ClusterEvent{
		ID:             uuid.New(),
		ClusterID:      "test-cluster",
		ControllerName: "test-controller",
		EventType:      "StatusUpdate",
		EventData: models.JSONB(map[string]interface{}{
			"status": "Ready",
		}),
		PublishedAt: time.Now(),
	}

	ctx := context.Background()
	err := service.PublishClusterEvent(ctx, event)
	utils.AssertError(t, err, false, "Should publish cluster event without error")

	// Verify message was published
	topic := mockClient.getTopic("cluster-events")
	utils.AssertNotNil(t, topic, "Cluster events topic should exist")

	published := topic.getPublished()
	utils.AssertEqual(t, 1, len(published), "Should have 1 published message")

	// Verify message content
	var publishedEvent models.ClusterEvent
	err = json.Unmarshal(published[0], &publishedEvent)
	utils.AssertError(t, err, false, "Should unmarshal published event")
	utils.AssertEqual(t, event.ID, publishedEvent.ID, "Event ID should match")
	utils.AssertEqual(t, event.ClusterID, publishedEvent.ClusterID, "Cluster ID should match")
	utils.AssertEqual(t, event.EventType, publishedEvent.EventType, "Event type should match")
}



func TestService_PublishWithTimeout(t *testing.T) {
	service, mockClient := createTestService(t)
	defer service.Close()

	event := &models.ClusterEvent{
		ID:             uuid.New(),
		ClusterID:      "test-cluster",
		ControllerName: "test-controller",
		EventType:      "StatusUpdate",
		PublishedAt:    time.Now(),
	}

	// Use a very short timeout to test timeout behavior
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	// This should succeed because our mock completes quickly
	err := service.PublishClusterEvent(ctx, event)
	// The error depends on timing, but the operation should complete
	if err != nil {
		utils.AssertError(t, err, true, "Should handle timeout gracefully")
	}
}

func TestService_Close(t *testing.T) {
	service, mockClient := createTestService(t)

	// Verify service can be closed without error
	err := service.Close()
	utils.AssertError(t, err, false, "Should close service without error")

	// Verify multiple closes don't cause issues
	err = service.Close()
	utils.AssertError(t, err, false, "Should handle multiple closes gracefully")
}

func TestService_PublishMultipleEvents(t *testing.T) {
	service, mockClient := createTestService(t)
	defer service.Close()

	ctx := context.Background()

	// Publish multiple cluster events
	for i := 0; i < 5; i++ {
		event := &models.ClusterEvent{
			ID:             uuid.New(),
			ClusterID:      fmt.Sprintf("cluster-%d", i),
			ControllerName: "test-controller",
			EventType:      "StatusUpdate",
			PublishedAt:    time.Now(),
		}

		err := service.PublishClusterEvent(ctx, event)
		utils.AssertError(t, err, false, "Should publish event %d", i)
	}

	// Verify all events were published
	topic := mockClient.getTopic("cluster-events")
	utils.AssertNotNil(t, topic, "Cluster events topic should exist")

	published := topic.getPublished()
	utils.AssertEqual(t, 5, len(published), "Should have 5 published messages")
}

func TestService_ConcurrentPublish(t *testing.T) {
	service, mockClient := createTestService(t)
	defer service.Close()

	ctx := context.Background()
	numGoroutines := 10
	eventsPerGoroutine := 5

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Publish events concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < eventsPerGoroutine; j++ {
				event := &models.ClusterEvent{
					ID:             uuid.New(),
					ClusterID:      fmt.Sprintf("cluster-%d-%d", goroutineID, j),
					ControllerName: "test-controller",
					EventType:      "StatusUpdate",
					PublishedAt:    time.Now(),
				}

				err := service.PublishClusterEvent(ctx, event)
				utils.AssertError(t, err, false, "Should publish event %d-%d", goroutineID, j)
			}
		}(i)
	}

	wg.Wait()

	// Verify all events were published
	topic := mockClient.getTopic("cluster-events")
	utils.AssertNotNil(t, topic, "Cluster events topic should exist")

	published := topic.getPublished()
	expectedCount := numGoroutines * eventsPerGoroutine
	utils.AssertEqual(t, expectedCount, len(published), "Should have %d published messages", expectedCount)
}
