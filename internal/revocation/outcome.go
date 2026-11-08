// Package revocation は、照合に失敗した発行処理の失効の結果である。
// Token Issuer が作り、監査記録が読む。監査記録が資格情報の型を公開する package を import しないよう、資格情報を持たない package に置く。
package revocation

// Outcome は、照合に失敗した発行処理の失効の結果で、値は Lambda Design の監査記録の revocation の値である。
// フィールドを非公開の struct にし、3 つの値とゼロ値のほかを作れないようにする。
type Outcome struct {
	value string
}

var (
	// NotAttempted は、トークン文字列がなく失効を試みなかったことを表す。
	NotAttempted = Outcome{value: "not_attempted"}
	// Succeeded は、失効を試みて成功したことを表す。
	Succeeded = Outcome{value: "succeeded"}
	// Failed は、失効を試みて失敗したことを表す。
	Failed = Outcome{value: "failed"}
)

// IsZero は、3 つの値のどれでもないゼロ値かを返す。
// ゼロ値をどの値にも当てないのは、失効の結果を決め忘れた照合失敗を、試みなかったとも試みたとも記録しないためである。
func (o Outcome) IsZero() bool {
	return o.value == ""
}

// String は、監査記録に書く値を返す。
func (o Outcome) String() string {
	return o.value
}
