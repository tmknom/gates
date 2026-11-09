package awsapi_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tmknom/gates/.github/actions/issue-token/internal/credential"
)

// setAWSEnvironment は、SDK が既定で読む環境変数と設定ファイルを、引数で渡す値と食い違う内容にする。
func setAWSEnvironment(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	awsDir := filepath.Join(home, ".aws")
	if err := os.MkdirAll(awsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(awsDir, "config")
	credentialsFile := filepath.Join(awsDir, "credentials")
	if err := os.WriteFile(configFile, []byte("[default]\nregion = us-east-1\nendpoint_url = https://config-file.invalid\nmax_attempts = 5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(credentialsFile, []byte("[default]\naws_access_key_id = AKIAFROMFILE\naws_secret_access_key = from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("AWS_CONFIG_FILE", configFile)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credentialsFile)
	t.Setenv("AWS_PROFILE", "default")
	t.Setenv("AWS_SDK_LOAD_CONFIG", "1")
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAFROMENV")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "from-env")
	t.Setenv("AWS_SESSION_TOKEN", "from-env-token")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	t.Setenv("AWS_ENDPOINT_URL", "https://env-endpoint.invalid")
	t.Setenv("AWS_ENDPOINT_URL_LAMBDA", "https://env-endpoint-lambda.invalid")
	t.Setenv("AWS_ENDPOINT_URL_STS", "https://env-endpoint-sts.invalid")
	t.Setenv("AWS_MAX_ATTEMPTS", "5")
	t.Setenv("AWS_RETRY_MODE", "standard")
}

// t.Setenv を使うため t.Parallel は呼ばない。
func TestClients_IgnoreAWSEnvironmentAndConfigFiles(t *testing.T) {
	setAWSEnvironment(t)

	t.Run("STSClient sends one request to the base endpoint", func(t *testing.T) {
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
	})

	t.Run("LambdaClient signs with the given credentials and region", func(t *testing.T) {
		got := make(chan capturedRequest, 5)
		client := newLambdaClient(t, func(w http.ResponseWriter, r *http.Request) {
			got <- capture(r)
			w.WriteHeader(http.StatusServiceUnavailable)
		}, testCredentials())

		_, err := client.Invoke(t.Context(), testFunctionName, []byte(testPayload))

		if err == nil {
			t.Fatal("Invoke() error = nil, want an error")
		}
		if n := len(got); n != 1 {
			t.Fatalf("requests = %d, want 1", n)
		}
		req := received(t, got)
		auth := req.header.Get("Authorization")
		if !strings.Contains(auth, "Credential="+testAccessKeyID+"/") || !strings.Contains(auth, "/"+testRegion+"/lambda/") {
			t.Errorf("Authorization = %q, want the given access key ID and region", auth)
		}
		if v := req.header.Get("X-Amz-Security-Token"); v != testSessionToken {
			t.Errorf("X-Amz-Security-Token = %q, want %q", v, testSessionToken)
		}
	})
}
