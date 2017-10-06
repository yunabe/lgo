package gojupyterscaffold

import "context"

type RequestHandlers interface {
	HandleKernelInfo() KernelInfo
	HandleExecuteRequest(ctx context.Context,
		req *ExecuteRequest,
		writeStream func(name, text string),
		writeDisplayData func(data *DisplayData)) *ExecuteResult
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
type DisplayData struct {
	Data      map[string]interface{} `json:"data,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Transidnt map[string]interface{} `json:"transient,omitempty"`
}
