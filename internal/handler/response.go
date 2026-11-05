package handler

import (
	"encoding/json/v2"
	"errors"
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

// errResponseNotEncodable は、応答を JSON に符号化できなかったことを表す。
// 符号化の error を包まないのは、その文言に応答の値（トークンを含む）が入り得るためである。
var errResponseNotEncodable = errors.New("response cannot be encoded as JSON")

// encodeSuccess は、照合を通った発行結果から成功応答を作る。
func encodeSuccess(requestID string, installationToken issuer.InstallationToken) ([]byte, error) {
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
func encodeFailure(requestID string, code errorcode.Code) ([]byte, error) {
	return encode(failureResponse{
		Error:     failureDetail{Code: code.Value(), Message: code.Message()},
		RequestID: requestID,
	})
}

// encode は、応答を JSON に符号化する。
// encoding/json/v2 の符号化は、UTF-8 として正しくない文字列で失敗する。
// 相関 ID が UTF-8 として正しいかは Lambda Handler が発行処理の前に確かめる（Lambda Design の「Failure」）。ここでは失敗を error で返すだけにする。
func encode(response any) ([]byte, error) {
	b, err := json.Marshal(response)
	if err != nil {
		return nil, errResponseNotEncodable
	}
	return b, nil
}
