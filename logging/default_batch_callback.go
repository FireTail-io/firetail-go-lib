package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"log"
)

func getDefaultBatchCallback(options BatchLoggerOptions) func([][]byte) {
	sendBatch := func(batchBytes [][]byte) error {
		log.Println("SENDING BATCH")

		reqBytes := []byte{}
		for _, entry := range batchBytes {
			reqBytes = append(reqBytes, entry...)
			reqBytes = append(reqBytes, '\n')
		}

		log.Println("reqBODY: ", string(bytes.NewBuffer(reqBytes))

		req, err := http.NewRequest("POST", options.LogApiUrl, bytes.NewBuffer(reqBytes))
		if err != nil {
			return err
		}

		req.Header.Set("x-ft-api-key", options.LogApiKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}

		var res map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&res)
		if res["message"] != "success" {
			return errors.New(fmt.Sprintf("got err response from firetail api: %v", res))
		}

		log.Println("respBODY: ", resp.Body)

		return nil
	}

	return func(batch [][]byte) {
		// If there's no log API url or log API key set then we can't log, so just return
		if options.LogApiUrl == "" || options.LogApiKey == "" {
			return
		}

		var err error
		retries := 0
		for {
			err = sendBatch(batch)
			retries++
			// If sendBatch succeeded, or we've had 3 retries, we give up
			if err == nil || retries >= 3 {
				break
			}
		}
	}
}
