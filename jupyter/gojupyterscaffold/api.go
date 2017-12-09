package gojupyterscaffold

import "context"

type RequestHandlers interface {
	HandleKernelInfo() KernelInfo
	// HandleExecuteRequest handles execute_request.
	// writeStream sends stdout/stderr texts and writeDisplayData sends display_data
	// (or update_display_data if update is true) to the client.
	HandleExecuteRequest(ctx context.Context,
		req *ExecuteRequest,
		writeStream func(name, text string),
		writeDisplayData func(data *DisplayData, update bool)) *ExecuteResult
	HandleComplete(req *CompleteRequest) *CompleteReply
	HandleInspect(req *InspectRequest) *InspectReply
	// http://jupyter-client.readthedocs.io/en/latest/messaging.html#code-completeness
	HandleIsComplete(req *IsCompleteRequest) *IsCompleteReply
}

type KernelInfo struct {
	ProtocolVersion       string             `json:"protocol_version"`
	Implementation        string             `json:"implementation"`
	ImplementationVersion string             `json:"implementation_version"`
	LanguageInfo          KernelLanguageInfo `json:"language_info"`
	Banner                string             `json:"banner"`
}

type KernelLanguageInfo struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	Mimetype      string `json:"mimetype"`
	FileExtension string `json:"file_extension"`
}

type ExecuteRequest struct {
	Code         string `json:"code"`
	Silent       bool   `json:"silent"`
	StoreHistory bool   `json:"store_history"`
	AllowStdin   bool   `json:"allow_stdin"`
	StopOnError  bool   `json:"stop_on_error"`
}

// See http://jupyter-client.readthedocs.io/en/latest/messaging.html#introspection
type InspectRequest struct {
	Code      string `json:"code"`
	CursorPos int    `json:"cursor_pos"`
	// The level of detail desired.  In IPython, the default (0) is equivalent to typing
	// 'x?' at the prompt, 1 is equivalent to 'x??'.
	// The difference is up to kernels, but in IPython level 1 includes the source code
	// if available.
	DetailLevel int `json:"detail_level"`
}

// See http://jupyter-client.readthedocs.io/en/latest/messaging.html#introspection
type InspectReply struct {
	// 'ok' if the request succeeded or 'error', with error information as in all other replies.
	Status string `json:"status"`
	// found should be true if an object was found, false otherwise
	Found bool `json:"found"`
	// data can be empty if nothing is found
	Data map[string]interface{} `json:"data,omitempty"`
}

// http://jupyter-client.readthedocs.io/en/latest/messaging.html#completion
type CompleteRequest struct {
	// The code context in which completion is requested
	// this may be up to an entire multiline cell, such as
	// 'foo = a.isal'
	Code string `json:"code"`
	// The cursor position within 'code' (in unicode characters) where completion is requested
	CursorPos int `json:"cursor_pos"`
}

type CompleteReply struct {
	// The list of all matches to the completion request, such as
	// ['a.isalnum', 'a.isalpha'] for the above example.
	Matches []string `json:"matches"`

	// The range of text that should be replaced by the above matches when a completion is accepted.
	// typically cursor_end is the same as cursor_pos in the request.
	CursorStart int `json:"cursor_start"`
	CursorEnd   int `json:"cursor_end"`

	// 'metadata' is omitted

	// status should be 'ok' unless an exception was raised during the request,
	// in which case it should be 'error', along with the usual error message content
	// in other messages.
	Status string `json:"status"`
}

// http://jupyter-client.readthedocs.io/en/latest/messaging.html#execution-results
type ExecuteResult struct {
	Status         string `json:"status"`
	ExecutionCount int    `json:"execution_count,omitempty"`
	// data and metadata are omitted because they are covered by DisplayData.
}

// DisplayData represents display_data defined in http://jupyter-client.readthedocs.io/en/latest/messaging.html#display-data
//
// omitempty is important not to output "metadata: null", which results in
// Failed validating u'type' in display_data[u'properties'][u'metadata'] in Jupyter notebook.
//
// c.f.
// The definition of MIME-type and the right format of value:
// Search for "MIME_HTML"
// https://github.com/jupyter/notebook/blob/master/notebook/static/notebook/js/outputarea.js
// A special handling of "application/json"
// https://github.com/jupyter/jupyter_client/blob/master/jupyter_client/adapter.py
type DisplayData struct {
	Data      map[string]interface{} `json:"data,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Transient map[string]interface{} `json:"transient,omitempty"`
}

// http://jupyter-client.readthedocs.io/en/latest/messaging.html#code-completeness
type IsCompleteRequest struct {
	// The code entered so far as a multiline string
	Code string `json:"code"`
}

// http://jupyter-client.readthedocs.io/en/latest/messaging.html#code-completeness
type IsCompleteReply struct {
	// One of 'complete', 'incomplete', 'invalid', 'unknown'
	Status string `json:"status"`
	// If status is 'incomplete', indent should contain the characters to use
	// to indent the next line. This is only a hint: frontends may ignore it
	// and use their own autoindentation rules. For other statuses, this
	// field does not exist.
	Indent string `json:"indent"`
}
