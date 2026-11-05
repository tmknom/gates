package audit

// RevocationOutcome は、照合に失敗した発行処理の失効の結果で、値は Lambda Design の監査記録の revocation の値である。
// Token Issuer の同名の型とは別に置き、値ごとの対応で写す。
// 基底型を文字列にし、Token Issuer の整数の値を変換で写すと go vet が止めるようにする。
type RevocationOutcome string

const (
	// RevocationNotAttempted は、失効を試みなかったことを表す。
	RevocationNotAttempted RevocationOutcome = "not_attempted"
	// RevocationSucceeded は、失効を試みて成功したことを表す。
	RevocationSucceeded RevocationOutcome = "succeeded"
	// RevocationFailed は、失効を試みて失敗したことを表す。
	RevocationFailed RevocationOutcome = "failed"
)

// valid は、3 つの値のどれかであるかを返す。ゼロ値は写し忘れを試みなかったと記録しないよう、どの値にも当たらない。
func (o RevocationOutcome) valid() bool {
	switch o {
	case RevocationNotAttempted, RevocationSucceeded, RevocationFailed:
		return true
	default:
		return false
	}
}
