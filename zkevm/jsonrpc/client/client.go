package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/tenderly/zkevm-erigon/zkevm/jsonrpc/types"
)

// Client defines typed wrappers for the zkEVM RPC API.
type Client struct {
	url string
}

// NewClient creates an instance of client
func NewClient(url string) *Client {
	return &Client{
		url: url,
	}
}

// JSONRPCCall executes a 2.0 JSON RPC HTTP Post Request to the provided URL with
// the provided method and parameters, which is compatible with the Ethereum
// JSON RPC Server.
func JSONRPCCall(url, method string, parameters ...interface{}) (types.Response, error) {
	const jsonRPCVersion = "2.0"

	params, err := json.Marshal(parameters)
	if err != nil {
		return types.Response{}, err
	}

	req := types.Request{
		JSONRPC: jsonRPCVersion,
		ID:      float64(1),
		Method:  method,
		Params:  params,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return types.Response{}, err
	}

	reqBodyReader := bytes.NewReader(reqBody)
	httpReq, err := http.NewRequest(http.MethodPost, url, reqBodyReader)
	if err != nil {
		return types.Response{}, err
	}

	httpReq.Header.Add("Content-type", "application/json")

	httpRes, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return types.Response{}, err
	}

	if httpRes.StatusCode != http.StatusOK {
		return types.Response{}, fmt.Errorf("Invalid status code, expected: %v, found: %v", http.StatusOK, httpRes.StatusCode)
	}

	resBody, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return types.Response{}, err
	}
	defer httpRes.Body.Close()

	var res types.Response
	err = json.Unmarshal(resBody, &res)
	if err != nil {
		return types.Response{}, err
	}

	return res, nil
}

func JSONRPCBatchCall(url string, methods []string, parameterGroups ...[]interface{}) ([]types.Response, error) {
	const jsonRPCVersion = "2.0"

	if len(methods) != len(parameterGroups) {
		return nil, fmt.Errorf("methods and parameterGroups must have the same length")
	}

	var batchRequest []types.Request

	for i, method := range methods {
		params, err := json.Marshal(parameterGroups[i])
		if err != nil {
			return nil, err
		}

		req := types.Request{
			JSONRPC: jsonRPCVersion,
			ID:      float64(i + 1),
			Method:  method,
			Params:  params,
		}

		batchRequest = append(batchRequest, req)
	}

	reqBody, err := json.Marshal(batchRequest)
	if err != nil {
		return nil, err
	}

	reqBodyReader := bytes.NewReader(reqBody)
	httpReq, err := http.NewRequest(http.MethodPost, url, reqBodyReader)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Add("Content-type", "application/json")

	httpRes, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpRes.Body.Close()

	if httpRes.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid status code, expected: %v, found: %v", http.StatusOK, httpRes.StatusCode)
	}

	resBody, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return nil, err
	}

	var batchResponse []types.Response
	err = json.Unmarshal(resBody, &batchResponse)
	if err != nil {
		return nil, err
	}

	return batchResponse, nil
}
