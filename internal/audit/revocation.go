package audit

// RevocationOutcome は、照合に失敗した発行処理の失効の結果で、値は Lambda Design の監査記録の revocation の値である。
// Token Issuer の同名の型とは別に置き、値ごとの対応で写す。
// フィールドを非公開の struct にし、3 つの値とゼロ値のほかを作れないようにする。Token Issuer の整数の値を変換で写すこともできない。
type RevocationOutcome struct {
	value string
}

var (
	// RevocationNotAttempted は、失効を試みなかったことを表す。
	RevocationNotAttempted = RevocationOutcome{value: "not_attempted"}
	// RevocationSucceeded は、失効を試みて成功したことを表す。
	RevocationSucceeded = RevocationOutcome{value: "succeeded"}
	// RevocationFailed は、失効を試みて失敗したことを表す。
	RevocationFailed = RevocationOutcome{value: "failed"}
)

// IsZero は、3 つの値のどれでもないゼロ値かを返す。ゼロ値は、写し忘れを試みなかったと記録しないよう、どの値にも当たらない。
func (o RevocationOutcome) IsZero() bool {
	return o.value == ""
}
