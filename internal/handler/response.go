package handler

import (
	"encoding/json/v2"
	"maps"
	"time"

	"github.com/tmknom/gates/internal/issuer"
)

// 応答の形は Lambda Design の「Responses」に従う。

type successResponse struct {
	OK    bool   `json:"ok"`
	Token string `json:"token"`
	// encoding/json/v2 は time.Time を RFC 3339 で符号化し、秒未満の桁も落とさないため、GitHub が定めた有効期限を変えない
	ExpiresAt    time.Time         `json:"expires_at"`
	Repositories []string          `json:"repositories"`
	Permissions  map[string]string `json:"permissions"`
	RequestID    string            `json:"request_id"`
}

type failureResponse struct {
	OK        bool          `json:"ok"`
	Error     failureDetail `json:"error"`
	RequestID string        `json:"request_id"`
}

type failureDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// encodeSuccess は、照合を通った発行結果から成功応答を作る。
func encodeSuccess(requestID string, installationToken issuer.InstallationToken) []byte {
	effective := installationToken.EffectiveRepositories()
	repositories := make([]string, 0, len(effective))
	for _, repository := range effective {
		repositories = append(repositories, repository.FullName().String())
	}
	return encode(successResponse{
		OK:           true,
		Token:        installationToken.Token().Expose(),
		ExpiresAt:    installationToken.ExpiresAt(),
		Repositories: repositories,
		Permissions:  maps.Collect(installationToken.EffectivePermissions().Entries()),
		RequestID:    requestID,
	})
}

// encodeFailure は、エラーコードから失敗応答を作る。
// error ではなくエラーコードを受け取り、部品のエラー文言が応答に入る経路を作らない。
func encodeFailure(requestID string, code errorCode) []byte {
	return encode(failureResponse{
		Error:     failureDetail{Code: code.code, Message: code.message},
		RequestID: requestID,
	})
}

// encode は、応答を JSON に符号化する。
// encoding/json/v2 の符号化が失敗するのは、UTF-8 として正しくない文字列と、RFC 3339 で表せない時刻である。
// 応答の文字列は、GitHub Client が読み取った値、固定の文言、UTF-8 として正しい前提で受け取る相関 ID に限られ、有効期限は GitHub Client が RFC 3339 として読んだ時刻である。
// そのため、失敗は不変条件の破れとして panic にする。
// panic の値に error を使わないのは、error の文言に応答の値（トークンを含む）が入り得るためである。
func encode(response any) []byte {
	b, err := json.Marshal(response)
	if err != nil {
		panic("response cannot be encoded as JSON")
	}
	return b
}
