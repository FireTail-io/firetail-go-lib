package logging

import (
	"encoding/json"
	"time"
)

// A batchLogger receives log entries via its Enqueue method & arranges them into batches that it then passes to its batchHandler
type batchLogger struct {
	queue         chan *LogEntry // A channel down which LogEntrys will be queued to be sent to Firetail
	maxBatchSize  int            // The maximum size of a batch in bytes
	maxLogAge     time.Duration  // The maximum age of a log item to hold onto
	batchCallback func([][]byte) // A handler that takes a batch of log entries as a slice of slices of bytes & sends them to Firetail
}

// BatchLoggerOptions is an options struct used by the NewBatchLogger constructor
type BatchLoggerOptions struct {
	MaxBatchSize  int            // The maximum size of a batch in bytes
	MaxLogAge     time.Duration  // The maximum age of a log item in a batch - once an item is older than this, the batch is passed to the callback
	LogApiKey     string         // The API key used by the default BatchCallback used to send logs to the Firetail logging API
	LogApiUrl     string         // The URL of the Firetail logging API endpoint to send log entries to
	BatchCallback func([][]byte) // An optional callback to which batches will be passed; the default callback sends logs to the Firetail logging API
}

// NewBatchLogger creates a new batchLogger with the provided options
func NewBatchLogger(options BatchLoggerOptions) *batchLogger {
	newLogger := &batchLogger{
		queue:        make(chan *LogEntry),
		maxBatchSize: options.MaxBatchSize,
		maxLogAge:    options.MaxLogAge,
	}

	newLogger.batchCallback = options.BatchCallback
	if newLogger.batchCallback == nil {
		newLogger.batchCallback = getDefaultBatchCallback(options)
	}

	go newLogger.worker()

	return newLogger
}

// Enqueue enqueues a logentry to be batched & sent to Firetail. Should normally be run in a new goroutine as it blocks until another routine receives from l.queue.
func (l *batchLogger) Enqueue(logEntry *LogEntry) {
	l.queue <- logEntry
}

// worker receives log entries via the batchLogger's queue and arranges them into batches of up to the batchLogger's maxBatchSize, and passes them to the logger's
// batchHandler when either (1) it receives a new log entry that would make the batch oversized, or (2) the oldest log entry in the current batch is older than
// the batchLogger's maxLogAge
func (l *batchLogger) worker() {
	currentBatch := [][]byte{}
	currentBatchSize := 0
	var overflowEntry *LogEntry
	var overflowEntryBytes *[]byte
	var oldestEntryCreatedAt *time.Time

	for {
		batchIsReady := false

		// Read a new entry from the queue if there's one available
		select {
		case newEntry := <-l.queue:
			// Marshal the entry to bytes...
			entryBytes, err := json.Marshal(newEntry)
			if err != nil {
				// TODO: log that we're skipping this entry?
				continue
			}

			if len(entryBytes) > l.maxBatchSize {
				// TODO: log that we're skipping this entry?
				continue
			}

			// If it's too big to add to the batch, place it in the overflow, flag the batch is ready to send & break from this case
			if len(entryBytes)+currentBatchSize > l.maxBatchSize {
				overflowEntry = newEntry
				overflowEntryBytes = &entryBytes
				batchIsReady = true
				break
			}

			// Append it to the batch & increment the currentBatchSize appropriately
			currentBatch = append(currentBatch, entryBytes)
			currentBatchSize += len(entryBytes)

			// If the new entry is older than the oldest currently in the batch, we update oldestEntryCreatedAt
			if oldestEntryCreatedAt == nil || newEntry.DateCreated < oldestEntryCreatedAt.UnixMilli() {
				createdAt := time.UnixMilli(newEntry.DateCreated)
				oldestEntryCreatedAt = &createdAt
			}
		default:
			// If there's no new entry available, just break
			break
		}

		// If the oldest entry in the currentBatch was logged long enough ago, then the currentBatch is ready to send
		if oldestEntryCreatedAt != nil && time.Since(*oldestEntryCreatedAt) > l.maxLogAge {
			batchIsReady = true
		}

		if batchIsReady {
			// Pass the batch to the batchHandler! :)
			go l.batchCallback(currentBatch)

			// Clear out the current batch & set oldestEntryCreatedAt to nil
			currentBatch = [][]byte{}
			currentBatchSize = 0
			oldestEntryCreatedAt = nil

			// If there's an overflow entry, add it to the batch immediately
			if overflowEntry != nil {
				createdAt := time.UnixMilli(overflowEntry.DateCreated)
				oldestEntryCreatedAt = &createdAt

				currentBatch = append(currentBatch, *overflowEntryBytes)
				currentBatchSize += len(*overflowEntryBytes)

				overflowEntry = nil
				overflowEntryBytes = nil
			}
		}

		// Give the CPU some time to do other things :)
		time.Sleep(1)
	}
}
