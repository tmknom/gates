package handler

import (
	"encoding/json/v2"
	"maps"
	"time"

	"github.com/tmknom/gates/internal/errorcode"
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

// response は、JSON に符号化した応答である。
// 符号化できなかったことをゼロ値で表し、error を返さない。符号化の error の文言には応答の値（トークンを含む）が入り得るため、捨てる。
type response []byte

// IsZero は、符号化できなかった応答かを返す。
func (r response) IsZero() bool {
	return r == nil
}

// encodeSuccess は、照合を通った発行結果から成功応答を作る。
func encodeSuccess(requestID string, installationToken issuer.InstallationToken) response {
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
func encodeFailure(requestID string, code errorcode.Code) response {
	return encode(failureResponse{
		Error:     failureDetail{Code: code.Value(), Message: code.Message()},
		RequestID: requestID,
	})
}

// encode は、応答を JSON に符号化する。
// encoding/json/v2 の符号化は、UTF-8 として正しくない文字列で失敗する。
func encode(value any) response {
	b, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return b
}
