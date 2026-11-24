package github_test

import (
	"context"
	"encoding/base64"
	"encoding/json/jsontext"
	"errors"
	"io"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tmknom/gates/internal/github"
	"github.com/tmknom/gates/internal/repository"
	"github.com/tmknom/gates/internal/repositorypolicy"
)

const (
	// repositoryPolicySizeLimit と repositoryPolicyTimeout は Lambda Design の GitHub Client と Call Time Budget が定める値である。
	repositoryPolicySizeLimit = 2 * 1024 * 1024
	repositoryPolicyTimeout   = 5 * time.Second

	// documentMarker は、リポジトリポリシーの内容がエラー文言に漏れていないことを確かめる目印である。
	documentMarker = "document-content-marker"
	// recordedDocument は、調査記録の、既定ブランチの YAML のファイルに見立てた文書である。
	recordedDocument = "version: 1\nrules:\n  - name: " + documentMarker + "\n"

	policyFullName = allowedOwnerName + "/repo-b"
)

func TestFetchRepositoryPolicy_SendsRequestForFixedPathWithoutRef(t *testing.T) {
	sent := sentFetchRequest(t)

	if sent.Method != http.MethodGet {
		t.Errorf("request method = %q, want %q", sent.Method, http.MethodGet)
	}
	if want := "https://api.github.com/repos/allowed-owner/repo-b/contents/.github/gates-policy.yml"; sent.URL.String() != want {
		t.Errorf("request URL = %q, want %q", sent.URL.String(), want)
	}
	for name, want := range map[string]string{
		"Accept":               "application/vnd.github+json",
		"Authorization":        "Bearer " + tokenMarker,
		"X-GitHub-Api-Version": "2026-03-10",
	} {
		if got := sent.Header.Get(name); got != want {
			t.Errorf("request header %s = %q, want %q", name, got, want)
		}
	}
}

func TestFetchRepositoryPolicy_WaitsAtMostRepositoryPolicyTimeout(t *testing.T) {
	deadline, ok := sentFetchRequest(t).Context().Deadline()
	if !ok {
		t.Fatal("request has no deadline")
	}
	if remaining := time.Until(deadline); remaining > repositoryPolicyTimeout || remaining < repositoryPolicyTimeout-time.Second {
		t.Errorf("remaining time until the deadline = %v, want about %v", remaining, repositoryPolicyTimeout)
	}
}

func TestFetchRepositoryPolicy_ClassifiesRecordedResponses(t *testing.T) {
	symlinkedDocument := "repositories: []\n# " + documentMarker + " linked\n"

	tests := []struct {
		name   string
		status int
		body   map[string]any
		array  []any
		want   repositorypolicy.FetchResult
	}{
		{name: "file returns the decoded document", status: 200, body: recordedContents(recordedDocument), want: repositorypolicy.Document(recordedDocument)},
		{name: "symlink to a file returns the linked document", status: 200, body: recordedContents(symlinkedDocument), want: repositorypolicy.Document(symlinkedDocument)},
		{name: "empty file returns the empty document", status: 200, body: recordedContents(""), want: repositorypolicy.Document("")},
		{name: "directory returns NotFile", status: 200, array: recordedDirectory(), want: repositorypolicy.NotFile()},
		{name: "dangling symlink returns NotFile", status: 200, body: recordedDanglingSymlink(), want: repositorypolicy.NotFile()},
		{name: "file over 1 MB returns TooLarge", status: 200, body: recordedLargeFile(), want: repositorypolicy.TooLarge()},
		{name: "status 404 returns NotFound", status: 404, body: recordedNotFound(), want: repositorypolicy.NotFound()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body any = tt.body
			if tt.array != nil {
				body = tt.array
			}

			got, err := fetchRepositoryPolicy(t, newAPIStub(t, respondContents(tt.status, nil, marshal(t, body))).client(t))

			if err != nil {
				t.Fatalf("FetchRepositoryPolicy returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("FetchRepositoryPolicy = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestFetchRepositoryPolicy_ClassifiesBySizeAtMaxDocumentBytes(t *testing.T) {
	atLimit := strings.Repeat("a", repositorypolicy.MaxDocumentBytes)
	overLimit := strings.Repeat("a", repositorypolicy.MaxDocumentBytes+1)

	tests := []struct {
		name string
		body map[string]any
		want repositorypolicy.FetchResult
	}{
		{name: "size at the limit returns the document", body: recordedContents(atLimit), want: repositorypolicy.Document(atLimit)},
		{name: "size over the limit returns TooLarge", body: recordedContents(overLimit), want: repositorypolicy.TooLarge()},
		// 大きすぎるファイルは encoding と content を読まない
		{name: "size over the limit without content returns TooLarge", body: withoutField(recordedContents(overLimit), "content"), want: repositorypolicy.TooLarge()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fetchRepositoryPolicy(t, newAPIStub(t, respondContents(http.StatusOK, nil, marshal(t, tt.body))).client(t))

			if err != nil {
				t.Fatalf("FetchRepositoryPolicy returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("FetchRepositoryPolicy = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// type が file 以外の文字列なら、ほかのフィールドを読まずにファイルでないと分類することを確かめる。
func TestFetchRepositoryPolicy_TypeOtherThanFileReturnsNotFile(t *testing.T) {
	tests := []struct {
		name string
		body map[string]any
	}{
		{name: "type symlink", body: withField(recordedContents(recordedDocument), "type", `"symlink"`)},
		{name: "type dir", body: withField(recordedContents(recordedDocument), "type", `"dir"`)},
		{name: "type submodule", body: withField(recordedContents(recordedDocument), "type", `"submodule"`)},
		{name: "type empty string", body: withField(recordedContents(recordedDocument), "type", `""`)},
		{name: "type File in a different case", body: withField(recordedContents(recordedDocument), "type", `"File"`)},
		{name: "type dir with invalid size", body: withField(withField(recordedContents(recordedDocument), "type", `"dir"`), "size", `"invalid"`)},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns NotFile", func(t *testing.T) {
			got, err := fetchRepositoryPolicy(t, newAPIStub(t, respondContents(http.StatusOK, nil, marshal(t, tt.body))).client(t))

			if err != nil {
				t.Fatalf("FetchRepositoryPolicy returned error: %v", err)
			}
			if got != repositorypolicy.NotFile() {
				t.Errorf("FetchRepositoryPolicy = %+v, want NotFile", got)
			}
		})
	}
}

// 404 は本文を読まずにリポジトリポリシーがないと分類することを確かめる。
func TestFetchRepositoryPolicy_NotFoundDoesNotReadBody(t *testing.T) {
	got, err := fetchRepositoryPolicy(t, newAPIStub(t, respondContents(http.StatusNotFound, nil, "<html>"+responseMarker)).client(t))

	if err != nil {
		t.Fatalf("FetchRepositoryPolicy returned error: %v", err)
	}
	if got != repositorypolicy.NotFound() {
		t.Errorf("FetchRepositoryPolicy = %+v, want NotFound", got)
	}
}

//trace:checks AC-QUALITY-001 AC-QUALITY-002
func TestFetchRepositoryPolicy_StatusFailure(t *testing.T) {
	failureBody := `{"message":"` + responseMarker + `","status":"403"}`

	tests := []struct {
		name   string
		status int
		header map[string]string
		body   string
		want   error
	}{
		{name: "status 403 not accessible by integration", status: 403, body: marshal(t, recordedNotAccessible()), want: github.ErrInternalFailure},
		{name: "status 403 with exhausted rate limit", status: 403, header: map[string]string{"X-Ratelimit-Remaining": "0"}, body: failureBody, want: github.ErrTransientFailure},
		{name: "status 403 with Retry-After", status: 403, header: map[string]string{"Retry-After": "60"}, body: failureBody, want: github.ErrTransientFailure},
		{name: "status 401", status: 401, body: failureBody, want: github.ErrInternalFailure},
		{name: "status 429", status: 429, body: failureBody, want: github.ErrTransientFailure},
		{name: "status 500", status: 500, body: failureBody, want: github.ErrTransientFailure},
		{name: "status 204", status: 204, want: github.ErrInternalFailure},
		// 200 以外の 2xx は、ファイルの応答と同じ本文でも読まない
		{name: "status 201 with a file body", status: 201, body: marshal(t, recordedContents(recordedDocument)), want: github.ErrInternalFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns "+tt.want.Error(), func(t *testing.T) {
			got, err := fetchRepositoryPolicy(t, newAPIStub(t, respondContents(tt.status, tt.header, tt.body)).client(t))

			assertFetchFailed(t, got, err, tt.want)
			for _, value := range []string{"status " + strconv.Itoa(tt.status), labeledRequestID} {
				if !strings.Contains(err.Error(), value) {
					t.Errorf("error message %q does not contain %q", err.Error(), value)
				}
			}
		})
	}
}

//trace:checks AC-QUALITY-002
func TestFetchRepositoryPolicy_DoesNotFollowRedirect(t *testing.T) {
	const redirectPath = "/redirected"
	body := marshal(t, recordedContents(recordedDocument))
	// 転送先は同じサーバーにあり、従えばリポジトリポリシーを取得できる
	var redirected atomic.Int32
	stub := newAPIStub(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == redirectPath {
			redirected.Add(1)
			respondBody(body)(w, r)
			return
		}
		w.Header().Set("X-GitHub-Request-Id", requestID)
		http.Redirect(w, r, redirectPath, http.StatusFound)
	})

	got, err := fetchRepositoryPolicy(t, stub.client(t))

	assertFetchFailed(t, got, err, github.ErrInternalFailure)
	if n := redirected.Load(); n != 0 {
		t.Errorf("redirect target received %d requests, want 0", n)
	}
}

//trace:checks AC-QUALITY-001 AC-QUALITY-002
func TestFetchRepositoryPolicy_CommunicationFailureReturnsErrTransientFailure(t *testing.T) {
	closeConnection := func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("Hijack returned error: %v", err)
			return
		}
		_ = conn.Close()
	}
	partialBody := `{"type":"file","message":"` + responseMarker
	truncateBody := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = io.WriteString(w, partialBody)
	}
	waitForCancel := func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }
	waitForCancelInBody := func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, partialBody)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}

	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{name: "connection closed before the response", handler: closeConnection},
		{name: "connection closed while reading the body", handler: truncateBody},
		{name: "deadline exceeded after the request is sent", handler: waitForCancel},
		{name: "deadline exceeded while reading the body", handler: waitForCancelInBody},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrTransientFailure", func(t *testing.T) {
			client := newAPIStub(t, tt.handler).client(t)
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()

			got, err := client.FetchRepositoryPolicy(ctx, issuedPolicyReadToken(t), fullName(t, policyFullName))

			assertFetchFailed(t, got, err, github.ErrTransientFailure)
		})
	}
}

//trace:checks AC-QUALITY-001 AC-QUALITY-002
func TestFetchRepositoryPolicy_ExpiredContextReturnsErrTransientFailure(t *testing.T) {
	client := newAPIStub(t, respondBody(marshal(t, recordedContents(recordedDocument)))).client(t)
	token := issuedPolicyReadToken(t)
	ctx, cancel := context.WithTimeout(t.Context(), -time.Second)
	defer cancel()

	got, err := client.FetchRepositoryPolicy(ctx, token, fullName(t, policyFullName))

	assertFetchFailed(t, got, err, github.ErrTransientFailure)
}

func TestFetchRepositoryPolicy_BodyAtTheLimitIsRead(t *testing.T) {
	body := padded(marshal(t, recordedContents(recordedDocument)), repositoryPolicySizeLimit)

	got, err := fetchRepositoryPolicy(t, newAPIStub(t, respondBody(body)).client(t))

	if err != nil {
		t.Fatalf("FetchRepositoryPolicy returned error: %v", err)
	}
	if want := repositorypolicy.Document(recordedDocument); got != want {
		t.Errorf("FetchRepositoryPolicy = %+v, want %+v", got, want)
	}
}

// 形が想定と異なる 200 の応答を、ファイルの有無を示さない内部障害にすることを確かめる。
//
//trace:checks AC-QUALITY-002
func TestFetchRepositoryPolicy_UnexpectedBodyReturnsErrInternalFailure(t *testing.T) {
	file := func(t *testing.T) string { return marshal(t, recordedContents(recordedDocument)) }
	// appendMembers は、ファイルの応答の最上位のオブジェクトの末尾に members を足す。
	// 末尾に置くのは、復号が途中で止まっても分類に使うフィールドを読み終えた後で拒否されることを確かめるためである
	appendMembers := func(t *testing.T, members string) string {
		body := file(t)
		return body[:len(body)-1] + "," + members + "}"
	}
	fileWithField := func(name, raw string) func(*testing.T) string {
		return func(t *testing.T) string { return marshal(t, withField(recordedContents(recordedDocument), name, raw)) }
	}
	fileWithoutField := func(name string) func(*testing.T) string {
		return func(t *testing.T) string { return marshal(t, withoutField(recordedContents(recordedDocument), name)) }
	}

	tests := []struct {
		name string
		body func(*testing.T) string
	}{
		{name: "body over the size limit", body: func(t *testing.T) string { return padded(file(t), repositoryPolicySizeLimit+1) }},
		{name: "non JSON body", body: func(*testing.T) string { return "<html>" + responseMarker }},
		{name: "object with trailing data", body: func(t *testing.T) string { return file(t) + "[]" }},
		{name: "broken array", body: func(*testing.T) string { return `[{"name":"` + responseMarker }},
		{name: "invalid UTF-8", body: func(t *testing.T) string { return appendMembers(t, "\"name\":\"\xff\"") }},
		{name: "duplicate top level name", body: func(t *testing.T) string { return appendMembers(t, `"type":"file"`) }},
		{name: "duplicate unread top level name", body: func(t *testing.T) string { return appendMembers(t, `"sha":"`+responseMarker+`"`) }},
		{name: "duplicate nested name", body: func(t *testing.T) string { return appendMembers(t, `"extra":{"self":"a","self":"b"}`) }},
		{name: "duplicate name in an array element", body: func(*testing.T) string { return `[{"name":"a","name":"b"}]` }},
		{name: "top level string", body: func(*testing.T) string { return `"file"` }},
		{name: "top level null", body: func(*testing.T) string { return `null` }},
		{name: "empty body", body: func(*testing.T) string { return `` }},
		{name: "type missing", body: fileWithoutField("type")},
		{name: "type null", body: fileWithField("type", `null`)},
		{name: "type number", body: fileWithField("type", `1`)},
		{name: "size missing", body: fileWithoutField("size")},
		{name: "size null", body: fileWithField("size", `null`)},
		{name: "size negative", body: fileWithField("size", `-58`)},
		{name: "size fraction notation", body: fileWithField("size", `58.0`)},
		{name: "size exponent notation", body: fileWithField("size", `5.8e1`)},
		{name: "size string", body: fileWithField("size", `"58"`)},
		// 空のファイルでは、読めない size を 0 と読むと長さの照合を通る
		{name: "size null for the empty file", body: func(t *testing.T) string { return marshal(t, withField(recordedContents(""), "size", `null`)) }},
		{name: "size fraction notation for the empty file", body: func(t *testing.T) string {
			return marshal(t, withField(recordedContents(""), "size", `0.0`))
		}},
		{name: "size over uint64", body: fileWithField("size", `18446744073709551616`)},
		{name: "encoding none", body: fileWithField("encoding", `"none"`)},
		{name: "encoding missing", body: fileWithoutField("encoding")},
		{name: "content missing", body: fileWithoutField("content")},
		{name: "content null", body: fileWithField("content", `null`)},
		{name: "content number", body: fileWithField("content", `1`)},
		// 不正な文字の前までを復号すると size と同じ長さになり、長さの照合では拒否されない
		{name: "content not base64", body: func(t *testing.T) string {
			return marshal(t, withField(recordedContents(recordedDocument), "content", marshal(t, contentsBase64(recordedDocument)+"!")))
		}},
		// 空のファイルでは、読めない content を空文字列と読むと長さの照合を通る
		{name: "content missing for the empty file", body: func(t *testing.T) string { return marshal(t, withoutField(recordedContents(""), "content")) }},
		{name: "content null for the empty file", body: func(t *testing.T) string {
			return marshal(t, withField(recordedContents(""), "content", `null`))
		}},
		{name: "content number for the empty file", body: func(t *testing.T) string {
			return marshal(t, withField(recordedContents(""), "content", `0`))
		}},
		{name: "decoded length shorter than size", body: fileWithField("size", strconv.Itoa(len(recordedDocument)+1))},
		{name: "decoded length longer than size", body: fileWithField("size", strconv.Itoa(len(recordedDocument)-1))},
	}
	for _, tt := range tests {
		t.Run(tt.name+" returns ErrInternalFailure", func(t *testing.T) {
			got, err := fetchRepositoryPolicy(t, newAPIStub(t, respondBody(tt.body(t))).client(t))

			assertFetchFailed(t, got, err, github.ErrInternalFailure)
			if !strings.Contains(err.Error(), labeledRequestID) {
				t.Errorf("error message %q does not contain %q", err.Error(), labeledRequestID)
			}
		})
	}
}

// リポジトリ名が . か .. の対象は、パスの区画から取り除かれて固定のパスと異なるエンドポイントになるため、要求を送らないことを確かめる。
//
//trace:checks AC-QUALITY-002
func TestFetchRepositoryPolicy_DotSegmentRepositoryNameIsNotSent(t *testing.T) {
	for _, name := range []string{".", ".."} {
		t.Run("repository name "+name+" returns ErrInternalFailure without request", func(t *testing.T) {
			stub := newAPIStub(t, respondBody(marshal(t, recordedContents(recordedDocument))))

			got, err := stub.client(t).FetchRepositoryPolicy(t.Context(), issuedPolicyReadToken(t), fullName(t, allowedOwnerName+"/"+name))

			assertFetchFailed(t, got, err, github.ErrInternalFailure)
			if stub.request.Load() != nil {
				t.Error("FetchRepositoryPolicy sent a request")
			}
		})
	}
}

func fetchRepositoryPolicy(t *testing.T, client *github.Client) (repositorypolicy.FetchResult, error) {
	t.Helper()
	return client.FetchRepositoryPolicy(t.Context(), issuedPolicyReadToken(t), fullName(t, policyFullName))
}

func fullName(t *testing.T, s string) repository.FullName {
	t.Helper()
	name, err := repository.ParseFullName(s)
	if err != nil {
		t.Fatalf("ParseFullName returned error: %v", err)
	}
	return name
}

// sentFetchRequest は、既定の API Host のまま FetchRepositoryPolicy を呼び、外へ送らずに受け取った要求を返す。
func sentFetchRequest(t *testing.T) *http.Request {
	t.Helper()
	token := issuedPolicyReadToken(t)
	var sent *http.Request
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		sent = request
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(marshal(t, recordedContents(recordedDocument)))),
		}, nil
	})
	client := github.NewClient(github.WithTransport(transport))
	if _, err := client.FetchRepositoryPolicy(t.Context(), token, fullName(t, policyFullName)); err != nil {
		t.Fatalf("FetchRepositoryPolicy returned error: %v", err)
	}
	return sent
}

func respondContents(status int, header map[string]string, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-GitHub-Request-Id", requestID)
		for name, value := range header {
			w.Header().Set(name, value)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

// recordedContents は、調査記録の、ファイルを返す Contents API の応答を、内容を document にして返す。
// download_url の値は記録していないため、資格情報を含まない形に置き換えている。
func recordedContents(document string) map[string]any {
	return map[string]any{
		"name":         "gates-policy.yml",
		"path":         repositorypolicy.Path,
		"sha":          "3d21ec53a331a6f037a91c368710b99387d012c1",
		"size":         len(document),
		"url":          "https://api.github.com/repos/octocat/example-repo/contents/.github/gates-policy.yml?ref=main",
		"html_url":     "https://github.com/octocat/example-repo/blob/main/.github/gates-policy.yml",
		"git_url":      "https://api.github.com/repos/octocat/example-repo/git/blobs/3d21ec53a331a6f037a91c368710b99387d012c1",
		"download_url": "https://raw.githubusercontent.com/octocat/example-repo/main/.github/gates-policy.yml",
		"type":         "file",
		"content":      contentsBase64(document),
		"encoding":     "base64",
		"_links": map[string]any{
			"self": "https://api.github.com/repos/octocat/example-repo/contents/.github/gates-policy.yml?ref=main",
			"git":  "https://api.github.com/repos/octocat/example-repo/git/blobs/3d21ec53a331a6f037a91c368710b99387d012c1",
			"html": "https://github.com/octocat/example-repo/blob/main/.github/gates-policy.yml",
		},
	}
}

// recordedDirectory は、調査記録の、パスがディレクトリのときの応答である。要素は内容を持たない。
func recordedDirectory() []any {
	entry := recordedContents("inner: true\n")
	entry["name"] = "inner.yml"
	entry["path"] = repositorypolicy.Path + "/inner.yml"
	delete(entry, "content")
	delete(entry, "encoding")
	return []any{entry}
}

// recordedDanglingSymlink は、調査記録の、存在しない先を指すシンボリックリンクの応答である。content のキーを持たない。
func recordedDanglingSymlink() map[string]any {
	body := recordedContents("")
	body["type"] = "symlink"
	body["target"] = "nonexistent.yml"
	body["size"] = 15
	delete(body, "content")
	delete(body, "encoding")
	return body
}

// recordedLargeFile は、調査記録の、1 MB を超えるファイルの応答である。
func recordedLargeFile() map[string]any {
	body := recordedContents("")
	body["size"] = 1152048
	body["encoding"] = "none"
	return body
}

func recordedNotFound() map[string]any {
	return map[string]any{
		"message":           "Not Found",
		"documentation_url": "https://docs.github.com/rest/repos/contents#get-repository-content",
		"status":            "404",
	}
}

// recordedNotAccessible は、調査記録の、トークンの対象のリポジトリで指定していないパスを読んだ応答である。
func recordedNotAccessible() map[string]any {
	return map[string]any{
		"message":           "Resource not accessible by integration",
		"documentation_url": "https://docs.github.com/rest/repos/contents#get-repository-content",
		"status":            "403",
	}
}

// withField は、応答の name のフィールドを JSON の raw に置き換えた写しを返す。
func withField(body map[string]any, name, raw string) map[string]any {
	clone := maps.Clone(body)
	clone[name] = jsontext.Value(raw)
	return clone
}

// withoutField は、応答から name のフィールドを除いた写しを返す。
func withoutField(body map[string]any, name string) map[string]any {
	clone := maps.Clone(body)
	delete(clone, name)
	return clone
}

// contentsBase64 は、GitHub の応答と同じく、Base64 を 60 文字ごとに改行で区切り、末尾にも改行を付ける。
func contentsBase64(document string) string {
	var b strings.Builder
	for line := range slices.Chunk([]byte(base64.StdEncoding.EncodeToString([]byte(document))), 60) {
		b.Write(line)
		b.WriteString("\n")
	}
	return b.String()
}

// assertFetchFailed は失敗の種別が一つだけで、取得の結果を返さず、エラー文言にトークン、応答本文、文書の内容、完全名が入らないことを確かめる。
func assertFetchFailed(t *testing.T, got repositorypolicy.FetchResult, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("FetchRepositoryPolicy error = %v, want %v", err, want)
	}
	for _, other := range []error{github.ErrTransientFailure, github.ErrInternalFailure} {
		if !errors.Is(other, want) && errors.Is(err, other) {
			t.Errorf("FetchRepositoryPolicy error = %v, also matches %v", err, other)
		}
	}
	if got != (repositorypolicy.FetchResult{}) {
		t.Errorf("FetchRepositoryPolicy = %+v, want the zero value", got)
	}
	for _, value := range []string{tokenMarker, responseMarker, documentMarker, policyFullName, "repo-b"} {
		if strings.Contains(err.Error(), value) {
			t.Errorf("error message %q contains %q", err.Error(), value)
		}
	}
}
