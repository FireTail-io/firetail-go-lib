package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"fmt"
)

func getDefaultBatchCallback(options BatchLoggerOptions) func([][]byte) {
	sendBatch := func(batchBytes [][]byte) error {
                reqBytes := []byte{}
                for _, entry := range batchBytes {
                        reqBytes = append(reqBytes, entry...)
                        reqBytes = append(reqBytes, '\n')
                }

		url := strings.TrimSpace(options.LogApiUrl)

                req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBytes))
		if err != nil {
                        return errors.New(fmt.Sprintf("request error: %v", err))
		}

		apiKey := strings.TrimSpace(options.LogApiKey)

		req.Header.Set("x-ft-api-key", apiKey)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
		        return errors.New(fmt.Sprintf("response error: %v", err))
		}

		defer resp.Body.Close()

		var res map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&res)
		if res["message"] != "success" {
			return errors.New(fmt.Sprintf("got err response from firetail api: %v", res))
		}

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
