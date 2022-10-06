package logging

import (
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var batchLogger *BatchLogger

func SetupLogger(batchChannel chan *[][]byte, maxBatchSize int, maxLogAge time.Duration) {
	batchLogger = NewBatchLogger(maxBatchSize, maxLogAge, "")

	// Replace the batchHandler with a custom one to throw the batches into a queue that we can receive from for testing
	batchLogger.batchHandler = func(b [][]byte) error {
		batchChannel <- &b
		return nil
	}
}

func TestOldLogIsSentImmediately(t *testing.T) {
	batchChannel := make(chan *[][]byte, 2)
	SetupLogger(batchChannel, 1024^3, time.Minute)

	// Create a test log entry & enqueue it
	testLogEntry := LogEntry{
		DateCreated: 0,
	}
	go batchLogger.Enqueue(&testLogEntry)

	// There should then be a batch in the channel for us to receive
	batch := <-batchChannel

	// Channel should be empty now, as it should only have had one batch in it
	assert.Equal(t, 0, len(batchChannel))

	// Marshal the testLogEntry to get our expected bytes
	expectedLogEntryBytes, err := json.Marshal(testLogEntry)
	require.Nil(t, err)

	// Assert the batch had one log entry in it, which matches our test log entry's bytes
	require.Equal(t, 1, len(*batch))
	assert.Equal(t, expectedLogEntryBytes, (*batch)[0])
}

func TestBatchesDoNotExceedMaxSize(t *testing.T) {
	const TestLogEntryCount = 10000
	const MaxBatchSize = 1024 * 512 // 512KB in Bytes

	// Buffer our batchChannel with TestLogEntryCount spaces (worst case, each entry ends up in its own batch)
	batchChannel := make(chan *[][]byte, TestLogEntryCount)
	SetupLogger(batchChannel, MaxBatchSize, time.Second)

	// Create a bunch of test entries
	testLogEntries := []*LogEntry{}
	for i := 0; i < TestLogEntryCount; i++ {
		testLogEntries = append(
			testLogEntries,
			&LogEntry{
				DateCreated:   time.Now().UnixMilli(),
				ExecutionTime: rand.Float64() * 100,
				Request: Request{
					Body: "{\"description\":\"This is a test request body\"}",
					Headers: map[string][]string{
						"Content-Type": {"application/json"},
					},
					HTTPProtocol: HTTP2,
					IP:           "8.8.8.8",
					Method:       "POST",
					URI:          "This isn't a real URI",
					Resource:     "This isn't a real resource",
				},
				Response: Response{
					// Create response bodies of varying size so the batch sizes aren't all the same
					Body: strings.Repeat("a", rand.Intn(10000)),
					Headers: map[string][]string{
						"Content-Type": {"text/plain"},
					},
					StatusCode: 200,
				},
			},
		)
	}

	// Enqueue them all
	for _, logEntry := range testLogEntries {
		go batchLogger.Enqueue(logEntry)
	}

	// Keep reading until we've seen all the log entries
	logEntriesReceived := 0
	for logEntriesReceived < TestLogEntryCount {
		batch := <-batchChannel

		logEntriesReceived += len(*batch)

		batchSize := 0
		for _, logBytes := range *batch {
			batchSize += len(logBytes)
		}

		assert.GreaterOrEqual(t, MaxBatchSize, batchSize)
	}

	// We should receive exactly the same number of log entries as we put in
	assert.Equal(t, TestLogEntryCount, logEntriesReceived)

	// There should also be no batches left
	assert.Equal(t, 0, len(batchChannel))
}
