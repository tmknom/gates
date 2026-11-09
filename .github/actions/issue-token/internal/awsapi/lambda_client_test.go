package awsapi_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/awsapi"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/communication"
)

const (
	testFunctionName = "gates-function"
	testPayload      = `{"version":1}`
)

func TestLambdaClient_Invoke_SendsSignedRequestWithRequestResponse(t *testing.T) {
	t.Parallel()

	got := make(chan capturedRequest, 1)
	client := newLambdaClient(t, func(w http.ResponseWriter, r *http.Request) {
		got <- capture(r)
		_, _ = w.Write([]byte(`{}`))
	}, testCredentials())

	if _, err := client.Invoke(t.Context(), testFunctionName, []byte(testPayload)); err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}

	req := received(t, got)
	if req.method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.method)
	}
	if want := "/2015-03-31/functions/" + testFunctionName + "/invocations"; req.path != want {
		t.Errorf("path = %q, want %q", req.path, want)
	}
	if auth, want := req.header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential="+testAccessKeyID+"/"; !strings.HasPrefix(auth, want) {
		t.Errorf("Authorization = %q, want prefix %q", auth, want)
	}
	if auth, want := req.header.Get("Authorization"), "/"+testRegion+"/lambda/"; !strings.Contains(auth, want) {
		t.Errorf("Authorization = %q, want it to contain %q", auth, want)
	}
	if v := req.header.Get("X-Amz-Security-Token"); v != testSessionToken {
		t.Errorf("X-Amz-Security-Token = %q, want %q", v, testSessionToken)
	}
	if v := req.header.Get("X-Amz-Invocation-Type"); v != "RequestResponse" {
		t.Errorf("X-Amz-Invocation-Type = %q, want RequestResponse", v)
	}
	if v := req.header.Values("X-Amz-Log-Type"); len(v) != 0 {
		t.Errorf("X-Amz-Log-Type = %v, want it absent", v)
	}
	if req.body != testPayload {
		t.Errorf("body = %q, want %q", req.body, testPayload)
	}
}

func TestLambdaClient_Invoke_ReturnsInvokeResult(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		status        int
		functionError string
		want          awsapi.InvokeResult
	}{
		"200 without FunctionError": {
			status: http.StatusOK,
			want:   awsapi.InvokeResult{StatusCode: 200, FunctionError: "", Payload: []byte(`{"ok":true}`)},
		},
		"200 with FunctionError is not an error": {
			status:        http.StatusOK,
			functionError: "Unhandled",
			want:          awsapi.InvokeResult{StatusCode: 200, FunctionError: "Unhandled", Payload: []byte(`{"ok":true}`)},
		},
		"202 is not an error": {
			status: http.StatusAccepted,
			want:   awsapi.InvokeResult{StatusCode: 202, FunctionError: "", Payload: []byte(`{"ok":true}`)},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := newLambdaClient(t, func(w http.ResponseWriter, _ *http.Request) {
				if tt.functionError != "" {
					w.Header().Set("X-Amz-Function-Error", tt.functionError)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"ok":true}`))
			}, testCredentials())

			got, err := client.Invoke(t.Context(), testFunctionName, []byte(testPayload))

			if err != nil {
				t.Fatalf("Invoke() error = %v", err)
			}
			if got.StatusCode != tt.want.StatusCode || got.FunctionError != tt.want.FunctionError || string(got.Payload) != string(tt.want.Payload) {
				t.Errorf("Invoke() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLambdaClient_Invoke_ErrorResponseReturnsAPIErrorWithoutMessage(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		handler http.HandlerFunc
		want    awsapi.APIError
	}{
		"403 with an AWS error code": {
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-Amzn-Errortype", "AccessDeniedException")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"Message":"` + messageMarker + `"}`))
			},
			want: awsapi.APIError{Code: "AccessDeniedException", HTTPStatus: 403},
		},
		"502 HTML without an AWS error code": {
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`<html>` + messageMarker + `</html>`))
			},
			want: awsapi.APIError{Code: "", HTTPStatus: 502},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := newLambdaClient(t, tt.handler, testCredentials())

			_, err := client.Invoke(t.Context(), testFunctionName, []byte(testPayload))

			apiErr, ok := errors.AsType[*awsapi.APIError](err)
			if !ok {
				t.Fatalf("error = %v (%T), want *awsapi.APIError", err, err)
			}
			if *apiErr != tt.want {
				t.Errorf("APIError = %+v, want %+v", *apiErr, tt.want)
			}
			if strings.Contains(err.Error(), messageMarker) {
				t.Errorf("Error() = %q, want it without the response message", err.Error())
			}
		})
	}
}

func TestLambdaClient_Invoke_DoesNotRetry(t *testing.T) {
	t.Parallel()

	var count atomic.Int32
	client := newLambdaClient(t, func(w http.ResponseWriter, _ *http.Request) {
		count.Add(1)
		w.Header().Set("X-Amzn-Errortype", "ServiceUnavailableException")
		w.WriteHeader(http.StatusServiceUnavailable)
	}, testCredentials())

	_, err := client.Invoke(t.Context(), testFunctionName, []byte(testPayload))

	if err == nil {
		t.Fatal("Invoke() error = nil, want an error")
	}
	if got := count.Load(); got != 1 {
		t.Errorf("requests = %d, want 1", got)
	}
}

func TestLambdaClient_Invoke_CommunicationFailure(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		newClient func(t *testing.T) *awsapi.LambdaClient
		newCtx    func(t *testing.T) context.Context
		wantIs    error
		wantMsg   string
	}{
		"deadline exceeded is a timeout": {
			newClient: func(t *testing.T) *awsapi.LambdaClient {
				return newLambdaClient(t, blockUntilCanceled, testCredentials())
			},
			newCtx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
				t.Cleanup(cancel)
				return ctx
			},
			wantIs:  context.DeadlineExceeded,
			wantMsg: "the request timed out",
		},
		"deadline exceeded while reading the 200 body is a timeout": {
			newClient: func(t *testing.T) *awsapi.LambdaClient {
				return newLambdaClient(t, func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.Copy(io.Discard, r.Body)
					w.Header().Set("Content-Length", "1000")
					w.WriteHeader(http.StatusOK)
					_, _ = io.WriteString(w, `{"ok":`)
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				}, testCredentials())
			},
			newCtx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
				t.Cleanup(cancel)
				return ctx
			},
			wantIs:  context.DeadlineExceeded,
			wantMsg: "the request timed out",
		},
		"canceled is a failure without a response": {
			newClient: func(t *testing.T) *awsapi.LambdaClient {
				return newLambdaClient(t, blockUntilCanceled, testCredentials())
			},
			newCtx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				time.AfterFunc(100*time.Millisecond, cancel)
				return ctx
			},
			wantIs:  context.Canceled,
			wantMsg: "the request failed without a response",
		},
		"connection refused is a failure without a response": {
			newClient: func(*testing.T) *awsapi.LambdaClient {
				return awsapi.NewLambdaClient(testRegion, testCredentials(), closedServerOptions()...)
			},
			newCtx:  func(t *testing.T) context.Context { return t.Context() },
			wantMsg: "the request failed without a response",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := tt.newClient(t)

			_, err := client.Invoke(tt.newCtx(t), testFunctionName, []byte(testPayload))

			if _, ok := errors.AsType[*communication.Failure](err); !ok {
				t.Fatalf("error = %v (%T), want *communication.Failure", err, err)
			}
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Errorf("errors.Is(err, %v) = false, want true", tt.wantIs)
			}
			if got := err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestLambdaClient_Invoke_InvalidRegionReturnsNotSent(t *testing.T) {
	t.Parallel()

	client := awsapi.NewLambdaClient("bad region", testCredentials())

	_, err := client.Invoke(t.Context(), testFunctionName, []byte(testPayload))

	if err == nil {
		t.Fatal("Invoke() error = nil, want an error")
	}
	if got, want := err.Error(), "the request was not sent"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
