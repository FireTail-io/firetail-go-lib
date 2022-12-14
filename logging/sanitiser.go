package logging

// DefaultSanitiserOptions is an options struct for the default sanitiser provided with Firetail.
type SanitiserOptions struct {
	// RequestHeadersMask is a map of header names (lower cased) to HeaderMask values, which can be used to control the request headers reported to Firetail
	RequestHeadersMask map[string]HeaderMask

	// RequestHeadersMaskStrict is an optional flag which, if set to true, will configure the Firetail middleware to only report request headers explicitly described in the RequestHeadersMask
	RequestHeadersMaskStrict bool

	// ResponseHeadersMask is a map of header names (lower cased) to HeaderMask values, which can be used to control the response headers reported to Firetail
	ResponseHeadersMask map[string]HeaderMask

	// ResponseHeadersMaskStrict is an optional flag which, if set to true, will configure the Firetail middleware to only report response headers explicitly described in the ResponseHeadersMask
	ResponseHeadersMaskStrict bool

	// RequestSanitisationCallback is an optional callback which is given the request body as bytes & returns a stringified request body which
	// is then logged to Firetail. This is useful for writing custom logic to redact any sensitive data from your request bodies before it is logged
	// in Firetail.
	RequestSanitisationCallback func(string) string

	// ResponseSanitisationCallback is an optional callback which is given the response body as bytes & returns a stringified response body which
	// is then logged to Firetail. This is useful for writing custom logic to redact any sensitive data from your response bodies before it is logged
	// in Firetail.
	ResponseSanitisationCallback func(string) string
}

func DefaultSanitiser() func(LogEntry) LogEntry {
	// TODO: Create sensible defaults here.
	return GetSanitiser(SanitiserOptions{
		RequestHeadersMask: map[string]HeaderMask{
			"set-cookie":    HashHeaderValues,
			"cookie":        HashHeaderValues,
			"authorization": HashHeaderValues,
			"x-api-key":     HashHeaderValues,
			"token":         HashHeaderValues,
			"api-token":     HashHeaderValues,
			"api-key":       HashHeaderValues,
		},
	})
}

func GetSanitiser(options SanitiserOptions) func(LogEntry) LogEntry {
	// Fill in zero values for nil options
	if options.RequestHeadersMask == nil {
		options.ResponseHeadersMask = map[string]HeaderMask{}
	}
	if options.ResponseHeadersMask == nil {
		options.ResponseHeadersMask = map[string]HeaderMask{}
	}
	if options.RequestSanitisationCallback == nil {
		options.RequestSanitisationCallback = func(s string) string { return s }
	}
	if options.ResponseSanitisationCallback == nil {
		options.ResponseSanitisationCallback = func(s string) string { return s }
	}

	return func(logEntry LogEntry) LogEntry {
		// If there's a request headers or response headers mask, apply them...
		if options.RequestHeadersMask != nil {
			logEntry.Request.Headers = MaskHeaders(
				logEntry.Request.Headers,
				options.RequestHeadersMask,
				options.RequestHeadersMaskStrict,
			)
		}
		if options.ResponseHeadersMask != nil {
			logEntry.Response.Headers = MaskHeaders(
				logEntry.Response.Headers,
				options.ResponseHeadersMask,
				options.ResponseHeadersMaskStrict,
			)
		}

		// If theres a request or response sanitisation callback, apply them...
		if options.RequestSanitisationCallback != nil {
			logEntry.Request.Body = options.RequestSanitisationCallback(logEntry.Request.Body)
		}
		if options.ResponseSanitisationCallback != nil {
			logEntry.Response.Body = options.ResponseSanitisationCallback(logEntry.Response.Body)
		}

		return logEntry
	}
}
