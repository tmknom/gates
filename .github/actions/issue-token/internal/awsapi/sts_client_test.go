package awsapi_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/awsapi"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/communication"
	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
)

const (
	testRoleARN = "arn:aws:iam::123456789012:role/gates-role"
	testJWT     = "eyJhbGciOi.payload.signature"
)

func writeXML(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func TestSTSClient_AssumeRoleWithWebIdentity_SendsUnsignedRequestAndReturnsCredentials(t *testing.T) {
	t.Parallel()

	got := make(chan capturedRequest, 1)
	client := newSTSClient(t, func(w http.ResponseWriter, r *http.Request) {
		got <- capture(r)
		writeXML(w, http.StatusOK, stsSuccessXML)
	})

	creds, err := client.AssumeRoleWithWebIdentity(t.Context(), testRoleARN, credential.NewOIDCJWT(testJWT))
	if err != nil {
		t.Fatalf("AssumeRoleWithWebIdentity() error = %v", err)
	}

	req := received(t, got)
	if req.method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.method)
	}
	if v := req.header.Get("Authorization"); v != "" {
		t.Errorf("Authorization = %q, want no signature", v)
	}
	form, err := url.ParseQuery(req.body)
	if err != nil {
		t.Fatalf("parse body %q: %v", req.body, err)
	}
	wantForm := map[string]string{
		"Action":           "AssumeRoleWithWebIdentity",
		"RoleArn":          testRoleARN,
		"WebIdentityToken": testJWT,
		"RoleSessionName":  "gates-issue-token",
		"DurationSeconds":  "900",
	}
	for key, want := range wantForm {
		if got := form.Get(key); got != want {
			t.Errorf("form %s = %q, want %q", key, got, want)
		}
	}

	if got := creds.AccessKeyID.Expose(); got != testAccessKeyID {
		t.Errorf("AccessKeyID = %q, want %q", got, testAccessKeyID)
	}
	if got := creds.SecretAccessKey.Expose(); got != testSecretAccessKey {
		t.Errorf("SecretAccessKey = %q, want %q", got, testSecretAccessKey)
	}
	if got := creds.SessionToken.Expose(); got != testSessionToken {
		t.Errorf("SessionToken = %q, want %q", got, testSessionToken)
	}
}

func TestSTSClient_AssumeRoleWithWebIdentity_AWSErrorReturnsAPIErrorWithoutMessage(t *testing.T) {
	t.Parallel()

	client := newSTSClient(t, func(w http.ResponseWriter, _ *http.Request) {
		writeXML(w, http.StatusForbidden, stsAccessDeniedXML)
	})

	_, err := client.AssumeRoleWithWebIdentity(t.Context(), testRoleARN, credential.NewOIDCJWT(testJWT))

	apiErr, ok := errors.AsType[*awsapi.APIError](err)
	if !ok {
		t.Fatalf("error = %v (%T), want *awsapi.APIError", err, err)
	}
	if apiErr.Code != "AccessDenied" || apiErr.HTTPStatus != http.StatusForbidden {
		t.Errorf("APIError = %+v, want Code AccessDenied and HTTPStatus 403", apiErr)
	}
	if strings.Contains(err.Error(), messageMarker) {
		t.Errorf("Error() = %q, must not contain the AWS message", err.Error())
	}
}

func TestSTSClient_AssumeRoleWithWebIdentity_MissingCredentialValueReturnsError(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"no Credentials element": `<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><AssumeRoleWithWebIdentityResult></AssumeRoleWithWebIdentityResult></AssumeRoleWithWebIdentityResponse>`,
		"no AccessKeyId":         strings.Replace(stsSuccessXML, "<AccessKeyId>"+testAccessKeyID+"</AccessKeyId>", "", 1),
		"no SecretAccessKey":     strings.Replace(stsSuccessXML, "<SecretAccessKey>"+testSecretAccessKey+"</SecretAccessKey>", "", 1),
		"no SessionToken":        strings.Replace(stsSuccessXML, "<SessionToken>"+testSessionToken+"</SessionToken>", "", 1),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := newSTSClient(t, func(w http.ResponseWriter, _ *http.Request) {
				writeXML(w, http.StatusOK, body)
			})

			_, err := client.AssumeRoleWithWebIdentity(t.Context(), testRoleARN, credential.NewOIDCJWT(testJWT))

			if err == nil {
				t.Fatal("AssumeRoleWithWebIdentity() error = nil, want an error")
			}
			if got, want := err.Error(), "the AWS STS response lacks AWS credentials"; got != want {
				t.Errorf("Error() = %q, want %q", got, want)
			}
		})
	}
}

func TestSTSClient_AssumeRoleWithWebIdentity_DeadlineReturnsTimedOutCommunicationFailure(t *testing.T) {
	t.Parallel()

	client := newSTSClient(t, blockUntilCanceled)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	_, err := client.AssumeRoleWithWebIdentity(ctx, testRoleARN, credential.NewOIDCJWT(testJWT))

	if _, ok := errors.AsType[*communication.Failure](err); !ok {
		t.Fatalf("error = %v (%T), want *communication.Failure", err, err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false, want true")
	}
	if got, want := err.Error(), "the request timed out"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestSTSClient_AssumeRoleWithWebIdentity_DoesNotRetry(t *testing.T) {
	t.Parallel()

	var count atomic.Int32
	client := newSTSClient(t, func(w http.ResponseWriter, _ *http.Request) {
		count.Add(1)
		writeXML(w, http.StatusServiceUnavailable, stsAccessDeniedXML)
	})

	_, err := client.AssumeRoleWithWebIdentity(t.Context(), testRoleARN, credential.NewOIDCJWT(testJWT))

	if err == nil {
		t.Fatal("AssumeRoleWithWebIdentity() error = nil, want an error")
	}
	if got := count.Load(); got != 1 {
		t.Errorf("requests = %d, want 1", got)
	}
}

func TestSTSClient_AssumeRoleWithWebIdentity_InvalidRegionReturnsNotSent(t *testing.T) {
	t.Parallel()

	client := awsapi.NewSTSClient("bad region")

	_, err := client.AssumeRoleWithWebIdentity(t.Context(), testRoleARN, credential.NewOIDCJWT(testJWT))

	if err == nil {
		t.Fatal("AssumeRoleWithWebIdentity() error = nil, want an error")
	}
	if got, want := err.Error(), "the request was not sent"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
